package merge_test

import (
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tealeg/xlsx/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
)

var _ = Describe("merge reports", func() {
	Context("where the source clinic's first patient has a summary", func() {
		It("has a last upload date in the report", func() {
			at := time.Now().Add(-time.Hour)
			th := newReportTestHelper(GinkgoT(), at)
			Expect(th.File).ToNot(BeNil())
			Expect(xlsxHasLatestUpload(th.File, at)).To(BeTrue())
		})

		It("includes the sites' names", func() {
			th := newReportTestHelper(GinkgoT(), time.Now())
			siteNames := xlsxSiteNames(th.File)
			Expect(len(siteNames)).To(Equal(4))
			Expect(siteNames[0]).To(Equal("Chicago"))
			Expect(siteNames[1]).To(Equal("Chicago (2)"))
			Expect(siteNames[2]).To(Equal("New York"))
			Expect(siteNames[3]).To(Equal("San Diego"))
		})

		It("includes the sites' actions", func() {
			th := newReportTestHelper(GinkgoT(), time.Now())
			th.WriteToFilename("/tmp/report.xlsx")
			Expect(xlsxSiteAction(th.File, "Chicago")).To(Equal("Retain"))
			Expect(xlsxSiteAction(th.File, "Chicago (2)")).To(Equal("Rename"))
			Expect(xlsxSiteAction(th.File, "New York")).To(Equal("Create"))
			Expect(xlsxSiteAction(th.File, "San Diego")).To(Equal("Retain"))
		})

		Context("the source patients tab", func() {
			It("includes the patient's sites", func() {
				th := newReportTestHelper(GinkgoT(), time.Now())
				Expect(xlsxSourcePatientSites(th.File, 0)).To(Equal("Some Source Site"))
			})
		})

		Context("the target patients tab", func() {
			It("includes the patient's sites", func() {
				th := newReportTestHelper(GinkgoT(), time.Now())
				Expect(xlsxTargetPatientSites(th.File, 0)).To(Equal("Some Target Site"))
			})
		})
	})
})

func xlsxHasLatestUpload(f *xlsx.File, date time.Time) bool {
	m, err := f.ToSlice()
	Expect(err).To(Succeed())
	return m[sourcePatientsSheetIdx][firstPatientRowIdx][latestUploadColIdx] == date.Format(merge.LastUploadTimeFormat)
}

func xlsxSiteAction(f *xlsx.File, siteName string) string {
	m, err := f.ToSlice()
	Expect(err).To(Succeed())
	rowIdx := 0
	for rowIdx = range 100 {
		if strings.HasPrefix(m[summarySheetIdx][rowIdx][siteNameColIdx], "Resulting Sites") {
			break
		}
	}
	Expect(rowIdx < 100).To(Equal(true))
	for rowOffset := range 100 {
		if sn := m[summarySheetIdx][rowIdx+rowOffset+1][siteNameColIdx]; sn == siteName {
			return m[summarySheetIdx][rowIdx+rowOffset+1][siteActionColIdx]
		}
	}
	return ""
}

func xlsxSiteNames(f *xlsx.File) []string {
	m, err := f.ToSlice()
	Expect(err).To(Succeed())
	rowIdx := 0
	for rowIdx = range 100 {
		if strings.HasPrefix(m[summarySheetIdx][rowIdx][siteNameColIdx], "Resulting Sites (") {
			break
		}
	}
	Expect(rowIdx < 100).To(Equal(true))
	siteNames := []string{}
	for rowOffset := range 100 {
		if siteName := m[summarySheetIdx][rowIdx+rowOffset+1][siteNameColIdx]; siteName != "" {
			siteNames = append(siteNames, siteName)
		} else {
			break
		}
	}
	return siteNames
}

func xlsxSourcePatientSites(f *xlsx.File, patientIdx int) string {
	m, err := f.ToSlice()
	Expect(err).To(Succeed())
	return m[sourcePatientsSheetIdx][firstPatientRowIdx+patientIdx][patientSitesColIdx]
}

func xlsxTargetPatientSites(f *xlsx.File, patientIdx int) string {
	m, err := f.ToSlice()
	Expect(err).To(Succeed())
	return m[targetPatientsSheetIdx][firstPatientRowIdx+patientIdx][patientSitesColIdx]
}

const (
	// sourcePatientsSheetIdx is the 0-based index of the sheet in the xlsx.
	sourcePatientsSheetIdx = 1
	// targetPatientsSheetIdx is the 0-based index of the sheet in the xlsx.
	targetPatientsSheetIdx = 2
	// firstPatientRowIdx is the 0-based index of the first patient data row in the xlsx.
	firstPatientRowIdx = 2
	// latestUploadColIdx is the 0-based index of the Latest Upload column in the xlsx.
	latestUploadColIdx = 7
	// siteNameColIdx is the 0-based index of Site names in the xlsx.
	siteNameColIdx = 0
	// siteActionColIdx is the 0-based index of Site action in the xlsx.
	siteActionColIdx = 2
	// summarySheetIdx is the 0-based index of the merge summary sheet in the xlsx.
	summarySheetIdx = 0
	// patientSitesColIdx is the 0-based index of a patient's sites in the xlsx.
	patientSitesColIdx = 6
)

