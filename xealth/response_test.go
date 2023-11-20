package xealth_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/clinic/xealth"
	"github.com/tidepool-org/clinic/xealth_models"
)

var _ = Describe("Response Builder", func() {
	Describe("Patient Flow", func() {
		It("Returns the expected initial response", func() {
			data := xealth.PatientFormData{}
			data.Patient.Email = "james.jellyfish@tidepool.org"
			response, err := xealth.NewPatientFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithData(data).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildInitialResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/expected_initial_response_patient_flow.json")
		})

		It("Returns the expected subsequent response when validation fails", func() {
			userInput := map[string]interface{}{
				"patient": map[string]interface{}{
					"email": "james",
				},
			}
			response, err := xealth.NewPatientFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidation().
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/expected_subsequent_response_patient_flow_validation_fail.json")
		})

		It("Returns the expected final response when validation succeeds", func() {
			userInput := map[string]interface{}{
				"patient": map[string]interface{}{
					"email": "james.jellyfish@tidepool.org",
				},
			}
			response, err := xealth.NewPatientFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidation().
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()

			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchString(response, []byte(`{}`))
		})
	})

	Describe("Guardian Flow", func() {
		It("Returns the expected initial response", func() {
			response, err := xealth.NewGuardianFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildInitialResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/expected_initial_response_guardian_flow.json")
		})

		It("Returns the expected subsequent response when validation fails for all fields", func() {
			userInput := map[string]interface{}{}
			response, err := xealth.NewGuardianFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidation().
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/expected_subsequent_response_guardian_flow_validation_fail.json")
		})

		It("Returns the expected final response when validation succeeds", func() {
			userInput := map[string]interface{}{
				"guardian": map[string]interface{}{
					"firstName": "James",
					"lastName":  "Jellyfish",
					"email":     "james.jellyfish@tidepool.org",
				},
			}
			response, err := xealth.NewGuardianFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidation().
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()

			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchString(response, []byte(`{}`))
		})
	})
})

func ExpectResponseToMatchFixture(response *xealth_models.PreorderFormResponse, fixturePath string) {
	fixture, err := test.LoadFixture(fixturePath)
	Expect(err).ToNot(HaveOccurred())
	ExpectResponseToMatchString(response, fixture)
}

func ExpectResponseToMatchString(response *xealth_models.PreorderFormResponse, expected []byte) {
	actual, err := json.Marshal(response)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())

	Expect(actual).To(MatchJSON(expected))
}
