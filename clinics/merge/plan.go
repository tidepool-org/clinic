package merge

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tealeg/xlsx/v3"
)

const (
	ReportSheetNameSummary = "Summary"
)

type Plan struct {
	Source clinics.Clinic
	Target clinics.Clinic

	SettingsTasks  []Task[SettingsReportDetails]
	TagTasks       []Task[TagReportDetails]
	ClinicianTasks []Task[ClinicianReportDetails]
	PatientTasks   []Task[PatientReportDetails]

	CreatedTime time.Time
}

func (p Plan) DryRun(ctx context.Context) error {
	return p.doRun(ctx, DryRunner)
}

func (p Plan) Run(ctx context.Context) error {
	return p.doRun(ctx, Runner)
}

func (p Plan) doRun(ctx context.Context, getRunner GetRunner) error {
	for _, t := range p.SettingsTasks {
		if err := getRunner(t)(ctx); err != nil {
			return err
		}
	}
	for _, t := range p.TagTasks {
		if err := getRunner(t)(ctx); err != nil {
			return err
		}
	}
	for _, t := range p.ClinicianTasks {
		if err := getRunner(t)(ctx); err != nil {
			return err
		}
	}
	for _, t := range p.PatientTasks {
		if err := getRunner(t)(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p Plan) CompileReport() (*xlsx.File, error) {
	report := xlsx.NewFile()

	return report, nil
}

func (p Plan) addSummary(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetNameSummary)
	if err != nil {
		return err
	}

	components := []func(sh *xlsx.Sheet) error{
		p.addSummaryHeader,
		p.addSettingsSummary,
		p.addClinicianSummary,
		p.addTagsSummary,
		p.addMeasuresSummary,
	}
	for _, fn := range components {
		if err := fn(sh); err != nil {
			return err
		}
	}

	return nil
}

func (p Plan) addSummaryHeader(sh *xlsx.Sheet) error {
	sh.AddRow().AddCell().SetValue("Summary")
	sh.AddRow()

	var currentRow *xlsx.Row
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Report Generated")
	currentRow.AddCell().SetValue(p.CreatedTime.Format(time.RFC3339))
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Merging from Workspace 1")
	currentRow.AddCell().SetValue(*p.Source.Name)
	currentRow.AddCell().SetValue("Merging to Workspace 2")
	currentRow.AddCell().SetValue(*p.Target.Name)
	sh.AddRow()

	return nil
}

func (p Plan) addSettingsSummary(sh *xlsx.Sheet) error {
	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Settings ---")
	currentRow.AddCell().SetValue("Do they match? ---")
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", *p.Source.Name))
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", *p.Target.Name))
	sh.AddRow()

	for _, t := range p.SettingsTasks {
		r, err := t.GetResult()
		if err != nil {
			return err
		}
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(r.ReportDetails.Name)
		currentRow.AddCell().SetValue(strconv.FormatBool(r.ReportDetails.ValuesMatch))
		currentRow.AddCell().SetValue(r.ReportDetails.SourceValue)
		currentRow.AddCell().SetValue(r.ReportDetails.TargetValue)
	}
	sh.AddRow().AddCell().SetValue("*If the target clinic has partial SSO and the source clinic does not, the clinic users in the source clinic should be manually invited to the target clinic before the merge. This way their SSO configuration will be correct.")
	sh.AddRow()

	return nil
}

func (p Plan) addClinicianSummary(sh *xlsx.Sheet) error {
	adminTasks := make([]Task[ClinicianReportDetails], 0)
	nonAdminTasks := make([]Task[ClinicianReportDetails], 0)

	for _, t := range p.ClinicianTasks {
		r, err := t.GetResult()
		if err != nil {
			return err
		}
		if r.ReportDetails.ClinicianAction == ClinicianActionMergeInto {
			// Results will be reported by the corresponding source merge task
			continue
		}
		if slices.Contains(r.ReportDetails.ResultingRoles, clinicians.ClinicAdmin) {
			adminTasks = append(adminTasks, t)
		} else {
			nonAdminTasks = append(nonAdminTasks, t)
		}
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Admins (%v)", len(adminTasks)))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Email ---")
	for _, t := range adminTasks {
		r, _ := t.GetResult()
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(r.ReportDetails.Name)
		currentRow.AddCell().SetValue(r.ReportDetails.Workspaces)
		currentRow.AddCell().SetValue(r.ReportDetails.Email)
	}
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Members (%v)", len(nonAdminTasks)))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Email ---")
	currentRow.AddCell().SetValue("Downgrade (Only if the person is an Admin at Workspace 1 but a Member at Workspace 2) ----")
	for _, t := range nonAdminTasks {
		r, _ := t.GetResult()
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(r.ReportDetails.Name)
		currentRow.AddCell().SetValue(r.ReportDetails.Workspaces)
		currentRow.AddCell().SetValue(r.ReportDetails.Email)
		if r.ReportDetails.Downgraded {
			currentRow.AddCell().SetValue("Yes")
		}
	}
	sh.AddRow()

	return nil
}

