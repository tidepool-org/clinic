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

const (
	ReportSheetNameSummary          = "Summary"
	ReportSheetDuplicatesInClinic   = "Duplicates in "
	ReportSheetNameDuplicateClaimed = "Duplicate Claimed Accounts"
)

type Report struct {
	plan ClinicMergePlan
}

func NewReport(plan ClinicMergePlan) Report {
	return Report{plan: plan}
}

func (r Report) Generate() (*xlsx.File, error) {
	report := xlsx.NewFile()

	components := []func(report *xlsx.File) error{
		r.addSummarySheet,
		r.addSourcePatientClusters,
		r.addTargetPatientClusters,
	}
	for _, fn := range components {
		if err := fn(report); err != nil {
			return nil, err
		}
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

func (r Report) addSourcePatientClusters(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetDuplicatesInClinic + *r.plan.Source.Name)
	if err != nil {
		return err
	}
	addDuplicatePatients(sh, r.plan.SourcePatientClusters)
	return nil
}

func (r Report) addTargetPatientClusters(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetDuplicatesInClinic + *r.plan.Target.Name)
	if err != nil {
		return err
	}
	addDuplicatePatients(sh, r.plan.TargetPatientClusters)
	return nil
}

func (r Report) addDuplicateClaimedSheet(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetNameDuplicateClaimed)
	if err != nil {
		return err
	}

	sh.AddRow().AddCell().SetValue("DUPLICATE CLAIMED ACCOUNTS")
	sh.AddRow().AddCell().SetValue("For claimed accounts, all tags are retained but if there are differencees in Name, DOB, or MRN, we defer to Seastar Pediatric Endo. Please review the differences below.")
	sh.AddRow()

	currentRow := sh.AddRow()
	currentRow.AddCell()
	currentRow.AddCell().SetValue("Name ---")
	currentRow.AddCell().SetValue("DOB ---")
	currentRow.AddCell().SetValue("MRN ---")
	currentRow.AddCell().SetValue("Email ---")
	currentRow.AddCell().SetValue("Tags ---")

	count := 1
	for _, patientPlan := range r.plan.PatientsPlan {
		if patientPlan.PatientAction == PatientActionMerge {
			if conflicts, ok := patientPlan.Conflicts[PatientConflictCategoryDuplicateAccounts]; ok {
				conflict := conflicts[0]

				sh.AddRow().AddCell().SetValue(fmt.Sprintf("Patient %d", count))

				currentRow = sh.AddRow()
				currentRow.AddCell().SetValue("Source")
				currentRow.AddCell().SetValue(patientPlan.SourcePatient.FullName)
				currentRow.AddCell().SetValue(patientPlan.SourcePatient.BirthDate)
				currentRow.AddCell().SetValue(patientPlan.SourcePatient.Mrn)
				currentRow.AddCell().SetValue(patientPlan.SourcePatient.Email)
				currentRow.AddCell().SetValue(strings.Join(patientPlan.SourceTagNames, ", "))

				currentRow = sh.AddRow()
				currentRow.AddCell().SetValue("Destination")
				currentRow.AddCell().SetValue(patientPlan.TargetPatient.FullName)
				currentRow.AddCell().SetValue(patientPlan.TargetPatient.BirthDate)
				currentRow.AddCell().SetValue(patientPlan.TargetPatient.Mrn)
				currentRow.AddCell().SetValue(patientPlan.TargetPatient.Email)
				currentRow.AddCell().SetValue(strings.Join(patientPlan.TargetTagNames, ", "))

				currentRow = sh.AddRow()
				currentRow.AddCell().SetValue("Resulting Account")
				currentRow.AddCell().SetValue(conflict.Patient.FullName)
				currentRow.AddCell().SetValue(conflict.Patient.BirthDate)
				currentRow.AddCell().SetValue(conflict.Patient.Mrn)
				currentRow.AddCell().SetValue(conflict.Patient.Email)
				currentRow.AddCell().SetValue(strings.Join(patientPlan.PostMigrationTagNames, ", "))

				count += 1
			}

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
	currentRow.AddCell().SetValue("Merging from Workspace 1 (Source)")
	currentRow.AddCell().SetValue(*r.plan.Source.Name)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Merging to Workspace 2 (Target)")
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
				case PatientConflictCategoryDuplicateAccounts:
					duplicateClaimedCount++
				case PatientConflictCategoryLikelyDuplicateAccounts:
					likelyDuplicateCount++
				//case PatientConflictCategoryPossibleDuplicateAccounts:
				//	possibleDuplicateCount++
				case PatientConflictCategoryMRNOnlyMatch:
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

func addDuplicatePatients(sh *xlsx.Sheet, clusters PatientClusters) {
	currentRow := sh.AddRow()
	currentRow.AddCell()
	currentRow.AddCell().SetValue("Name ---")
	currentRow.AddCell().SetValue("Claimed? ---")
	currentRow.AddCell().SetValue("UUID ---")
	currentRow.AddCell().SetValue("DOB ---")
	currentRow.AddCell().SetValue("MRN ---")
	currentRow.AddCell().SetValue("Tags ---")
	currentRow.AddCell().SetValue("Latest Upload ---")
	currentRow.AddCell().SetValue("Likely Duplicates ---")
	currentRow.AddCell().SetValue("Name Only Matches ---")
	currentRow.AddCell().SetValue("MRN Only Matches ---")

	for i, cluster := range clusters {
		if len(cluster.Patients) > 1 {
			currentRow = sh.AddRow()
			currentRow.AddCell().SetValue("Review " + strconv.Itoa(i))

			for _, p := range cluster.Patients {
				currentRow = sh.AddRow()
				currentRow.AddCell().SetValue(p.Patient.FullName)
				currentRow.AddCell().SetValue(strconv.FormatBool(p.Patient.IsCustodial()))
				currentRow.AddCell().SetValue(p.Patient.UserId)
				currentRow.AddCell().SetValue(p.Patient.BirthDate)
				currentRow.AddCell().SetValue(p.Patient.Mrn)
				currentRow.AddCell().SetValue("") // TODO: Tags
				currentRow.AddCell().SetValue(p.Patient.Summary.GetLastUploadDate().Format(time.RFC3339))
				currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryLikelyDuplicateAccounts], ", "))
				currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryNameOnlyMatch], ", "))
				currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryMRNOnlyMatch], ", "))
			}
		}
	}
}
