package xealth_test

import (
	"context"
	"encoding/json"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/clinic/xealth"
	"github.com/tidepool-org/clinic/xealth_client"
	"go.uber.org/zap"
)

const DeploymentInFixtures = "artificialhealthcare"

var _ = Describe("Matcher", func() {
	var patientsCtrl *gomock.Controller
	var clinicsCtrl *gomock.Controller

	var patientsService *patientsTest.MockService
	var clinicsService *clinicsTest.MockService

	BeforeEach(func() {
		tb := GinkgoT()
		patientsCtrl = gomock.NewController(tb)
		patientsService = patientsTest.NewMockService(patientsCtrl)
		clinicsCtrl = gomock.NewController(tb)
		clinicsService = clinicsTest.NewMockService(clinicsCtrl)
	})

	AfterEach(func() {
		patientsCtrl.Finish()
		clinicsCtrl.Finish()
	})

	Describe("Matching Criteria", func() {
		var clinic *clinics.Clinic
		var expectedCriteria xealth.PatientMatchingCriteria

		BeforeEach(func() {
			clinic = clinicsTest.EnableXealth(clinicsTest.RandomClinic(), DeploymentInFixtures)
			clinicsService.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]*clinics.Clinic{clinic}, nil)
			patientsService.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&patients.ListResult{}, nil)

			expectedCriteria = xealth.PatientMatchingCriteria{
				FirstName:   "FirstName First",
				LastName:    "LastName Last",
				FullName:    "FirstName First LastName Last",
				Mrn:         "10052608740",
				DateOfBirth: "1985-01-11",
				Email:       "home@example.com",
			}
		})

		When("matching initial preorder form request", func() {
			var initialPreorderFormRequest xealth_client.PreorderFormRequest0

			BeforeEach(func() {
				body, err := test.LoadFixture("test/fixtures/preorder_initial_request.json")
				Expect(err).ToNot(HaveOccurred())

				Expect(json.Unmarshal(body, &initialPreorderFormRequest)).To(Succeed())
			})

			It("is correct", func() {
				matcher := xealth.NewMatcher[*xealth_client.PreorderFormResponse](clinicsService, patientsService, zap.NewNop().Sugar()).
					FromInitialPreorderForRequest(initialPreorderFormRequest).
					DisableErrorOnNoMatchingPatients()

				res, err := matcher.Match(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Response).To(BeNil())

				Expect(res.Criteria).ToNot(BeNil())
				Expect(res.Criteria).To(PointTo(Equal(expectedCriteria)))
			})
		})

		When("matching subsequent preorder form request", func() {
			var subsequentPreorderFormRequest xealth_client.PreorderFormRequest1

			BeforeEach(func() {
				body, err := test.LoadFixture("test/fixtures/preorder_subsequent_request.json")
				Expect(err).ToNot(HaveOccurred())

				Expect(json.Unmarshal(body, &subsequentPreorderFormRequest)).To(Succeed())
			})

			It("is correct", func() {
				matcher := xealth.NewMatcher[*xealth_client.PreorderFormResponse](clinicsService, patientsService, zap.NewNop().Sugar()).
					FromSubsequentPreorderForRequest(subsequentPreorderFormRequest).
					DisableErrorOnNoMatchingPatients()

				res, err := matcher.Match(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Response).To(BeNil())

				Expect(res.Criteria).ToNot(BeNil())
				Expect(res.Criteria).To(PointTo(Equal(expectedCriteria)))
			})
		})

		When("matching order event", func() {
			var orderEvent xealth.OrderEvent

			BeforeEach(func() {
				orderBody, err := test.LoadFixture("test/fixtures/order.json")
				Expect(err).ToNot(HaveOccurred())

				Expect(json.Unmarshal(orderBody, &orderEvent.OrderData)).To(Succeed())
			})

			It("is correct", func() {
				matcher := xealth.NewMatcher[*xealth_client.PreorderFormResponse](clinicsService, patientsService, zap.NewNop().Sugar()).
					FromOrder(orderEvent).
					DisableErrorOnNoMatchingPatients()

				res, err := matcher.Match(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Response).To(BeNil())

				Expect(res.Criteria).ToNot(BeNil())
				Expect(res.Criteria).To(PointTo(Equal(expectedCriteria)))
			})
		})

		When("matching programs request", func() {
			var request xealth_client.GetProgramsRequest

			BeforeEach(func() {
				body, err := test.LoadFixture("test/fixtures/get_programs_request.json")
				Expect(err).ToNot(HaveOccurred())

				Expect(json.Unmarshal(body, &request)).To(Succeed())
			})

			It("is correct", func() {
				matcher := xealth.NewMatcher[*xealth_client.PreorderFormResponse](clinicsService, patientsService, zap.NewNop().Sugar()).
					FromProgramsRequest(request).
					DisableErrorOnNoMatchingPatients()

				res, err := matcher.Match(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Response).To(BeNil())

				Expect(res.Criteria).ToNot(BeNil())
				Expect(res.Criteria).To(PointTo(Equal(expectedCriteria)))
			})
		})

		When("matching program urls request", func() {
			var request xealth_client.GetProgramUrlRequest

			BeforeEach(func() {
				body, err := test.LoadFixture("test/fixtures/get_program_url_request.json")
				Expect(err).ToNot(HaveOccurred())

				Expect(json.Unmarshal(body, &request)).To(Succeed())
			})

			It("is correct", func() {
				matcher := xealth.NewMatcher[*xealth_client.PreorderFormResponse](clinicsService, patientsService, zap.NewNop().Sugar()).
					FromProgramUrlRequest(request).
					DisableErrorOnNoMatchingPatients()

				res, err := matcher.Match(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Response).To(BeNil())

				Expect(res.Criteria).ToNot(BeNil())
				Expect(res.Criteria).To(PointTo(Equal(expectedCriteria)))
			})
		})

	})

})