func (p Plan) addTagsSummary(sh *xlsx.Sheet) error {
	resultingTagsCount := 0
	for _, t := range p.TagTasks {
		r, err := t.GetResult()
		if err != nil {
			return err
		}
		if r.ReportDetails.TagAction == TagActionCreate || r.ReportDetails.TagAction == TagActionKeep {
			resultingTagsCount += 1
		}
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Tags (%v) ---", resultingTagsCount))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Merge ---")

	for _, t := range p.TagTasks {
		r, err := t.GetResult()
		if err != nil {
			return err
		}
		if r.ReportDetails.TagAction == TagActionSkip {
			continue
		}

		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(r.ReportDetails.Name)
		currentRow.AddCell().SetValue(strings.Join(r.ReportDetails.Workspaces, ", "))
		currentRow.AddCell().SetValue(r.ReportDetails.Workspaces)
		if r.ReportDetails.Merge == true {
			currentRow.AddCell().SetValue("Yes")
		}
	}
	sh.AddRow()

	return nil
}

func (p Plan) addMeasuresSummary(sh *xlsx.Sheet) error {
	adminTasks := make([]Task[ClinicianReportDetails], 0)
	nonAdminTasks := make([]Task[ClinicianReportDetails], 0)
	membersDowngraded := 0

	for _, t := range p.ClinicianTasks {
		r, err := t.GetResult()
		if err != nil {
			return err
		}
		if r.ReportDetails.ClinicianAction == ClinicianActionMergeInto {
			// Results will be reported by the corresponding source merge task
			continue
		}
		if slices.Contains(r.ReportDetails.ResultingRoles, clinicians.ClinicAdmin) {
			adminTasks = append(adminTasks, t)
		} else {
			nonAdminTasks = append(nonAdminTasks, t)
			if r.ReportDetails.Downgraded {
				membersDowngraded++
			}
		}
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Measures ---")
	currentRow.AddCell().SetValue("Count ---")

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Resulting Members & Admins")
	currentRow.AddCell().SetValue(len(adminTasks) + len(nonAdminTasks))

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Members downgraded from Admin")
	currentRow.AddCell().SetValue(membersDowngraded)

	resultingTagsCount := 0
	duplicateTagsCount := 0
	for _, t := range p.TagTasks {
		r, _ := t.GetResult()
		if r.ReportDetails.TagAction == TagActionCreate || r.ReportDetails.TagAction == TagActionKeep {
			resultingTagsCount++
		}
		if r.ReportDetails.TagAction == TagActionSkip {
			duplicateTagsCount++
		}
	}

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Resulting Tags")
	currentRow.AddCell().SetValue(resultingTagsCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate tags that will be merged")
	currentRow.AddCell().SetValue(duplicateTagsCount)

	resultingPatientsCount := 0
	duplicateClaimedCount := 0
	likelyDuplicateCount := 0
	possibleDuplicateCount := 0
	duplicateMRNsCount := 0

	for _, t := range p.PatientTasks {
		r, _ := t.GetResult()
		if r.ReportDetails.PatientAction == PatientActionMergeInto {
			continue
		}

		resultingPatientsCount++

		for _, c := range r.ReportDetails.ConflictCategories {
			switch c {
			case PatientConflictCategoryDuplicateClaimed:
				duplicateClaimedCount++
			case PatientConflictCategoryLikelyDuplicateAccounts:
				likelyDuplicateCount++
			case PatientConflictCategoryPossibleDuplicateAccounts:
				possibleDuplicateCount++
			case PatientConflictCategoryDuplicateMRNs:
				duplicateMRNsCount++
			}
		}
	}

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Resulting Patient Accounts")
	currentRow.AddCell().SetValue(resultingPatientsCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate Claimed Accounts")
	currentRow.AddCell().SetValue(duplicateClaimedCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Likely Duplicate Accounts")
	currentRow.AddCell().SetValue(likelyDuplicateCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Possible Duplicate Accounts")
	currentRow.AddCell().SetValue(possibleDuplicateCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate MRNs (likely typos)")
	currentRow.AddCell().SetValue(possibleDuplicateCount)

	return nil
}
