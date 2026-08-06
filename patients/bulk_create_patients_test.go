package patients_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"

	clinicErrs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/go-common/clients/shoreline"
)

var _ = Describe("Bulk Account Creation", func() {

	var ctrl *gomock.Controller
	var patientSvc *patientsTest.MockService
	var userSvc *patientsTest.MockUserService

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		patientSvc = patientsTest.NewMockService(ctrl)
		userSvc = patientsTest.NewMockUserService(ctrl)
	})

	It("ParsePotentialCsvPatients", func() {
		ctx := context.Background()
		types := patients.ValidCsvPatientValues{
			ValidDiagnoses: []string{"gestational", "lada", "mody", "other", "prediabetes", "type1", "type2", "type3c"},
			ValidPresets:   []string{"adaHighRisk", "adaPregnancyType1", "adaPregnancyType2", "adaStandard"},
			DefaultPreset:  "adaStandard",
		}

		clinicId, err := primitive.ObjectIDFromHex("6a73255ba61db3fae2d878c2")
		Expect(err).ToNot(HaveOccurred())

		records := [][]string{
			{"Name", "Birthdate", "Mrn", "Email", "Diabetes Type", "Glycemic Target"},
			{"George Washington", "1950-01-02", "123456789", "duplicate@tidepool.org"},
			{"John Adams", "1951-01-02", "DUPLICATEMRN", "john+adams@tidepool.org"},
			{"Thomas Jefferson", "1952-01-02", "DUPLICATEMRN", "duplicate@tidepool.org"},
			{"James Madison", "1953-01-02", "123456790", "james+madison@tidepool.org"},
			{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk"},
			{"Missing Required Field Mrn", "1955-01-02", ""},
			{"Missing Required Field DOB", "", "123456"},
			{"Default Invalid Glycemic Target To Default", "1955-01-02", "123456791", "dev@tidepool.org", "", "Some Glycemic Target"},
			{"Diabetes Type", "1955-01-02", "123456791", "dev@tidepool.org", "type1"},
		}
		expectedOutput := [][]string{
			{"Name", "Birthdate", "Mrn", "Email", "Diabetes Type", "Glycemic Target", "Reason", "Emailed?"},
			{"George Washington", "1950-01-02", "123456789", "duplicate@tidepool.org", "", "adaStandard", "duplicate email", ""},
			{"John Adams", "1951-01-02", "DUPLICATEMRN", "john+adams@tidepool.org", "", "adaStandard", "duplicate MRN", ""},
			{"Thomas Jefferson", "1952-01-02", "DUPLICATEMRN", "duplicate@tidepool.org", "", "adaStandard", "duplicate MRN, duplicate email", ""},
			{"James Madison", "1953-01-02", "123456790", "james+madison@tidepool.org", "", "adaStandard", "", ""},
			{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk", "", ""},
			{"Missing Required Field Mrn", "1955-01-02", "", "", "", "", "missing mrn", ""},
			{"Missing Required Field DOB", "", "123456", "", "", "", "missing birthdate", ""},
			{"Default Invalid Glycemic Target To Default", "1955-01-02", "123456791", "dev@tidepool.org", "", "adaStandard", "", ""},
			{"Diabetes Type", "1955-01-02", "123456791", "dev@tidepool.org", "type1", "adaStandard", "", ""},
		}

		patientSvc.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(
				func(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
					if filter != nil && filter.Mrn != nil && *filter.Mrn == "DUPLICATEMRN" {
						return &patients.ListResult{MatchingCount: 1}, nil
					}
					return &patients.ListResult{MatchingCount: 0}, nil
				}).
			AnyTimes()
		userSvc.EXPECT().
			GetUser(gomock.Any()).
			DoAndReturn(
				func(userId string) (*shoreline.UserData, error) {
					if userId == "duplicate@tidepool.org" {
						return &shoreline.UserData{Username: "duplicate@tidepool.org"}, nil
					}
					return nil, clinicErrs.NotFound
				}).
			AnyTimes()

		outputRecords, header, _, err := patients.ParsePotentialCsvPatients(ctx, patientSvc, userSvc, records, clinicId, types)
		Expect(err).ToNot(HaveOccurred())
		Expect(header).To(Equal(expectedOutput[0]))
		Expect(outputRecords).To(Equal(expectedOutput))
	})
})
