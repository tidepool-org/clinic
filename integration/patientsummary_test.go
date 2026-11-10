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

// Summaries pushed for a patient land only in the records whose glycemic ranges they were
// computed with, when tagged with a glycemic ranges key, and in all of them otherwise.
var _ = Describe("Patient Summary Integration Test", Ordered, func() {
	const customKey = "custom:mg/dL:54,70,180,250"
	var clinic client.ClinicV1
	var patient api.PatientV1

	createBody := `{
	  "fullName": "Timothy Bixby",
	  "birthDate": "2008-01-06",
	  "mrn": "0000000003",
	  "email": "test@tidepool.org",
	  "glycemicRanges": {
	    "type": "custom",
	    "custom": {
	      "name": "Mine",
	      "units": "mg/dL",
	      "thresholds": {"veryLow": 54, "low": 70, "high": 180, "veryHigh": 250}
	    }
	  }
	}`

	getPatient := func() api.PatientV1 {
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet,
			fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id), "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		fetched := api.PatientV1{}
		dec := json.NewDecoder(rec.Result().Body)
		Expect(dec.Decode(&fetched)).To(Succeed())
		return fetched
	}

	updateSummary := func(fixture string, query string) int {
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodPost,
			fmt.Sprintf("/v1/patients/%s/summary%s", *patient.Id, query), fixture)
		asServer(req)

		server.ServeHTTP(rec, req)
		return rec.Result().StatusCode
	}

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

	Describe("Create a patient with custom ranges", func() {
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
			Expect(patient.GlycemicRanges.Key).To(Equal(customKey))
		})
	})

	Describe("Update the summary tagged with the standard key", func() {
		It("Succeeds", func() {
			status := updateSummary("./test/patientsummary_fixtures/01_range_set_summary.json",
				"?glycemicRangesKey=preset:adaStandard")
			Expect(status).To(Equal(http.StatusOK))
		})

		It("Stores nothing, as the patient uses other ranges", func() {
			Expect(getPatient().Summary).To(BeNil())
		})
	})

	Describe("Update the summary tagged with the patient's key", func() {
		It("Succeeds", func() {
			status := updateSummary("./test/patientsummary_fixtures/01_range_set_summary.json",
				"?glycemicRangesKey="+customKey)
			Expect(status).To(Equal(http.StatusOK))
		})

		It("Stores the stats without an id and with the optional thresholds absent", func() {
			summary := getPatient().Summary
			Expect(summary).ToNot(BeNil())
			Expect(summary.CgmStats).ToNot(BeNil())
			Expect(summary.CgmStats.Id).To(BeNil())
			Expect(summary.CgmStats.Config.GlycemicRangesKey).To(Equal(customKey))
			Expect(summary.CgmStats.Config.VeryLowGlucoseThreshold).To(BeNil())
			Expect(summary.CgmStats.Config.VeryHighGlucoseThreshold).To(BeNil())
			Expect(summary.CgmStats.Config.LowGlucoseThreshold).To(Equal(3.9))
			Expect(summary.BgmStats).ToNot(BeNil())
			Expect(summary.BgmStats.Id).To(BeNil())
		})
	})

	Describe("Update the summary without a key", func() {
		It("Succeeds", func() {
			status := updateSummary("./test/patientsummary_fixtures/02_standard_summary.json", "")
			Expect(status).To(Equal(http.StatusOK))
		})

		It("Replaces the CGM stats and keeps the BGM stats", func() {
			summary := getPatient().Summary
			Expect(summary).ToNot(BeNil())
			Expect(summary.CgmStats).ToNot(BeNil())
			Expect(summary.CgmStats.Id).To(HaveValue(Equal(api.SummaryIdV1("2fe2488217ee43e1b2e83c2f"))))
			Expect(summary.CgmStats.Config.GlycemicRangesKey).To(Equal("preset:adaStandard"))
			Expect(summary.CgmStats.Config.VeryLowGlucoseThreshold).To(HaveValue(Equal(3.0)))
			Expect(summary.BgmStats).ToNot(BeNil())
			Expect(summary.BgmStats.Id).To(BeNil())
		})
	})

	Describe("Delete the CGM summary", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/patients/%s/summary/cgm", *patient.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Removes the CGM stats and keeps the BGM stats", func() {
			summary := getPatient().Summary
			Expect(summary).ToNot(BeNil())
			Expect(summary.CgmStats).To(BeNil())
			Expect(summary.BgmStats).ToNot(BeNil())
		})

		It("Succeeds again, with nothing left to remove", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/patients/%s/summary/cgm", *patient.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Rejects an unknown summary type", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete,
				fmt.Sprintf("/v1/patients/%s/summary/con", *patient.Id), "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusBadRequest))
		})

		It("Reports no change for a user who is not a patient", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, "/v1/patients/0123456789/summary/cgm", "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
			Expect(rec.Body.Len()).To(BeZero())
		})
	})
})
