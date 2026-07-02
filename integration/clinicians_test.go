package integration_test

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
)

// Pins clinician management: creation authz, list filters (text search,
// email, role), the invite lifecycle, role updates including last-admin
// protection, deletion, and the clinician-clinics listings.
var _ = Describe("Clinicians", Ordered, func() {
	var adminId string
	var auth func(*http.Request)
	var clinicId string
	var fam string

	var memberId, adminTwoId string
	var memberName, adminTwoName string

	listClinicians := func(query url.Values, reqAuth func(*http.Request)) []client.ClinicianV1 {
		GinkgoHelper()
		endpoint := fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId)
		if len(query) > 0 {
			endpoint = fmt.Sprintf("%s?%s", endpoint, query.Encode())
		}
		req := prepareRequest(http.MethodGet, endpoint, "")
		reqAuth(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		return decodeAs[[]client.ClinicianV1](resp)
	}

	clinicianIds := func(list []client.ClinicianV1) []string {
		ids := make([]string, 0, len(list))
		for _, c := range list {
			if c.Id != nil {
				ids = append(ids, *c.Id)
			}
		}
		return ids
	}

	updateClinician := func(clinicianId string, body map[string]interface{}, reqAuth func(*http.Request)) *http.Response {
		GinkgoHelper()
		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/clinicians/%s", clinicId, clinicianId), jsonBody(body))
		reqAuth(req)
		return do(req)
	}

	BeforeAll(func() {
		admin := newStubUser()
		adminId = admin.UserID
		auth = asUser(adminId)
		clinicId = *createClinic(auth).Id
		fam = uniqueId()

		member := newStubUser()
		memberId = member.UserID
		memberName = fmt.Sprintf("Zeb%s Member", fam)

		adminTwo := newStubUser()
		adminTwoId = adminTwo.UserID
		adminTwoName = fmt.Sprintf("Yara%s Admin", fam)
	})

	Describe("creation", func() {
		It("is limited to backend services", func() {
			body := map[string]interface{}{
				"id":    memberId,
				"name":  memberName,
				"email": fmt.Sprintf("zeb+%s@integration.test", fam),
				"roles": []string{"CLINIC_MEMBER"},
			}
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId), jsonBody(body))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)

			req = prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId), jsonBody(body))
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			body = map[string]interface{}{
				"id":    adminTwoId,
				"name":  adminTwoName,
				"email": fmt.Sprintf("yara+%s@integration.test", fam),
				"roles": []string{"CLINIC_ADMIN"},
			}
			req = prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId), jsonBody(body))
			asServer(req)
			expectStatus(do(req), http.StatusOK)
		})

		It("returns the created clinician by id", func() {
			clinician := getClinician(clinicId, memberId)
			Expect(clinician.Id).To(PointTo(Equal(memberId)))
			Expect(clinician.Name).To(PointTo(Equal(memberName)))
			Expect(clinician.Roles).To(ConsistOf("CLINIC_MEMBER"))
			Expect(clinician.CreatedTime).ToNot(BeNil())
		})
	})

	Describe("listing", func() {
		It("returns all clinicians of the clinic", func() {
			ids := clinicianIds(listClinicians(nil, auth))
			Expect(ids).To(ConsistOf(adminId, memberId, adminTwoId))
		})

		It("filters by role", func() {
			ids := clinicianIds(listClinicians(url.Values{"role": {"CLINIC_MEMBER"}}, auth))
			Expect(ids).To(ConsistOf(memberId))

			ids = clinicianIds(listClinicians(url.Values{"role": {"CLINIC_ADMIN"}}, auth))
			Expect(ids).To(ConsistOf(adminId, adminTwoId))
		})

		It("filters by email", func() {
			ids := clinicianIds(listClinicians(url.Values{"email": {fmt.Sprintf("zeb+%s@integration.test", fam)}}, auth))
			Expect(ids).To(ConsistOf(memberId))
		})

		It("only matches email tokens in text search", func() {
			// The text index is declared on email+fullName but clinician
			// documents store the name under "name", so searching by name
			// never matches — only email tokens do. The Postgres port must
			// decide whether to preserve or fix this.
			Expect(listClinicians(url.Values{"search": {fmt.Sprintf("Zeb%s", fam)}}, auth)).To(BeEmpty())

			matched := listClinicians(url.Values{"search": {"zeb"}}, auth)
			Expect(matched).To(HaveLen(1))
			Expect(matched[0].Id).To(PointTo(Equal(memberId)))
		})

		It("paginates", func() {
			page := listClinicians(url.Values{"limit": {"2"}, "offset": {"0"}}, auth)
			Expect(page).To(HaveLen(2))
			page = listClinicians(url.Values{"limit": {"2"}, "offset": {"2"}}, auth)
			Expect(page).To(HaveLen(1))
		})
	})

	Describe("invites", func() {
		var inviteId string
		var inviteEmail string
		var invitedUserId string

		BeforeAll(func() {
			inviteId = fmt.Sprintf("invite-%s", uniqueId())
			inviteEmail = fmt.Sprintf("invited+%s@integration.test", fam)
			invited := newStubUser()
			invitedUserId = invited.UserID

			body := map[string]interface{}{
				"inviteId": inviteId,
				"name":     fmt.Sprintf("Invited %s", fam),
				"email":    inviteEmail,
				"roles":    []string{"CLINIC_MEMBER"},
			}
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId), jsonBody(body))
			asServer(req)
			expectStatus(do(req), http.StatusOK)
		})

		inviteEndpoint := func() string {
			return fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", clinicId, inviteId)
		}

		It("returns the invited clinician without a user id", func() {
			req := prepareRequest(http.MethodGet, inviteEndpoint(), "")
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			invited := decodeAs[client.ClinicianV1](resp)
			Expect(invited.Id).To(BeNil())
			Expect(invited.InviteId).To(PointTo(Equal(inviteId)))
			Expect(invited.Email).To(Equal(inviteEmail))
		})

		It("associates the invite to a user", func() {
			req := prepareRequestWithBody(http.MethodPatch, inviteEndpoint(),
				jsonBody(map[string]interface{}{"userId": invitedUserId}))
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			clinician := decodeAs[client.ClinicianV1](resp)
			Expect(clinician.Id).To(PointTo(Equal(invitedUserId)))

			// The invite is consumed and the clinician is now a member.
			req = prepareRequest(http.MethodGet, inviteEndpoint(), "")
			asServer(req)
			expectStatus(do(req), http.StatusNotFound)

			Expect(getClinician(clinicId, invitedUserId).Roles).To(ConsistOf("CLINIC_MEMBER"))
		})

		It("deletes pending invites", func() {
			secondInviteId := fmt.Sprintf("invite-%s", uniqueId())
			body := map[string]interface{}{
				"inviteId": secondInviteId,
				"email":    fmt.Sprintf("invited2+%s@integration.test", fam),
				"roles":    []string{"CLINIC_MEMBER"},
			}
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", clinicId), jsonBody(body))
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			endpoint := fmt.Sprintf("/v1/clinics/%s/invites/clinicians/%s/clinician", clinicId, secondInviteId)
			req = prepareRequest(http.MethodDelete, endpoint, "")
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			req = prepareRequest(http.MethodGet, endpoint, "")
			asServer(req)
			expectStatus(do(req), http.StatusNotFound)
		})
	})

	Describe("updates", func() {
		It("round-trips role changes", func() {
			body := map[string]interface{}{
				"name":  memberName,
				"email": fmt.Sprintf("zeb+%s@integration.test", fam),
				"roles": []string{"CLINIC_ADMIN"},
			}
			resp := updateClinician(memberId, body, auth)
			expectStatus(resp, http.StatusOK)
			Expect(decodeAs[client.ClinicianV1](resp).Roles).To(ConsistOf("CLINIC_ADMIN"))

			body["roles"] = []string{"CLINIC_MEMBER"}
			resp = updateClinician(memberId, body, auth)
			expectStatus(resp, http.StatusOK)
			Expect(getClinician(clinicId, memberId).Roles).To(ConsistOf("CLINIC_MEMBER"))
		})

		It("rejects updates from clinic members", func() {
			body := map[string]interface{}{
				"name":  adminTwoName,
				"email": fmt.Sprintf("yara+%s@integration.test", fam),
				"roles": []string{"CLINIC_MEMBER"},
			}
			resp := updateClinician(adminTwoId, body, asUser(memberId))
			expectStatus(resp, http.StatusForbidden)
		})
	})

	Describe("last admin protection", func() {
		var soloClinicId string
		var soloAdminId string

		BeforeAll(func() {
			solo := newStubUser()
			soloAdminId = solo.UserID
			soloClinicId = *createClinic(asUser(soloAdminId)).Id
		})

		It("rejects demoting the only admin", func() {
			// "constraint violation: the clinic must have at least one admin"
			body := map[string]interface{}{
				"name":  "Solo Admin",
				"email": fmt.Sprintf("solo+%s@integration.test", uniqueId()),
				"roles": []string{"CLINIC_MEMBER"},
			}
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/clinicians/%s", soloClinicId, soloAdminId), jsonBody(body))
			asServer(req)
			expectStatus(do(req), http.StatusUnprocessableEntity)
		})

		It("rejects deleting the only admin", func() {
			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/clinics/%s/clinicians/%s", soloClinicId, soloAdminId), "")
			asServer(req)
			expectStatus(do(req), http.StatusUnprocessableEntity)
		})
	})

	Describe("deletion", func() {
		It("removes the clinician", func() {
			gone := newStubUser()
			createClinicianDirect(clinicId, gone.UserID, "CLINIC_MEMBER")

			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", clinicId, gone.UserID), "")
			auth(req)
			expectStatus(do(req), http.StatusOK)

			req = prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", clinicId, gone.UserID), "")
			auth(req)
			expectStatus(do(req), http.StatusNotFound)
		})
	})

	Describe("clinician clinics listing", func() {
		It("returns the clinics of the requesting clinician", func() {
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinicians/%s/clinics", memberId), "")
			asUser(memberId)(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			relationships := decodeAs[client.ClinicianClinicRelationshipsV1](resp)
			Expect(relationships).To(HaveLen(1))
			Expect(relationships[0].Clinic.Id).To(PointTo(Equal(clinicId)))
			Expect(relationships[0].Clinician.Id).To(PointTo(Equal(memberId)))
		})

		It("rejects requests for other users", func() {
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinicians/%s/clinics", memberId), "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("allows backend services", func() {
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinicians/%s/clinics", memberId), "")
			asServer(req)
			expectStatus(do(req), http.StatusOK)
		})
	})

	Describe("listing all clinicians", func() {
		It("is unreachable: no policy rule allows any caller", func() {
			// GET /v1/clinicians has a handler but auth/policy.rego defines
			// no allow rule for the path, so every caller — including
			// backend services — receives 403. The endpoint is dead code;
			// the Postgres port can likely drop it.
			req := prepareRequest(http.MethodGet, "/v1/clinicians", "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)

			query := url.Values{"createdTimeStart": {time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)}}
			req = prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinicians?%s", query.Encode()), "")
			asServer(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})
})
