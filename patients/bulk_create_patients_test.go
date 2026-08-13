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

	var types patients.ValidCSVPatientValues
	var err error
	var clinicId primitive.ObjectID

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		patientSvc = patientsTest.NewMockService(ctrl)
		userSvc = patientsTest.NewMockUserService(ctrl)
		types = patients.ValidCSVPatientValues{
			ValidDiagnoses: []string{"gestational", "lada", "mody", "other", "prediabetes", "type1", "type2", "type3c"},
			ValidPresets:   []string{"adaHighRisk", "adaPregnancyType1", "adaPregnancyType2", "adaStandard"},
			DefaultPreset:  "adaStandard",
		}
		clinicId, err = primitive.ObjectIDFromHex("6a73255ba61db3fae2d878c2")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("ParsePotentialCSVPatients", func() {
		It("returns a fatal blocking error if input CSV does not have the necessary number of columns", func() {
			ctx := context.Background()
			records := [][]string{
				{"Name", "Birthdate"},
				{"George Washington", "1950-01-02"},
			}
			_, _, _, err := patients.ParsePotentialCSVPatients(ctx, patientSvc, userSvc, records, clinicId, types)
			Expect(err).To(MatchError(patients.ErrCSVNotEnoughColumns))
		})

		It("returns correct fields and errors depending on criteria", func() {
			ctx := context.Background()
			records := [][]string{
				{"Name", "Birthdate", "Mrn", "Email", "Diabetes Type", "Glycemic Target"},
				{"George Washington", "1950-01-02", "123456789", "duplicate@tidepool.org"},
				{"John Adams", "1951-01-02", "DUPLICATEMRN", "john+adams@tidepool.org"},
				{"Thomas Jefferson", "1952-01-02", "DUPLICATEMRN", "duplicate@tidepool.org"},
				{"James Madison", "1953-01-02", "123456790", "james+madison@tidepool.org"},
				{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk"},
				{"Invalid Email", "1960-01-02", "123456791", "invalid+email", "type2", ""},
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
				{"Invalid Email", "1960-01-02", "123456791", "invalid+email", "type2", "", "invalid email", ""},
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

			outputRecords, header, parsedPatients, err := patients.ParsePotentialCSVPatients(ctx, patientSvc, userSvc, records, clinicId, types)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(parsedPatients)).To(Equal(10))
			Expect(header).To(Equal(expectedOutput[0]))
			Expect(outputRecords).To(Equal(expectedOutput))
		})
	})

	Describe("CreateCSVPatients", func() {
		It("creates an account for each valid patient and returns an updated CSV of the emailed status", func() {
			ctx := context.Background()
			records := [][]string{
				{"Name", "Birthdate", "Mrn", "Email", "Diabetes Type", "Glycemic Target"},
				{"George Washington", "1950-01-02", "123456789", "duplicate@tidepool.org"},
				{"Invalid Email", "1960-01-02", "123456791", "invalid+email", "type2", ""},
				{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk"},
			}
			expectedOutput := [][]string{
				{"Name", "Birthdate", "Mrn", "Email", "Diabetes Type", "Glycemic Target", "Reason", "Emailed?"},
				{"George Washington", "1950-01-02", "123456789", "duplicate@tidepool.org", "", "adaStandard", "duplicate email", "N"},
				{"Invalid Email", "1960-01-02", "123456791", "invalid+email", "type2", "", "invalid email", "N (invalid email)"},
				{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk", "", "Y"},
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
			patientSvc.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				Return(&patients.Patient{}, nil).
				Times(1)

			_, header, parsedPatients, err := patients.ParsePotentialCSVPatients(ctx, patientSvc, userSvc, records, clinicId, types)
			Expect(len(parsedPatients)).To(Equal(3))
			Expect(err).ToNot(HaveOccurred())
			outputRecords := patients.CreateCSVPatients(ctx, patientSvc, header, parsedPatients)
			Expect(outputRecords).To(Equal(expectedOutput))
		})
	})
})
