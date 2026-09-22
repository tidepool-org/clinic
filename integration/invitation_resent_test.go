package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Invitation Resent Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics",
				"./test/common_fixtures/01_create_clinic.json")
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

	Describe("Create Patient", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id),
				"./test/common_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})
	})

	Describe("Record Invitation Resent", func() {
		endpoint := func() string {
			return fmt.Sprintf("/v1/clinics/%s/patients/%s/invitation_resent",
				*clinic.Id, *patient.Id)
		}

		It("Succeeds for a backend service", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint(), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Is forbidden for a clinician", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint(), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusForbidden))
		})

		It("Returns not found for an unknown patient", func() {
			rec := httptest.NewRecorder()
			unknown := fmt.Sprintf("/v1/clinics/%s/patients/0000000000/invitation_resent",
				*clinic.Id)
			req := prepareRequest(http.MethodPost, unknown, "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
