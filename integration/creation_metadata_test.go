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

var _ = Describe("Creation Metadata Integration Test", Ordered, func() {
	var clinic client.ClinicV1

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/creation_metadata_fixtures/01_create_clinic.json")
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

	Describe("Server creates patient with creation metadata", func() {
		var patient client.PatientV1

		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients", *clinic.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "./test/creation_metadata_fixtures/02_create_patient_with_metadata.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
		})

		It("Returns creation metadata in the response", func() {
			Expect(patient.CreationMetadata).ToNot(BeNil())
			Expect(patient.CreationMetadata.Integration).ToNot(BeNil())
			Expect(*patient.CreationMetadata.Integration).To(Equal(client.PatientCreationMetadataV1IntegrationRedox))
		})

		It("Returns creation metadata when fetching the patient", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var fetched client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &fetched)).To(Succeed())

			Expect(fetched.CreationMetadata).ToNot(BeNil())
			Expect(fetched.CreationMetadata.Integration).ToNot(BeNil())
			Expect(*fetched.CreationMetadata.Integration).To(Equal(client.PatientCreationMetadataV1IntegrationRedox))
		})

		It("Preserves creation metadata after update", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, endpoint, "./test/creation_metadata_fixtures/04_update_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var updated client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &updated)).To(Succeed())

			Expect(updated.FullName).To(Equal("Metadata Patient Updated"))
			Expect(updated.CreationMetadata).ToNot(BeNil())
			Expect(updated.CreationMetadata.Integration).ToNot(BeNil())
			Expect(*updated.CreationMetadata.Integration).To(Equal(client.PatientCreationMetadataV1IntegrationRedox))
		})
	})

	Describe("Clinician creates patient with creation metadata", func() {
		var patient client.PatientV1

		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients", *clinic.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "./test/creation_metadata_fixtures/03_create_patient_clinician.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
		})

		It("Does not set creation metadata", func() {
			Expect(patient.CreationMetadata).To(BeNil())
		})

		It("Does not return creation metadata when fetching the patient", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var fetched client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &fetched)).To(Succeed())

			Expect(fetched.CreationMetadata).To(BeNil())
		})
	})
})
