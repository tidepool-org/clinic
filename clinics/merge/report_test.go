package merge_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tealeg/xlsx/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	"github.com/tidepool-org/clinic/patients"
)

var _ = Describe("merge reports", func() {
	Context("where the source clinic's first patient has a summary", func() {
		It("has a last upload date in the report", func() {
			at := time.Now().Add(-time.Hour)
			plan := newTestPlan(at)
			report := merge.NewReport(plan)
			xlsxFile, err := report.Generate()
			Expect(err).To(Succeed())
			Expect(xlsxFile).ToNot(BeNil())
			Expect(xlsxHasLatestUpload(at, xlsxFile, sourcePatientsSheetIdx, firstPatientRowIdx)).To(BeTrue())
		})
	})
})

func xlsxHasLatestUpload(date time.Time, f *xlsx.File, sheetIdx, rowIdx int) bool {
	m, err := f.ToSlice()
	Expect(err).To(Succeed())
	return m[sheetIdx][rowIdx][latestUploadColIdx] == date.Format(merge.LastUploadTimeFormat)
}

// sourcePatientsSheetIdx is the 0-based index of the sheet in the xlsx.
const sourcePatientsSheetIdx = 1

// firstPatientRowIdx is the 0-based index of the first patient data row in the xlsx.
const firstPatientRowIdx = 2

// latestUploadColIdx is the 0-based index of the Latest Upload column in the xlsx.
const latestUploadColIdx = 6

func newTestPlan(at time.Time) merge.ClinicMergePlan {
	return merge.ClinicMergePlan{
		Source:                          clinics.Clinic{},
		Target:                          clinics.Clinic{},
		MembershipRestrictionsMergePlan: merge.MembershipRestrictionsMergePlan{},
		SourcePatientClusters:           merge.PatientClusters{},
		TargetPatientClusters:           merge.PatientClusters{},
		SettingsPlans:                   merge.SettingsPlans{},
		TagsPlans:                       merge.TagPlans{},
		ClinicianPlans:                  merge.ClinicianPlans{},
		PatientPlans: merge.PatientPlans{
			{
				SourceClinicId:   &primitive.ObjectID{},
				TargetClinicId:   &primitive.ObjectID{},
				SourceClinicName: "",
				TargetClinicName: "",
				SourcePatient: &patients.Patient{
					Id:                 &primitive.ObjectID{},
					ClinicId:           &primitive.ObjectID{},
					UserId:             new(string),
					BirthDate:          new(string),
					Email:              new(string),
					FullName:           new(string),
					Mrn:                new(string),
					TargetDevices:      &[]string{},
					Tags:               &[]primitive.ObjectID{},
					DataSources:        &[]patients.DataSource{},
					Permissions:        &patients.Permissions{},
					IsMigrated:         false,
					LegacyClinicianIds: []string{},
					CreatedTime:        time.Time{},
					UpdatedTime:        time.Time{},
					InvitedBy:          new(string),
					Summary: &patients.Summary{
						CGM: &patients.PatientCGMStats{
							Config: patients.PatientSummaryConfig{},
							Dates: patients.PatientSummaryDates{
								FirstData:         &time.Time{},
								HasFirstData:      false,
								HasLastData:       false,
								HasLastUploadDate: false,
								HasOutdatedSince:  false,
								LastData:          &time.Time{},
								LastUpdatedDate:   &time.Time{},
								LastUpdatedReason: &[]string{},
								// !!!
								// !!! Here we set the date via "at"
								// !!!
								LastUploadDate:     &at,
								OutdatedReason:     &[]string{},
								OutdatedSince:      &time.Time{},
								OutdatedSinceLimit: &time.Time{},
							},
							OffsetPeriods: patients.PatientCGMPeriods{},
							Periods:       patients.PatientCGMPeriods{},
							TotalHours:    0,
						},
						BGM: &patients.PatientBGMStats{},
					},
					Reviews:                        []patients.Review{},
					LastUploadReminderTime:         time.Time{},
					ProviderConnectionRequests:     patients.ProviderConnectionRequests{},
					RequireUniqueMrn:               false,
					EHRSubscriptions:               patients.EHRSubscriptions{},
					LastRequestedDexcomConnectTime: at,
				},
				TargetPatient:              &patients.Patient{},
				Conflicts:                  map[string][]merge.Conflict{},
				PatientAction:              "",
				SourceTagNames:             []string{},
				TargetTagNames:             []string{},
				PostMigrationTagNames:      []string{},
				PostMigrationMRNUniqueness: false,
				CanExecuteAction:           false,
				Error:                      &merge.ReportError{},
			},
		},
		CreatedTime: time.Time{},
	}
}
