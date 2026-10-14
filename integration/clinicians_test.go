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

var _ = Describe("Clinician Management", Ordered, func() {
	var clinic client.ClinicV1
	var clinicianId string

	Describe("Create clinic for clinician tests", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/clinicians_fixtures/01_create_clinic.json")
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

	Describe("Create a second clinician", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "./test/clinicians_fixtures/02_create_clinician.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var created client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &created)).To(Succeed())
			Expect(created.Id).ToNot(BeNil())
			clinicianId = *created.Id
			Expect(created.Roles).To(ContainElement("CLINIC_MEMBER"))
		})
	})

	Describe("List clinicians for the clinic", func() {
		It("Returns both admin and member", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinicians client.CliniciansV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinicians)).To(Succeed())
			Expect(len(clinicians)).To(BeNumerically(">=", 2))
		})
	})

	Describe("List clinicians filtered by role", func() {
		It("Returns only CLINIC_MEMBER clinicians", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians?role=CLINIC_MEMBER", *clinic.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinicians client.CliniciansV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinicians)).To(Succeed())
			Expect(len(clinicians)).To(BeNumerically(">=", 1))
			for _, c := range clinicians {
				Expect(c.Roles).To(ContainElement("CLINIC_MEMBER"))
			}
		})
	})

	Describe("Get clinician by ID", func() {
		It("Returns the clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, clinicianId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var clinicianResp client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinicianResp)).To(Succeed())
			Expect(clinicianResp.Id).ToNot(BeNil())
			Expect(*clinicianResp.Id).To(Equal(clinicianId))
		})
	})

	Describe("Update clinician role to CLINIC_ADMIN", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, clinicianId), "./test/clinicians_fixtures/03_update_clinician.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var updated client.ClinicianV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &updated)).To(Succeed())
			Expect(updated.Roles).To(ContainElement("CLINIC_ADMIN"))
		})
	})

	Describe("List clinics for a clinician", func() {
		It("Returns clinic details", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinicians/%s/clinics", integrationTest.TestUserId), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var relationships client.ClinicianClinicRelationshipsV1
			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &relationships)).To(Succeed())
			Expect(len(relationships)).To(BeNumerically(">=", 1))
		})
	})

	Describe("Delete clinician", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, clinicianId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Returns 404 for deleted clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", *clinic.Id, clinicianId), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
