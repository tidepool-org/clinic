package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Clinic Settings", Ordered, func() {
	var clinic client.ClinicV1

	Describe("Create clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/settings_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("EHR Settings", func() {
		It("Returns 404 when none set", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/settings/ehr", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})

		It("Updates EHR settings", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/settings/ehr", *clinic.Id), "./test/settings_fixtures/02_ehr_settings.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Returns EHR settings after update", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/settings/ehr", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var settings client.EhrSettingsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &settings)).To(Succeed())
			Expect(settings.Enabled).To(BeTrue())
			Expect(string(settings.Provider)).To(Equal("redox"))
		})
	})

	Describe("MRN Settings", func() {
		It("Returns 404 when none set", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/settings/mrn", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})

		It("Updates MRN settings", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/settings/mrn", *clinic.Id), "./test/settings_fixtures/03_mrn_settings.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Returns MRN settings after update", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/settings/mrn", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var settings client.MrnSettingsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &settings)).To(Succeed())
			Expect(settings.Required).To(BeTrue())
			Expect(settings.Unique).To(BeTrue())
		})
	})

	Describe("Patient Count Settings", func() {
		It("Updates patient count settings", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/settings/patient_count", *clinic.Id), "./test/settings_fixtures/04_patient_count_settings.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Returns patient count settings after update", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/settings/patient_count", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var settings client.PatientCountSettingsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &settings)).To(Succeed())
			Expect(settings.HardLimit).ToNot(BeNil())
			Expect(settings.HardLimit.Plan).To(Equal(250))
		})
	})

	Describe("Update tier", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/tier", *clinic.Id), "./test/settings_fixtures/05_update_tier.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Reflects updated tier", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var updatedClinic client.ClinicV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &updatedClinic)).To(Succeed())
			Expect(updatedClinic.Tier).ToNot(BeNil())
			Expect(*updatedClinic.Tier).To(Equal("tier0100"))
		})
	})

	Describe("Update suppressed notifications", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/suppressed_notifications", *clinic.Id), "./test/settings_fixtures/06_suppressed_notifications.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Reflects suppressed notifications", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var updatedClinic client.ClinicV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &updatedClinic)).To(Succeed())
			Expect(updatedClinic.SuppressedNotifications).ToNot(BeNil())
			Expect(updatedClinic.SuppressedNotifications.PatientClinicInvitation).ToNot(BeNil())
			Expect(*updatedClinic.SuppressedNotifications.PatientClinicInvitation).To(BeTrue())
		})
	})

	Describe("Get clinic by share code", func() {
		It("Returns clinic by valid share code", func() {
			// First get the share code from the clinic
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var c client.ClinicV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &c)).To(Succeed())
			Expect(c.ShareCode).ToNot(BeNil())

			// Now look up by share code
			rec2 := httptest.NewRecorder()
			req2 := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/share_code/%s", *c.ShareCode), "")
			asServer(req2)

			server.ServeHTTP(rec2, req2)
			Expect(rec2.Result()).ToNot(BeNil())
			Expect(rec2.Result().StatusCode).To(Equal(http.StatusOK))

			var found client.ClinicV1
			body2, err := io.ReadAll(rec2.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body2, &found)).To(Succeed())
			Expect(*found.Id).To(Equal(*clinic.Id))
		})

		It("Returns 404 for invalid share code", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, "/v1/clinics/share_code/nonexistent-code-xyz", "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Membership restrictions", func() {
		It("Returns empty initially", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/membership_restrictions", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var restrictions client.MembershipRestrictionsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &restrictions)).To(Succeed())
		})

		It("Updates membership restrictions", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/membership_restrictions", *clinic.Id), "./test/settings_fixtures/07_membership_restrictions.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Returns updated membership restrictions", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/membership_restrictions", *clinic.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var restrictions client.MembershipRestrictionsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &restrictions)).To(Succeed())
			Expect(restrictions.Restrictions).ToNot(BeNil())
			Expect(len(*restrictions.Restrictions)).To(Equal(1))
			Expect((*restrictions.Restrictions)[0].EmailDomain).To(Equal("example.com"))
		})
	})
})
