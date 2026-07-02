package integration_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

// Pins patient counts and count settings. Noteworthy: the country gate
// compares against the country code "US", so clinics created with the
// fixture's "USA" country are treated as non-US and are exempt from patient
// count limits — the enforcement specs below create clinics with "US".
var _ = Describe("Patient Counts", Ordered, func() {
	var auth func(*http.Request)
	var clinicId string

	getCount := func(cid string, reqAuth func(*http.Request)) client.PatientCountV1 {
		GinkgoHelper()
		req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patient_count", cid), "")
		reqAuth(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		return decodeAs[client.PatientCountV1](resp)
	}

	refreshCount := func(cid string) {
		GinkgoHelper()
		req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patient_count/refresh", cid), "")
		asServer(req)
		expectStatus(do(req), http.StatusOK)
	}

	getSettings := func(cid string) client.PatientCountSettingsV1 {
		GinkgoHelper()
		req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/settings/patient_count", cid), "")
		asServer(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		return decodeAs[client.PatientCountSettingsV1](resp)
	}

	putSettings := func(cid string, settings map[string]interface{}, expectedStatus int) {
		GinkgoHelper()
		req := prepareRequestWithBody(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/settings/patient_count", cid), jsonBody(settings))
		asServer(req)
		expectStatus(do(req), expectedStatus)
	}

	createUSClinic := func(reqAuth func(*http.Request)) string {
		GinkgoHelper()
		body := fixtureWithOverrides("./test/common_fixtures/01_create_clinic.json", map[string]interface{}{
			"name":    fmt.Sprintf("US Clinic %s", uniqueId()),
			"country": "US",
		})
		req := prepareRequestWithBody(http.MethodPost, "/v1/clinics", body)
		reqAuth(req)
		resp := do(req)
		expectStatus(resp, http.StatusOK)
		return *decodeAs[client.ClinicV1](resp).Id
	}

	BeforeAll(func() {
		admin := newStubUser()
		auth = asUser(admin.UserID)
		clinicId = createUSClinic(auth)
	})

	Describe("patient count", func() {
		It("is zero for a fresh clinic", func() {
			count := getCount(clinicId, auth)
			Expect(count.Total).To(Equal(0))
			Expect(count.Demo).To(Equal(0))
			Expect(count.Plan).To(Equal(0))
		})

		It("increments as patients are created", func() {
			createCustodialPatient(clinicId, auth, nil)
			user := newStubUser()
			createPatientFromUser(clinicId, user.UserID, asServer, map[string]interface{}{"birthDate": "1980-04-04"})

			count := getCount(clinicId, auth)
			Expect(count.Total).To(Equal(2))
			Expect(count.Demo).To(Equal(0))
			Expect(count.Plan).To(Equal(2))
		})

		It("decrements after a patient is deleted and the count is refreshed", func() {
			doomed := createCustodialPatient(clinicId, auth, nil)
			Expect(getCount(clinicId, auth).Total).To(Equal(3))

			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicId, *doomed.Id), "")
			auth(req)
			expectStatus(do(req), http.StatusNoContent)

			refreshCount(clinicId)
			count := getCount(clinicId, auth)
			Expect(count.Total).To(Equal(2))
			Expect(count.Plan).To(Equal(2))
		})

		It("rejects refreshes from clinicians", func() {
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patient_count/refresh", clinicId), "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})

	Describe("patient count settings", func() {
		It("defaults to a 250 patient hard limit for US default-tier clinics", func() {
			settings := getSettings(clinicId)
			Expect(settings.HardLimit).ToNot(BeNil())
			Expect(settings.HardLimit.Plan).To(Equal(250))
			Expect(settings.SoftLimit).To(BeNil())
		})

		It("resolves to unlimited for non-US clinics", func() {
			// The country gate compares to the code "US"; the shared fixture's
			// "USA" therefore also resolves to unlimited.
			nonUS := createClinic(auth)
			settings := getSettings(*nonUS.Id)
			Expect(settings.HardLimit).To(BeNil())
			Expect(settings.SoftLimit).To(BeNil())
		})

		It("round-trips explicit settings", func() {
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
			putSettings(clinicId, map[string]interface{}{
				"hardLimit": map[string]interface{}{
					"plan":      3,
					"startDate": start.Format(time.RFC3339),
					"endDate":   end.Format(time.RFC3339),
				},
			}, http.StatusOK)

			settings := getSettings(clinicId)
			Expect(settings.HardLimit).ToNot(BeNil())
			Expect(settings.HardLimit.Plan).To(Equal(3))
			Expect(settings.HardLimit.StartDate).To(HaveValue(Equal(start.Format(time.RFC3339))))
			Expect(settings.HardLimit.EndDate).To(HaveValue(Equal(end.Format(time.RFC3339))))
		})

		It("rejects negative limits and inverted date windows", func() {
			putSettings(clinicId, map[string]interface{}{
				"hardLimit": map[string]interface{}{"plan": -1},
			}, http.StatusBadRequest)

			putSettings(clinicId, map[string]interface{}{
				"hardLimit": map[string]interface{}{
					"plan":      3,
					"startDate": "2027-01-01T00:00:00Z",
					"endDate":   "2026-01-01T00:00:00Z",
				},
			}, http.StatusBadRequest)
		})

		It("rejects updates from clinicians", func() {
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/settings/patient_count", clinicId),
				jsonBody(map[string]interface{}{"hardLimit": map[string]interface{}{"plan": 3}}))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})

	Describe("hard limit enforcement", func() {
		limitSettings := func(plan int, overrides map[string]interface{}) map[string]interface{} {
			hardLimit := map[string]interface{}{"plan": plan}
			for k, v := range overrides {
				hardLimit[k] = v
			}
			return map[string]interface{}{"hardLimit": hardLimit}
		}

		It("rejects custodial patients beyond the plan limit with 402", func() {
			cid := createUSClinic(auth)
			putSettings(cid, limitSettings(1, nil), http.StatusOK)

			createCustodialPatient(cid, auth, nil)

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients", cid),
				jsonBody(map[string]interface{}{
					"fullName":  "Over Limit " + uniqueId(),
					"birthDate": "1990-01-01",
				}))
			auth(req)
			expectStatus(do(req), http.StatusPaymentRequired)
		})

		It("allows non-custodial patients beyond the limit", func() {
			cid := createUSClinic(auth)
			putSettings(cid, limitSettings(1, nil), http.StatusOK)

			createCustodialPatient(cid, auth, nil)

			user := newStubUser()
			createPatientFromUser(cid, user.UserID, asServer, map[string]interface{}{"birthDate": "1985-03-03"})
		})

		It("does not enforce limits for non-US clinics", func() {
			nonUS := createClinic(auth)
			putSettings(*nonUS.Id, limitSettings(1, nil), http.StatusOK)

			createCustodialPatient(*nonUS.Id, auth, nil)
			createCustodialPatient(*nonUS.Id, auth, nil)
		})

		It("does not enforce limits for non-default tiers", func() {
			cid := createUSClinic(auth)
			putSettings(cid, limitSettings(1, nil), http.StatusOK)

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/tier", cid),
				jsonBody(map[string]interface{}{"tier": "tier0300"}))
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			createCustodialPatient(cid, auth, nil)
			createCustodialPatient(cid, auth, nil)
		})

		It("does not enforce limits outside the configured window", func() {
			cid := createUSClinic(auth)
			putSettings(cid, limitSettings(1, map[string]interface{}{
				"startDate": "2020-01-01T00:00:00Z",
				"endDate":   "2021-01-01T00:00:00Z",
			}), http.StatusOK)

			createCustodialPatient(cid, auth, nil)
			createCustodialPatient(cid, auth, nil)
		})
	})
})
