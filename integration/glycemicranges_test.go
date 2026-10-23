package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
)

var _ = Describe("Glycemic Ranges Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	// A bogus key is sent with every request to verify that the server derives it.
	createBody := `{
	  "fullName": "Timothy Bixby",
	  "birthDate": "2008-01-06",
	  "mrn": "0000000002",
	  "email": "test@tidepool.org",
	  "glycemicRanges": {"type": "preset", "preset": "adaHighRisk", "key": "bogus"}
	}`
	updateBody := `{
	  "fullName": "Timothy Bixby",
	  "birthDate": "2008-01-06",
	  "mrn": "0000000002",
	  "email": "test@tidepool.org",
	  "glycemicRanges": {
	    "type": "custom",
	    "key": "bogus",
	    "custom": {
	      "name": "Mine",
	      "units": "mg/dL",
	      "thresholds": {"veryLow": 54, "low": 70, "high": 180, "veryHigh": 250}
	    }
	  }
	}`

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics",
				"./test/common_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Create a patient with preset ranges", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id),
				strings.NewReader(createBody))
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).ToNot(BeNil())
		})

		It("Derives the key from the preset", func() {
			Expect(patient.GlycemicRanges).ToNot(BeNil())
			Expect(patient.GlycemicRanges.Key).To(Equal("preset:adaHighRisk"))
		})
	})

	Describe("Update the patient to custom ranges", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id),
				strings.NewReader(updateBody))
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
		})

		It("Derives the key from the custom thresholds", func() {
			Expect(patient.GlycemicRanges).ToNot(BeNil())
			Expect(patient.GlycemicRanges.Key).To(Equal("custom:mg/dL:54,70,180,250"))
		})

		It("Returns the key when the patient is fetched", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet,
				fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			fetched := api.PatientV1{}
			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&fetched)).To(Succeed())
			Expect(fetched.GlycemicRanges.Key).To(Equal("custom:mg/dL:54,70,180,250"))
		})
	})
})
