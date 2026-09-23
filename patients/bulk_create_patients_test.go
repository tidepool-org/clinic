package patients_test

import (
	"context"
	"errors"
	"strings"

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

	var err error
	var clinicId primitive.ObjectID

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		DeferCleanup(ctrl.Finish)
		patientSvc = patientsTest.NewMockService(ctrl)
		userSvc = patientsTest.NewMockUserService(ctrl)
		clinicId, err = primitive.ObjectIDFromHex("6a73255ba61db3fae2d878c2")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("ParsePotentialCSVPatients", func() {
		It("returns correct fields and errors depending on criteria", func() {
			ctx := context.Background()
			records := [][]string{
				{"George Washington", "1950-01-02", "123456789", "existing+email@tidepool.org"},
				{"John Adams", "1951-01-02", "DUPLICATEMRN", "john+adams@tidepool.org"},
				{"Thomas Jefferson", "1952-01-02", "DUPLICATEMRN", "existing+email@tidepool.org"},
				{"Existing Email Case Insensitive Check", "1959-01-02", "444455555", "EXISTING+email@tidepool.org"},
				{"James Madison", "1953-01-02", "123456790", "james+madison@tidepool.org"},
				{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk"},
				{"Default Invalid Glycemic Target To Default", "1955-01-02", "123456793", "dev@tidepool.org", "", "Some Glycemic Target"},
				{"Diabetes Type", "1955-01-02", "123456794", "diabetes+type@tidepool.org", "type1"},
				{"Test extra input columns are not included in output", "1960-02-03", "88888888", "extra+columns@tidepool.org", "type1", "adaStandard", "Extra column 1", "Extra column 2", "Extra column 3"},
				{"Person 1 duplicate MRN within CSV", "1999-12-10", "55555555", "duplicate+mrn1@tidepool.org", "type1", "adaStandard"},
				{"Person 2 duplicate MRN within CSV", "2000-10-10", "55555555", "duplicate+mrn2@tidepool.org", "type1", "adaStandard"},
				{"Person 1 duplicate email within CSV", "1999-12-10", "55555556", "duplicate+email@tidepool.org", "type1", "adaStandard"},
				{"Person 2 duplicate email within CSV", "2000-10-10", "55555557", "duplicate+email@tidepool.org", "type1", "adaStandard"},
				{"Person 3 duplicate email within CSV ignoring case", "2000-11-12", "55555558", "DUPLICATE+EMAIL@tidepool.org", "type1", "adaStandard"},
			}
			expectedOutput := [][]string{
				{"Name", "Birthdate", "MRN", "Email", "Diabetes Type", "Glycemic Target", "Reason", "Emailed?"},
				{"George Washington", "1950-01-02", "123456789", "existing+email@tidepool.org", "", "adaStandard", "duplicate email", ""},
				{"John Adams", "1951-01-02", "DUPLICATEMRN", "john+adams@tidepool.org", "", "adaStandard", "duplicate MRN", ""},
				{"Thomas Jefferson", "1952-01-02", "DUPLICATEMRN", "existing+email@tidepool.org", "", "adaStandard", "duplicate MRN, duplicate email", ""},
				{"Existing Email Case Insensitive Check", "1959-01-02", "444455555", "EXISTING+email@tidepool.org", "", "adaStandard", "duplicate email", ""},
				{"James Madison", "1953-01-02", "123456790", "james+madison@tidepool.org", "", "adaStandard", "", ""},
				{"James Monroe", "1954-01-02", "123456791", "james+monroe@tidepool.org", "", "adaHighRisk", "", ""},
				{"Default Invalid Glycemic Target To Default", "1955-01-02", "123456793", "dev@tidepool.org", "", "adaStandard", "", ""},
				{"Diabetes Type", "1955-01-02", "123456794", "diabetes+type@tidepool.org", "type1", "adaStandard", "", ""},
				{"Test extra input columns are not included in output", "1960-02-03", "88888888", "extra+columns@tidepool.org", "type1", "adaStandard", "", ""},
				{"Person 1 duplicate MRN within CSV", "1999-12-10", "55555555", "duplicate+mrn1@tidepool.org", "type1", "adaStandard", "duplicate MRN", ""},
				{"Person 2 duplicate MRN within CSV", "2000-10-10", "55555555", "duplicate+mrn2@tidepool.org", "type1", "adaStandard", "duplicate MRN", ""},
				{"Person 1 duplicate email within CSV", "1999-12-10", "55555556", "duplicate+email@tidepool.org", "type1", "adaStandard", "duplicate email", ""},
				{"Person 2 duplicate email within CSV", "2000-10-10", "55555557", "duplicate+email@tidepool.org", "type1", "adaStandard", "duplicate email", ""},
				{"Person 3 duplicate email within CSV ignoring case", "2000-11-12", "55555558", "DUPLICATE+EMAIL@tidepool.org", "type1", "adaStandard", "duplicate email", ""},
			}

			patientSvc.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
						return &patients.ListResult{
							Patients: []*patients.Patient{
								{
									Mrn: strp("DUPLICATEMRN"),
								},
							},
							MatchingCount: 1,
						}, nil
					}).
				AnyTimes()
			userSvc.EXPECT().
				GetUser(gomock.Any()).
				DoAndReturn(
					func(userId string) (*shoreline.UserData, error) {
						if userId == "existing+email@tidepool.org" {
							return &shoreline.UserData{Username: "existing+email@tidepool.org"}, nil
						}
						return nil, clinicErrs.NotFound
					}).
				AnyTimes()

			outputRecords, header, parsedPatients, err := patients.ParsePotentialCSVPatients(ctx, patientSvc, userSvc, records, clinicId, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(parsedPatients)).To(Equal(len(expectedOutput) - 1))
			Expect(header).To(Equal(expectedOutput[0]))
			Expect(outputRecords).To(Equal(expectedOutput))
		})
	})

	Describe("CreateCSVPatients", func() {
		It("creates an account for each valid patient and returns an updated CSV of the emailed status", func() {
			ctx := context.Background()
			invitedBy := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
			records := [][]string{
				{"George Washington", "1950-01-02", "123456789", "existing+email@tidepool.org"},
				{"James Monroe", "1954-01-02", "123456792", "james+monroe@tidepool.org", "", "adaHighRisk"},
			}
			expectedOutput := [][]string{
				{"Name", "Birthdate", "MRN", "Email", "Diabetes Type", "Glycemic Target", "Reason", "Emailed?"},
				{"George Washington", "1950-01-02", "123456789", "existing+email@tidepool.org", "", "adaStandard", "duplicate email", "N"},
				{"James Monroe", "1954-01-02", "123456792", "james+monroe@tidepool.org", "", "adaHighRisk", "", "Y"},
				{"Patients Processed", "2"},
				{"Patients Created", "1"},
				{"Patients Skipped", "1"},
				{"Patients Emailed", "1"},
				{"Duplicate MRNs count", "0"},
				{"Duplicate emails count", "1"},
			}
			patientSvc.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
						return &patients.ListResult{
							Patients: []*patients.Patient{
								{
									Mrn: strp("DUPLICATEMRN"),
								},
							},
							MatchingCount: 1,
						}, nil
					}).
				AnyTimes()
			userSvc.EXPECT().
				GetUser(gomock.Any()).
				DoAndReturn(
					func(userId string) (*shoreline.UserData, error) {
						if userId == "existing+email@tidepool.org" {
							return &shoreline.UserData{Username: "existing+email@tidepool.org"}, nil
						}
						return nil, clinicErrs.NotFound
					}).
				AnyTimes()
			patientSvc.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				Return(&patients.Patient{}, nil).
				Times(1)

			_, header, parsedPatients, err := patients.ParsePotentialCSVPatients(ctx, patientSvc, userSvc, records, clinicId, &invitedBy)
			Expect(len(parsedPatients)).To(Equal(2))
			Expect(err).ToNot(HaveOccurred())
			outputRecords := patients.CreateCSVPatients(ctx, patientSvc, header, parsedPatients)
			Expect(outputRecords).To(Equal(expectedOutput))
		})
	})

	Describe("ParsePotentialCSVPatientsReader", func() {
		Describe("validation", func() {
			BeforeEach(func() {
				patientSvc.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(
						func(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
							return &patients.ListResult{
								Patients: []*patients.Patient{
									{
										Mrn: strp("DUPLICATEMRN"),
									},
								},
								MatchingCount: 1,
							}, nil
						}).
					AnyTimes()
				userSvc.EXPECT().
					GetUser(gomock.Any()).
					DoAndReturn(
						func(userId string) (*shoreline.UserData, error) {
							if userId == "existing+email@tidepool.org" {
								return &shoreline.UserData{Username: "existing+email@tidepool.org"}, nil
							}
							return nil, clinicErrs.NotFound
						}).
					AnyTimes()
				patientSvc.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(&patients.Patient{}, nil).
					AnyTimes()
			})

			DescribeTable("valid CSV", func(contentsCSV string, expectedOutput [][]string) {
				ctx := context.Background()
				outputRecords, header, parsedPatients, err := patients.ParsePotentialCSVPatientsReader(ctx, strings.NewReader(contentsCSV), patientSvc, userSvc, clinicId, nil)
				Expect(err).ToNot(HaveOccurred())
				outputRecords = patients.CreateCSVPatients(ctx, patientSvc, header, parsedPatients)
				Expect(outputRecords).To(Equal(expectedOutput))
			},
				Entry(
					"valid CSV no non-blocking patient errors",
					`George Washington,1950-01-02,123456789,george+washington@tidepool.org
John Adams,1951-01-02,123456780,john+adams@tidepool.org`,
					[][]string{
						{"Name", "Birthdate", "MRN", "Email", "Diabetes Type", "Glycemic Target", "Reason", "Emailed?"},
						{"George Washington", "1950-01-02", "123456789", "george+washington@tidepool.org", "", "adaStandard", "", "Y"},
						{"John Adams", "1951-01-02", "123456780", "john+adams@tidepool.org", "", "adaStandard", "", "Y"},
						{"Patients Processed", "2"},
						{"Patients Created", "2"},
						{"Patients Skipped", "0"},
						{"Patients Emailed", "2"},
						{"Duplicate MRNs count", "0"},
						{"Duplicate emails count", "0"},
					}),
				Entry(
					"valid CSV with mix of valid patients and duplicate mrn / emails",
					`Thomas Jefferson,1952-01-09,112233,thomas+jefferson@tidepool.org
Ben Franklin,1952-01-10,112234
George Washington,1950-01-02,123456789,existing+email@tidepool.org
John Adams,1951-01-02,DUPLICATEMRN,john+adams@tidepool.org
Invalid Glycemic Target Defaults To adaStandard,1973-03-04,22334455,james+madison@tidepool.org,,invalid type`,
					[][]string{
						{"Name", "Birthdate", "MRN", "Email", "Diabetes Type", "Glycemic Target", "Reason", "Emailed?"},
						{"Thomas Jefferson", "1952-01-09", "112233", "thomas+jefferson@tidepool.org", "", "adaStandard", "", "Y"},
						{"Ben Franklin", "1952-01-10", "112234", "", "", "adaStandard", "", "N"},
						{"George Washington", "1950-01-02", "123456789", "existing+email@tidepool.org", "", "adaStandard", "duplicate email", "N"},
						{"John Adams", "1951-01-02", "DUPLICATEMRN", "john+adams@tidepool.org", "", "adaStandard", "duplicate MRN", "N"},
						{"Invalid Glycemic Target Defaults To adaStandard", "1973-03-04", "22334455", "james+madison@tidepool.org", "", "adaStandard", "", "Y"},
						{"Patients Processed", "5"},
						{"Patients Created", "3"},
						{"Patients Skipped", "2"},
						{"Patients Emailed", "2"},
						{"Duplicate MRNs count", "1"},
						{"Duplicate emails count", "1"},
					}),
			)

			It("invalid CSV", func() {
				contentsCSV := `Missing MRN,1952-01-02,,,
	,,,,
	Missing birthdate and MRN,,,
	,1999-01-02,,,
	Invalid birthdate,yyyy-mm-dd,1111,,
	Invalid email,1999-01-12,2222,invalid+email,`
				ctx := context.Background()
				expectedErrs := []error{
					patients.ErrCSVPatientMissingName,
					patients.ErrCSVPatientMissingMRN,
					patients.ErrCSVPatientInvalidEmail,
					patients.ErrCSVPatientInvalidBirthdate,
					patients.ErrCSVPatientMissingBirthdate,
				}
				expectedErrStrs := []string{
					"missing MRN: row 1, column 3",
					"missing name: row 2, column 1",
					"missing birthdate: row 2, column 2",
					"missing MRN: row 2, column 3",
					"missing birthdate: row 3, column 2",
					"missing MRN: row 3, column 3",
					"missing name: row 4, column 1",
					"missing MRN: row 4, column 3",
					"invalid birthdate: row 5, column 2",
					"invalid email: row 6, column 4",
				}
				_, _, _, err := patients.ParsePotentialCSVPatientsReader(ctx, strings.NewReader(contentsCSV), patientSvc, userSvc, clinicId, nil)
				Expect(patients.IsBulkPatientCSVValidationErr(err)).To(Equal(true))
				for _, expectedErr := range expectedErrs {
					Expect(err).To(MatchError(expectedErr))
				}
				for _, errSubstr := range expectedErrStrs {
					Expect(err).To(MatchError(ContainSubstring(errSubstr)))
				}
			})
		})

		Describe("server error", func() {
			var contentsCSV string
			BeforeEach(func() {
				contentsCSV = `George Washington,1950-01-02,123456789,george+washington@tidepool.org
John Adams,1951-01-02,123456780,john+adams@tidepool.org`
			})
			It("patient svc List error", func() {
				patientSvc.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(
						func(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
							return &patients.ListResult{
								Patients: []*patients.Patient{
									{
										Mrn: strp("DUPLICATEMRN"),
									},
								},
								MatchingCount: 1,
							}, errors.New("error retrieving existing patients")
						}).
					Times(1)
				ctx := context.Background()
				_, _, _, err := patients.ParsePotentialCSVPatientsReader(ctx, strings.NewReader(contentsCSV), patientSvc, userSvc, clinicId, nil)
				Expect(err).To(HaveOccurred())
				Expect(patients.IsBulkPatientCSVValidationErr(err)).To(Equal(false))
			})

			It("user svc GetUser error", func() {
				patientSvc.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(
						func(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
							return &patients.ListResult{
								Patients: []*patients.Patient{
									{
										Mrn: strp("DUPLICATEMRN"),
									},
								},
								MatchingCount: 1,
							}, nil
						}).
					Times(1)
				userSvc.EXPECT().
					GetUser(gomock.Any()).
					DoAndReturn(
						func(userId string) (*shoreline.UserData, error) {
							return nil, errors.New("error retrieving user")
						}).
					AnyTimes()
				ctx := context.Background()
				_, _, _, err := patients.ParsePotentialCSVPatientsReader(ctx, strings.NewReader(contentsCSV), patientSvc, userSvc, clinicId, nil)
				Expect(err).To(HaveOccurred())
				Expect(patients.IsBulkPatientCSVValidationErr(err)).To(Equal(false))
			})
		})
	})
})

func strp(s string) *string {
	return &s
}