type reportTestHelper struct {
	Plan   merge.ClinicMergePlan
	Report merge.Report
	File   *xlsx.File

	t FullGinkgoTInterface
}

func newReportTestHelper(t FullGinkgoTInterface, at time.Time) *reportTestHelper {
	plan := newTestPlan(at)
	report := merge.NewReport(plan)
	xlsxFile, err := report.Generate()
	if err != nil {
		t.Fatalf("failed to generate report: %s", err)
	}
	if xlsxFile == nil {
		t.Fatalf("failed to generate report: report is nil")
	}
	return &reportTestHelper{
		Plan:   plan,
		Report: report,
		File:   xlsxFile,
		t:      t,
	}
}

func (r *reportTestHelper) WriteToFilename(filename string) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		r.t.Fatalf("failed to write report to file %q: %s", filename, err)
	}
	defer f.Close()
	if err := r.File.Write(f); err != nil {
		r.t.Fatalf("failed to write report: %s", err)
	}
}

func newTestPlan(at time.Time) merge.ClinicMergePlan {
	return merge.ClinicMergePlan{
		Source:                          clinics.Clinic{},
		Target:                          clinics.Clinic{},
		MembershipRestrictionsMergePlan: merge.MembershipRestrictionsMergePlan{},
		SourcePatientClusters:           merge.PatientClusters{},
		TargetPatientClusters:           merge.PatientClusters{},
		SettingsPlans:                   merge.SettingsPlans{},
		TagsPlans:                       merge.TagPlans{},
		SitesPlans: merge.SitesPlans{
			{
				Site: sites.Site{
					Id:   primitive.NewObjectID(),
					Name: "Chicago",
				},
				Action:          merge.SiteActionRetain,
				SourceWorkspace: "Test Target Clinic",
				SourceClinicId:  &primitive.ObjectID{},
				TargetClinicId:  &primitive.ObjectID{},
			},
			{
				Site: sites.Site{
					Id:   primitive.NewObjectID(),
					Name: "Chicago (2)",
				},
				Action:          merge.SiteActionRename,
				SourceWorkspace: "Test Source Clinic",
				SourceClinicId:  &primitive.ObjectID{},
				TargetClinicId:  &primitive.ObjectID{},
			},
			{
				Site: sites.Site{
					Id:   primitive.NewObjectID(),
					Name: "New York",
				},
				Action:          merge.SiteActionMove,
				SourceWorkspace: "Test Source Clinic",
				SourceClinicId:  &primitive.ObjectID{},
				TargetClinicId:  &primitive.ObjectID{},
			},
			{
				Site: sites.Site{
					Id:   primitive.NewObjectID(),
					Name: "San Diego",
				},
				Action:          merge.SiteActionRetain,
				SourceWorkspace: "Test Target Clinic",
				SourceClinicId:  &primitive.ObjectID{},
				TargetClinicId:  &primitive.ObjectID{},
			},
		},
		ClinicianPlans: merge.ClinicianPlans{},
		PatientPlans: merge.PatientPlans{
			{
				SourceClinicId:   &primitive.ObjectID{},
				TargetClinicId:   &primitive.ObjectID{},
				SourceClinicName: "Test Source Clinic",
				TargetClinicName: "Test Target Clinic",
				SourcePatient: &patients.Patient{
					Id:            &primitive.ObjectID{},
					ClinicId:      &primitive.ObjectID{},
					UserId:        new(string),
					BirthDate:     new(string),
					Email:         new(string),
					FullName:      strp("John \"Source\" Doe"),
					Mrn:           new(string),
					TargetDevices: &[]string{},
					Tags:          &[]primitive.ObjectID{},
					Sites: &[]sites.Site{
						{
							Id:   primitive.NewObjectID(),
							Name: "Some Source Site",
						},
					},
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
							Periods: patients.PatientCGMPeriods{},
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
			{
				SourceClinicId:   &primitive.ObjectID{},
				TargetClinicId:   &primitive.ObjectID{},
				SourceClinicName: "Test Source Clinic",
				TargetClinicName: "Test Target Clinic",
				SourcePatient:    nil, // this patient is a "retain"
				TargetPatient: &patients.Patient{
					Id:            &primitive.ObjectID{},
					ClinicId:      &primitive.ObjectID{},
					UserId:        new(string),
					BirthDate:     new(string),
					Email:         new(string),
					FullName:      strp("John \"Target\" Doe"),
					Mrn:           new(string),
					TargetDevices: &[]string{},
					Tags:          &[]primitive.ObjectID{},
					Sites: &[]sites.Site{
						{
							Id:   primitive.NewObjectID(),
							Name: "Some Target Site",
						},
					},
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
							Periods: patients.PatientCGMPeriods{},
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

func strp(s string) *string {
	return &s
}
