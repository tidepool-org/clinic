package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Patient Tags", Ordered, func() {
	var clinic client.ClinicV1
	var tagId string
	var secondTagId string
	var patientAId string
	var patientBId string

	Describe("Create clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/patienttags_fixtures/01_create_clinic.json")
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

	Describe("Create patient tag", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patient_tags", *clinic.Id), "./test/patienttags_fixtures/02_create_tag.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var tag client.PatientTagV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &tag)).To(Succeed())
			Expect(tag.Id).ToNot(BeNil())
			Expect(tag.Name).To(Equal("Tag Alpha"))
			tagId = *tag.Id
		})
	})

	Describe("Create second patient tag", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			body := `{"name": "Tag Beta"}`
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patient_tags", *clinic.Id), strings.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var tag client.PatientTagV1
			respBody, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(respBody, &tag)).To(Succeed())
			Expect(tag.Id).ToNot(BeNil())
			secondTagId = *tag.Id
		})
	})

	Describe("Update patient tag name", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/patient_tags/%s", *clinic.Id, tagId), "./test/patienttags_fixtures/03_update_tag.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Verify updated tag via GetClinic", func() {
		It("Shows updated tag name", func() {
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
			Expect(updatedClinic.PatientTags).ToNot(BeNil())

			found := false
			for _, t := range *updatedClinic.PatientTags {
				if t.Id != nil && *t.Id == tagId {
					Expect(t.Name).To(Equal("Tag Alpha Updated"))
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("Create two patients", func() {
		It("Creates patient A", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id), "./test/patienttags_fixtures/04_create_patient_a.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			Expect(patient.Id).ToNot(BeNil())
			patientAId = *patient.Id
		})

		It("Creates patient B", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id), "./test/patienttags_fixtures/05_create_patient_b.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			Expect(patient.Id).ToNot(BeNil())
			patientBId = *patient.Id
		})
	})

	Describe("Assign tag to specific patients", func() {
		It("Succeeds", func() {
			body := fmt.Sprintf(`["%s", "%s"]`, patientAId, patientBId)
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/assign_tag/%s", *clinic.Id, tagId), strings.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Verify patients have tag assigned", func() {
		It("Patient A has the tag", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, patientAId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			Expect(patient.Tags).ToNot(BeNil())
			Expect(*patient.Tags).To(ContainElement(tagId))
		})
	})

	Describe("Delete tag from specific patients", func() {
		It("Succeeds", func() {
			body := fmt.Sprintf(`["%s"]`, patientAId)
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/delete_tag/%s", *clinic.Id, tagId), strings.NewReader(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Patient A no longer has the tag", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, patientAId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var patient client.PatientV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &patient)).To(Succeed())
			if patient.Tags != nil {
				Expect(*patient.Tags).ToNot(ContainElement(tagId))
			}
		})
	})

	Describe("Delete patient tag", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/patient_tags/%s", *clinic.Id, secondTagId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})
	})

	Describe("Verify deleted tag gone from clinic", func() {
		It("Tag is removed", func() {
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

			if updatedClinic.PatientTags != nil {
				for _, t := range *updatedClinic.PatientTags {
					Expect(*t.Id).ToNot(Equal(secondTagId))
				}
			}
		})
	})
})
