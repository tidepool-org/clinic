package xealth_test

import (
	"encoding/json"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/clinic/xealth"
	xealthTest "github.com/tidepool-org/clinic/xealth/test"
	"github.com/tidepool-org/clinic/xealth_client"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

var _ = Describe("Response Builder", func() {
	var ctrl *gomock.Controller
	var users *patientsTest.MockUserService

	BeforeEach(func() {
		tb := GinkgoT()
		ctrl = gomock.NewController(tb)
		users = patientsTest.NewMockUserService(ctrl)
	})

	Describe("Patient Flow", func() {
		It("Returns the expected initial response", func() {
			data := xealth.PatientFormData{}
			data.Patient.Email = "james.jellyfish@tidepool.org"
			response, err := xealth.NewPatientFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithData(data).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				WithTags(xealthTest.Tags()).
				BuildInitialResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/fixtures/expected_initial_response_patient_flow.json")
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
				WithDataValidator(xealth.NewPatientDataValidator(users)).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/fixtures/expected_subsequent_response_patient_flow_validation_fail.json")
		})

		It("Returns the expected final response when validation succeeds", func() {
			userInput := map[string]interface{}{
				"patient": map[string]interface{}{
					"email": "james.jellyfish@tidepool.org",
				},
			}
			users.EXPECT().
				GetUser("james.jellyfish@tidepool.org").
				Return(nil, nil)

			response, err := xealth.NewPatientFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidator(xealth.NewPatientDataValidator(users)).
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
				WithTags(xealthTest.Tags()).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildInitialResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/fixtures/expected_initial_response_guardian_flow.json")
		})

		It("Returns the expected subsequent response when validation fails for all fields", func() {
			userInput := map[string]interface{}{}

			response, err := xealth.NewGuardianFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidator(xealth.NewGuardianDataValidator(users)).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/fixtures/expected_subsequent_response_guardian_flow_validation_fail.json")
		})

		It("Returns the expected subsequent response when validation fails with duplicate email", func() {
			email := "james.jellyfish@tidepool.org"
			users.EXPECT().GetUser(email).Return(&shoreline.UserData{
				UserID:        "12345678",
				Username:      email,
				Emails:        []string{email},
				EmailVerified: true,
			}, nil)

			userInput := map[string]interface{}{
				"guardian": map[string]interface{}{
					"firstName": "James",
					"lastName":  "Jellyfish",
					"email":     email,
				},
			}

			response, err := xealth.NewGuardianFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidator(xealth.NewGuardianDataValidator(users)).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()
			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchFixture(response, "test/fixtures/expected_subsequent_response_guardian_flow_duplicate_email.json")
		})

		It("Returns the expected final response when validation succeeds", func() {
			userInput := map[string]interface{}{
				"guardian": map[string]interface{}{
					"firstName": "James",
					"lastName":  "Jellyfish",
					"email":     "james.jellyfish@tidepool.org",
				},
			}
			users.EXPECT().
				GetUser("james.jellyfish@tidepool.org").
				Return(nil, nil)

			response, err := xealth.NewGuardianFlowResponseBuilder().
				WithDataTrackingId("1234567890").
				WithUserInput(&userInput).
				WithDataValidator(xealth.NewGuardianDataValidator(users)).
				WithRenderedTitleTemplate(xealth.FormTitlePatientNameTemplate, "James Jellyfish").
				BuildSubsequentResponse()

			Expect(err).ToNot(HaveOccurred())
			ExpectResponseToMatchString(response, []byte(`{}`))
		})
	})
})

func ExpectResponseToMatchFixture(response *xealth_client.PreorderFormResponse, fixturePath string) {
	fixture, err := test.LoadFixture(fixturePath)
	Expect(err).ToNot(HaveOccurred())
	ExpectResponseToMatchString(response, fixture)
}

func ExpectResponseToMatchString(response *xealth_client.PreorderFormResponse, expected []byte) {
	actual, err := json.Marshal(response)
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())

	Expect(actual).To(MatchJSON(expected))
}
