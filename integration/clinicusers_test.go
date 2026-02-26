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

var _ = Describe("Clinic User Operations", Ordered, func() {
	var clinic client.ClinicV1
	var patientId string
	var clinicianUserId = "4444444444"

	Describe("Create clinic with patient and clinician", func() {
		It("Creates clinic", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/clinicusers_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})

		It("Creates patient", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id), "./test/clinicusers_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			Expect(patient.Id).ToNot(BeNil())
			patientId = *patient.Id
		})

		It("Creates clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "./test/clinicusers_fixtures/03_create_clinician.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Update clinic user details", func() {
		It("Updates email for the clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/users/%s/clinics", clinicianUserId), "./test/clinicusers_fixtures/04_update_user_details.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Verify email updated on clinician record", func() {
		It("Shows updated email", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, clinicianUserId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var c client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &c)).To(Succeed())
			Expect(c.Email).To(Equal("updated@tidepool.org"))
		})
	})

	Describe("Delete user from all clinics", func() {
		It("Succeeds for the clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/users/%s/clinics", clinicianUserId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Verify clinician removed", func() {
		It("Returns 404", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, clinicianUserId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	_ = patientId
})
