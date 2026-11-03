package merge

import (
	"context"
	"time"

	"github.com/tealeg/xlsx/v3"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/pointer"
	"github.com/tidepool-org/clinic/sites"
)

const (
	ReportSheetPatients   = "Patients"
	ReportSheetDuplicates = "Duplicates"
)

// DuplicatesReportPlan lists a single workspace's patients and their likely duplicates. It is read-only.
type DuplicatesReportPlan struct {
	Clinic          clinics.Clinic
	Patients        []patients.Patient
	PatientClusters PatientClusters
	CreatedTime     time.Time
}

func PlanDuplicatesReport(ctx context.Context, clinicsService clinics.Service, patientsService patients.Service, clinicId string) (plan DuplicatesReportPlan, err error) {
	clinic, err := clinicsService.Get(ctx, clinicId)
	if err != nil {
		return
	}
	plan.Clinic = *clinic

	plan.Patients, err = listAllPatients(ctx, patientsService, *clinic)
	if err != nil {
		return
	}
	plan.PatientClusters, err = NewPatientClusterReporter(plan.Patients).Plan(ctx)
	if err != nil {
		return
	}
	plan.CreatedTime = time.Now()
	return
}

type DuplicatesReport struct {
	plan DuplicatesReportPlan
}

func NewDuplicatesReport(plan DuplicatesReportPlan) DuplicatesReport {
	return DuplicatesReport{plan: plan}
}

func (r DuplicatesReport) Generate() (*xlsx.File, error) {
	report := xlsx.NewFile()

	components := []func(report *xlsx.File) error{
		r.addSummarySheet,
		r.addPatients,
		r.addDuplicates,
	}
	for _, fn := range components {
		if err := fn(report); err != nil {
			return nil, err
		}
	}

	setColumnWidths(report)
	return report, nil
}

func (r DuplicatesReport) addSummarySheet(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetNameSummary)
	if err != nil {
		return err
	}

	sh.AddRow().AddCell().SetValue("SUMMARY")
	sh.AddRow()

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Report Generated")
	currentRow.AddCell().SetValue(r.plan.CreatedTime.Format(ReportTimeFormat))
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Workspace")
	currentRow.AddCell().SetValue(pointer.ToString(r.plan.Clinic.Name))
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Measures ---")
	currentRow.AddCell().SetValue("Count ---")
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Patient Accounts")
	currentRow.AddCell().SetValue(len(r.plan.Patients))
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Groups of Possible Duplicates")
	currentRow.AddCell().SetValue(len(r.plan.PatientClusters))

	return nil
}

func (r DuplicatesReport) addPatients(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetPatients)
	if err != nil {
		return err
	}

	addPatientsHeader(sh)
	tags := buildTagsMap(r.plan.Clinic.PatientTags)
	for _, patient := range r.plan.Patients {
		addPatientDetails(sh.AddRow(), patient, getUniquePatientTagNames(patient, tags), siteNames(patient.Sites, []sites.Site{}))
	}
	return nil
}

func (r DuplicatesReport) addDuplicates(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetDuplicates)
	if err != nil {
		return err
	}
	addDuplicatePatients(sh, r.plan.PatientClusters, r.plan.Clinic)
	return nil
}
