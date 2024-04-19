package merge

import (
	"fmt"
	"github.com/tealeg/xlsx/v3"
	"github.com/tidepool-org/clinic/clinicians"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Report struct {
	plan ClinicMergePlan
}

func (r Report) GenerateReport() (*xlsx.File, error) {
	report := xlsx.NewFile()

	if err := r.addSummarySheet(report); err != nil {
		return nil, err
	}

	return report, nil
}

func (r Report) addSummarySheet(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetNameSummary)
	if err != nil {
		return err
	}

	components := []func(sh *xlsx.Sheet) error{
		r.addSummaryHeader,
		r.addSettingsSummary,
		r.addClinicianSummary,
		r.addTagsSummary,
		r.addMeasuresSummary,
	}
	for _, fn := range components {
		if err := fn(sh); err != nil {
			return err
		}
	}

	return nil
}

func (r Report) addSummaryHeader(sh *xlsx.Sheet) error {
	sh.AddRow().AddCell().SetValue("Summary")
	sh.AddRow()

	var currentRow *xlsx.Row
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Report Generated")
	currentRow.AddCell().SetValue(r.plan.CreatedTime.Format(time.RFC3339))
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Merging from Workspace 1")
	currentRow.AddCell().SetValue(*r.plan.Source.Name)
	currentRow.AddCell().SetValue("Merging to Workspace 2")
	currentRow.AddCell().SetValue(*r.plan.Target.Name)
	sh.AddRow()

	return nil
}

func (r Report) addSettingsSummary(sh *xlsx.Sheet) error {
	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Settings ---")
	currentRow.AddCell().SetValue("Do they match? ---")
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", *r.plan.Source.Name))
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", *r.plan.Target.Name))
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(TaskTypeClinicSettingsHasPartialSSO)
	currentRow.AddCell().SetValue(strconv.FormatBool(r.plan.MembershipRestrictionsMergePlan.ValuesMatch()))
	currentRow.AddCell().SetValue(r.plan.MembershipRestrictionsMergePlan.GetSourceValue())
	currentRow.AddCell().SetValue(r.plan.MembershipRestrictionsMergePlan.GetTargetValue())

	for _, s := range r.plan.SettingsPlan {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(s.Name)
		currentRow.AddCell().SetValue(strconv.FormatBool(s.ValuesMatch()))
		currentRow.AddCell().SetValue(s.SourceValue)
		currentRow.AddCell().SetValue(s.TargetValue)
	}
	sh.AddRow().AddCell().SetValue("*If the target clinic has partial SSO and the source clinic does not, the clinic users in the source clinic should be manually invited to the target clinic before the merge. This way their SSO configuration will be correct.")
	sh.AddRow()

	return nil
}

func (r Report) addClinicianSummary(sh *xlsx.Sheet) error {
	adminTasks := make([]ClinicianPlan, 0)
	nonAdminTasks := make([]ClinicianPlan, 0)

	for _, c := range r.plan.CliniciansPlan {
		if c.ClinicianAction == ClinicianActionMergeInto {
			// Results will be reported by the corresponding source merge task
			continue
		}
		if slices.Contains(c.ResultingRoles, clinicians.ClinicAdmin) {
			adminTasks = append(adminTasks, c)
		} else {
			nonAdminTasks = append(nonAdminTasks, c)
		}
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Admins (%v)", len(adminTasks)))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Email ---")
	for _, a := range adminTasks {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(a.Name)
		currentRow.AddCell().SetValue(a.Workspaces)
		currentRow.AddCell().SetValue(a.Email)
	}
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Members (%v)", len(nonAdminTasks)))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Email ---")
	currentRow.AddCell().SetValue("Downgrade (Only if the person is an Admin at Workspace 1 but a Member at Workspace 2) ----")
	for _, plan := range nonAdminTasks {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(plan.Name)
		currentRow.AddCell().SetValue(plan.Workspaces)
		currentRow.AddCell().SetValue(plan.Email)
		if plan.Downgraded {
			currentRow.AddCell().SetValue("Yes")
		}
	}
	sh.AddRow()

	return nil
}

func (r Report) addTagsSummary(sh *xlsx.Sheet) error {
	resultingTagsCount := 0
	for _, plan := range r.plan.TagsPlan {
		if plan.TagAction == TagActionCreate || plan.TagAction == TagActionKeep {
			resultingTagsCount += 1
		}
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Tags (%v) ---", resultingTagsCount))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Merge ---")

	for _, plan := range r.plan.TagsPlan {
		if plan.TagAction == TagActionSkip {
			continue
		}

		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(plan.Name)
		currentRow.AddCell().SetValue(strings.Join(plan.Workspaces, ", "))
		currentRow.AddCell().SetValue(plan.Workspaces)
		if plan.Merge == true {
			currentRow.AddCell().SetValue("Yes")
		}
	}
	sh.AddRow()

	return nil
}

func (r Report) addMeasuresSummary(sh *xlsx.Sheet) error {
	adminTasks := make([]ClinicianPlan, 0)
	nonAdminTasks := make([]ClinicianPlan, 0)
	membersDowngraded := 0

	for _, plan := range r.plan.CliniciansPlan {
		if plan.ClinicianAction == ClinicianActionMergeInto {
			// Results will be reported by the corresponding source merge task
			continue
		}
		if slices.Contains(plan.ResultingRoles, clinicians.ClinicAdmin) {
			adminTasks = append(adminTasks, plan)
		} else {
			nonAdminTasks = append(nonAdminTasks, plan)
			if plan.Downgraded {
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
	for _, plan := range r.plan.TagsPlan {
		if plan.TagAction == TagActionCreate || plan.TagAction == TagActionKeep {
			resultingTagsCount++
		}
		if plan.TagAction == TagActionSkip {
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

	for _, plan := range r.plan.PatientsPlan {
		if plan.PatientAction == PatientActionMergeInto {
			continue
		}

		resultingPatientsCount++

		for _, conflicts := range plan.Conflicts {
			for _, conflict := range conflicts {
				switch conflict.Category {
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
