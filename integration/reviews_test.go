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
	integrationTest "github.com/tidepool-org/clinic/integration/test"
)

var _ = Describe("Patient Reviews", Ordered, func() {
	var clinic client.ClinicV1
	var patientId string

	Describe("Create clinic and patient", func() {
		It("Creates clinic", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/reviews_fixtures/01_create_clinic.json")
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
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id), "./test/reviews_fixtures/02_create_patient.json")
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
	})

	Describe("Add a review", func() {
		It("Succeeds as clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/patients/%s/reviews", *clinic.Id, patientId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var reviews client.PatientReviewsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &reviews)).To(Succeed())
			Expect(len(reviews)).To(Equal(1))
			Expect(reviews[0].ClinicianId).To(Equal(integrationTest.TestUserId))
		})
	})

	Describe("Verify review appears on patient", func() {
		It("Patient has review", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, patientId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			Expect(len(patient.Reviews)).To(Equal(1))
			Expect(patient.Reviews[0].ClinicianId).To(Equal(integrationTest.TestUserId))
		})
	})

	Describe("Delete review", func() {
		It("Succeeds as same clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/patients/%s/reviews", *clinic.Id, patientId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var reviews client.PatientReviewsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &reviews)).To(Succeed())
			Expect(len(reviews)).To(Equal(0))
		})
	})

	Describe("Verify review removed", func() {
		It("Patient has no reviews", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, patientId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			Expect(len(patient.Reviews)).To(Equal(0))
		})
	})
})
