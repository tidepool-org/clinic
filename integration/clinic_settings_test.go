package integration_test

import (
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
)

// Pins MRN and EHR settings behavior (404 before set, backend-only writes,
// member reads, nested round-trips) and the EHR sync trigger endpoints.
var _ = Describe("Clinic Settings", Ordered, func() {
	var auth func(*http.Request)
	var clinicId string

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicId = *createClinic(auth).Id
	})

	Describe("MRN settings", func() {
		endpoint := func() string {
			return fmt.Sprintf("/v1/clinics/%s/settings/mrn", clinicId)
		}

		It("returns 404 before the settings are set", func() {
			req := prepareRequest(http.MethodGet, endpoint(), "")
			auth(req)
			expectStatus(do(req), http.StatusNotFound)
		})

		It("is updatable by backend services only", func() {
			body := map[string]interface{}{"required": true, "unique": false}

			req := prepareRequestWithBody(http.MethodPut, endpoint(), jsonBody(body))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)

			req = prepareRequestWithBody(http.MethodPut, endpoint(), jsonBody(body))
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			settings := decodeAs[client.MrnSettingsV1](resp)
			Expect(settings.Required).To(BeTrue())
			Expect(settings.Unique).To(BeFalse())
		})

		It("is readable by clinic members", func() {
			member := newStubUser()
			createClinicianDirect(clinicId, member.UserID, "CLINIC_MEMBER")

			req := prepareRequest(http.MethodGet, endpoint(), "")
			asUser(member.UserID)(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			settings := decodeAs[client.MrnSettingsV1](resp)
			Expect(settings.Required).To(BeTrue())
		})

		It("is not readable by non-members", func() {
			outsider := newStubUser()
			req := prepareRequest(http.MethodGet, endpoint(), "")
			asUser(outsider.UserID)(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})

	Describe("EHR settings", func() {
		endpoint := func() string {
			return fmt.Sprintf("/v1/clinics/%s/settings/ehr", clinicId)
		}

		It("returns 404 before the settings are set", func() {
			req := prepareRequest(http.MethodGet, endpoint(), "")
			auth(req)
			expectStatus(do(req), http.StatusNotFound)
		})

		// The source id must not collide with the shared redox fixture's
		// source id: clinic lookup during EHR message matching is by source
		// id, and a duplicate would break the redox integration specs.
		sourceId := fmt.Sprintf("00000000-0000-4000-8000-%012s", uniqueId())
		settingsBody := func() io.Reader {
			return fixtureWithOverrides("./test/redox_fixtures/02_enable_redox.json", map[string]interface{}{
				"sourceId": sourceId,
			})
		}

		It("rejects updates from clinicians", func() {
			req := prepareRequestWithBody(http.MethodPut, endpoint(), settingsBody())
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("round-trips the full settings document", func() {
			req := prepareRequestWithBody(http.MethodPut, endpoint(), settingsBody())
			asServer(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)
			settings := decodeAs[client.EhrSettingsV1](resp)

			Expect(settings.Enabled).To(BeTrue())
			Expect(string(settings.Provider)).To(Equal("redox"))
			Expect(settings.SourceId).To(Equal(sourceId))
			Expect(string(settings.MrnIdType)).To(Equal("MR"))
			Expect(settings.DestinationIds).ToNot(BeNil())
			Expect(settings.DestinationIds.Flowsheet).To(Equal("af394f14-b34a-464f-8d24-895f370af4c9"))
			Expect(settings.ProcedureCodes.EnableSummaryReports).To(PointTo(Equal("49086-2")))
			Expect(string(settings.ScheduledReports.Cadence)).To(Equal("14d"))
			Expect(settings.ScheduledReports.OnUploadEnabled).To(BeTrue())

			req = prepareRequest(http.MethodGet, endpoint(), "")
			auth(req)
			resp = do(req)
			expectStatus(resp, http.StatusOK)
			fetched := decodeAs[client.EhrSettingsV1](resp)
			Expect(fetched.Enabled).To(BeTrue())
			Expect(fetched.Tags).ToNot(BeNil())
		})
	})

	Describe("EHR sync triggers", func() {
		It("is limited to backend services", func() {
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/ehr/sync", clinicId), "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("accepts clinic-level sync requests", func() {
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/ehr/sync", clinicId), "")
			asServer(req)
			expectStatus(do(req), http.StatusAccepted)
		})

		It("accepts patient-level sync requests for patients without subscriptions", func() {
			patient := createCustodialPatient(clinicId, auth, nil)
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/patients/%s/ehr/sync", *patient.Id), "")
			asServer(req)
			expectStatus(do(req), http.StatusAccepted)
		})
	})
})
