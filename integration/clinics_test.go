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

// Pins clinic lifecycle behavior: creation defaults (share code, tier),
// share-code lookup, backend-only listing, updates, tier changes, suppressed
// notifications, membership restrictions and the non-empty deletion guard.
var _ = Describe("Clinics", Ordered, func() {
	var adminId string
	var auth func(*http.Request)
	var clinic client.ClinicV1

	getClinic := func(clinicId string, reqAuth func(*http.Request)) *http.Response {
		GinkgoHelper()
		req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", clinicId), "")
		reqAuth(req)
		return do(req)
	}

	BeforeAll(func() {
		admin := newStubUser()
		adminId = admin.UserID
		auth = asUser(adminId)
		clinic = createClinic(auth)
	})

	Describe("creation", func() {
		It("assigns a share code, the default tier and a creation time", func() {
			Expect(clinic.Id).ToNot(BeNil())
			Expect(clinic.ShareCode).To(PointTo(MatchRegexp(`^[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{4}-[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{4}-[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{4}$`)))
			Expect(clinic.Tier).To(PointTo(Equal("tier0100")))
			Expect(clinic.TierDescription).To(PointTo(Equal("Free")))
			Expect(clinic.CreatedTime).ToNot(BeNil())
			// Clinics created through the API are born migrated, so they are
			// not subject to legacy patient migration.
			Expect(clinic.CanMigrate).To(PointTo(BeFalse()))
		})

		It("makes the creator a clinic admin", func() {
			clinician := getClinician(*clinic.Id, adminId)
			Expect(clinician.Roles).To(ContainElement("CLINIC_ADMIN"))
		})
	})

	Describe("fetching", func() {
		It("allows any authenticated user to fetch a clinic by id", func() {
			// Clinic reads are not restricted to members.
			outsider := newStubUser()
			resp := getClinic(*clinic.Id, asUser(outsider.UserID))
			expectStatus(resp, http.StatusOK)
			fetched := decodeAs[client.ClinicV1](resp)
			Expect(fetched.Id).To(Equal(clinic.Id))
		})

		It("finds clinics by share code", func() {
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/share_code/%s", *clinic.ShareCode), "")
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			fetched := decodeAs[client.ClinicV1](resp)
			Expect(fetched.Id).To(Equal(clinic.Id))
		})

		It("returns 404 for unknown share codes", func() {
			req := prepareRequest(http.MethodGet, "/v1/clinics/share_code/XXXX-XXXX-XXXX", "")
			auth(req)
			expectStatus(do(req), http.StatusNotFound)
		})
	})

	Describe("listing", func() {
		It("is limited to backend services", func() {
			req := prepareRequest(http.MethodGet, "/v1/clinics", "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("filters by share code", func() {
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics?shareCode=%s", *clinic.ShareCode), "")
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			list := decodeAs[[]client.ClinicV1](resp)
			Expect(list).To(HaveLen(1))
			Expect(list[0].Id).To(Equal(clinic.Id))
		})

		It("filters by creation time", func() {
			query := url.Values{
				"shareCode":        {*clinic.ShareCode},
				"createdTimeStart": {time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)},
			}
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics?%s", query.Encode()), "")
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			Expect(decodeAs[[]client.ClinicV1](resp)).To(BeEmpty())
		})
	})

	Describe("updates", func() {
		It("updates profile fields and preserves the share code", func() {
			newName := fmt.Sprintf("Updated Clinic %s", uniqueId())
			body := fixtureWithOverrides("./test/common_fixtures/01_create_clinic.json", map[string]interface{}{
				"name": newName,
				"city": "Palo Alto",
			})
			req := prepareRequestWithBody(http.MethodPut, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), body)
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			updated := decodeAs[client.ClinicV1](resp)
			Expect(updated.Name).To(Equal(newName))
			Expect(updated.City).To(PointTo(Equal("Palo Alto")))
			Expect(updated.ShareCode).To(Equal(clinic.ShareCode))
		})

		It("rejects updates from clinic members", func() {
			member := newStubUser()
			createClinicianDirect(*clinic.Id, member.UserID, "CLINIC_MEMBER")

			body := fixtureWithOverrides("./test/common_fixtures/01_create_clinic.json", map[string]interface{}{
				"name": fmt.Sprintf("Member Update %s", uniqueId()),
			})
			req := prepareRequestWithBody(http.MethodPut, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), body)
			asUser(member.UserID)(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})

	Describe("tier", func() {
		It("is updatable by backend services only", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/tier", *clinic.Id)

			req := prepareRequestWithBody(http.MethodPost, endpoint, jsonBody(map[string]interface{}{"tier": "tier0300"}))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)

			req = prepareRequestWithBody(http.MethodPost, endpoint, jsonBody(map[string]interface{}{"tier": "tier0300"}))
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			resp := getClinic(*clinic.Id, auth)
			expectStatus(resp, http.StatusOK)
			fetched := decodeAs[client.ClinicV1](resp)
			Expect(fetched.Tier).To(PointTo(Equal("tier0300")))
			Expect(fetched.TierDescription).To(PointTo(Equal("Premium")))
		})
	})

	Describe("suppressed notifications", func() {
		It("is updatable by clinic admins and visible on the clinic", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/suppressed_notifications", *clinic.Id)
			body := map[string]interface{}{
				"suppressedNotifications": map[string]interface{}{"patientClinicInvitation": true},
			}

			req := prepareRequestWithBody(http.MethodPost, endpoint, jsonBody(body))
			auth(req)
			expectStatus(do(req), http.StatusOK)

			resp := getClinic(*clinic.Id, auth)
			expectStatus(resp, http.StatusOK)
			fetched := decodeAs[client.ClinicV1](resp)
			Expect(fetched.SuppressedNotifications).ToNot(BeNil())
			Expect(fetched.SuppressedNotifications.PatientClinicInvitation).To(PointTo(BeTrue()))
		})
	})

	Describe("membership restrictions", func() {
		endpoint := func() string {
			return fmt.Sprintf("/v1/clinics/%s/membership_restrictions", *clinic.Id)
		}

		It("is updatable by backend services only", func() {
			body := map[string]interface{}{
				"restrictions": []map[string]interface{}{
					{"emailDomain": "example.org", "requiredIdp": "okta"},
				},
			}

			req := prepareRequestWithBody(http.MethodPut, endpoint(), jsonBody(body))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)

			req = prepareRequestWithBody(http.MethodPut, endpoint(), jsonBody(body))
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			restrictions := decodeAs[client.MembershipRestrictionsV1](resp)
			Expect(restrictions.Restrictions).To(PointTo(HaveLen(1)))
		})

		It("is listable by clinic admins", func() {
			req := prepareRequest(http.MethodGet, endpoint(), "")
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			restrictions := decodeAs[client.MembershipRestrictionsV1](resp)
			Expect(restrictions.Restrictions).To(PointTo(HaveLen(1)))
			Expect((*restrictions.Restrictions)[0].EmailDomain).To(Equal("example.org"))
			Expect((*restrictions.Restrictions)[0].RequiredIdp).To(PointTo(Equal("okta")))
		})

		It("is cleared by an empty update", func() {
			req := prepareRequestWithBody(http.MethodPut, endpoint(), jsonBody(map[string]interface{}{}))
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			restrictions := decodeAs[client.MembershipRestrictionsV1](resp)
			if restrictions.Restrictions != nil {
				Expect(*restrictions.Restrictions).To(BeEmpty())
			}
		})
	})

	Describe("deletion", func() {
		var deletable client.ClinicV1
		var patient client.PatientV1

		BeforeAll(func() {
			deletable = createClinic(auth)
			patient = createCustodialPatient(*deletable.Id, auth, nil)
		})

		It("is blocked while the clinic has patients", func() {
			// Deleting a non-empty clinic is a 400, not a 409.
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s", *deletable.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusBadRequest)
		})

		It("succeeds once the patients are removed", func() {
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/patients/%s", *deletable.Id, *patient.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusNoContent)

			req = prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s", *deletable.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusNoContent)

			expectStatus(getClinic(*deletable.Id, auth), http.StatusNotFound)
		})
	})
})
