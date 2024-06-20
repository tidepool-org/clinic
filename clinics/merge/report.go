package merge

import (
	"fmt"
	"github.com/tealeg/xlsx/v3"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/pointer"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	ReportSheetNameSummary                     = "Summary"
	ReportSheetDuplicatesInSourceClinic        = "Duplicates in Source Clinic"
	ReportSheetDuplicatesInTargetClinic        = "Duplicates in Target Clinic"
	ReportSheetNameDuplicateClaimed            = "Duplicate Claimed Accounts"
	ReportSheetNameDuplicatesInMergedWorkspace = "Duplicates in Merged Workspace"
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
		r.addDuplicatesInMergedSheet,
	}
	for _, fn := range components {
		if err := fn(report); err != nil {
			return nil, err
		}
	}

	for _, sh := range report.Sheets {
		sh.SetColWidth(1, 1, 50)
		for i := 2; i <= sh.MaxCol; i++ {
			_ = sh.SetColAutoWidth(i, xlsx.DefaultAutoWidth)
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
	sh, err := report.AddSheet(ReportSheetDuplicatesInSourceClinic)
	if err != nil {
		return err
	}
	addDuplicatePatients(sh, r.plan.SourcePatientClusters, r.plan.Source)
	return nil
}

func (r Report) addTargetPatientClusters(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetDuplicatesInTargetClinic)
	if err != nil {
		return err
	}
	addDuplicatePatients(sh, r.plan.TargetPatientClusters, r.plan.Target)
	return nil
}

func (r Report) addDuplicatesInMergedSheet(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetNameDuplicatesInMergedWorkspace)
	if err != nil {
		return err
	}

	targetTags := buildTagsMap(r.plan.Target.PatientTags)

	sh.AddRow().AddCell().SetValue("MERGED WORKSPACE PATIENT REVIEW")
	sh.AddRow().AddCell().SetValue("- We have identified the following patients from the Source Clinic that appear to be duplicates of one or more patients in the Target Clinic. You can see the original patient in the row next to \"Patient 1,\" and what we are doing with the patient under the \"Status\" column")
	sh.AddRow().AddCell().SetValue("- Accounts with the same UUID are exact matches. While all diabetes data is already synced, workspaces may have different descriptive text for these accounts. All tags will be retained and other fields will defer to the Target Clinic if they differ.")
	sh.AddRow().AddCell().SetValue("- Likely duplicates match in at least 2 fields out of Name, DOB, and MRN (blank fields do not count as matches). All of these accounts will be brought into the resulting workspace and should be manually reviewed to remove duplicates.")
	sh.AddRow()

	currentRow := sh.AddRow()
	currentRow.AddCell()
	currentRow.AddCell().SetValue("Status ---")
	currentRow.AddCell().SetValue("Original Workspace ---")
	currentRow.AddCell().SetValue("Claimed ---")
	currentRow.AddCell().SetValue("UUID ---")
	currentRow.AddCell().SetValue("Name ---")
	currentRow.AddCell().SetValue("DOB ---")
	currentRow.AddCell().SetValue("MRN ---")
	currentRow.AddCell().SetValue("Tags ---")
	currentRow.AddCell().SetValue("Latest Upload ---")

	count := 1
	for _, patientPlan := range r.plan.PatientsPlan {
		if patientPlan.SourcePatient == nil || !patientPlan.HasConflicts() {
			continue
		}

		status := "(retained)"
		if patientPlan.PatientAction == PatientActionMerge {
			status = "(combined)"
		}
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(fmt.Sprintf("Patient %v", count))
		currentRow.AddCell().SetValue(status)
		currentRow.AddCell().SetValue(pointer.ToString(r.plan.Source.Name))
		if !patientPlan.SourcePatient.IsCustodial() {
			currentRow.AddCell().SetValue("Y")
		} else {
			currentRow.AddCell().SetValue("-")
		}
		currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.UserId))
		currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.FullName))
		currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.BirthDate))
		currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.Mrn))
		currentRow.AddCell().SetValue(strings.Join(patientPlan.SourceTagNames, ", "))
		if patientPlan.SourcePatient.Summary != nil {
			lastUpload := patientPlan.SourcePatient.Summary.GetLastUploadDate()
			if !lastUpload.IsZero() {
				currentRow.AddCell().SetValue(lastUpload.Format(time.DateOnly))
			}
		}

		conflicts := patientPlan.Conflicts[PatientConflictCategoryDuplicateAccounts]
		if num := len(conflicts); num > 1 {
			return fmt.Errorf("unexpected number of duplicate accounts %v for patient %s", num, *patientPlan.SourcePatient.UserId)
		} else if num == 1 {
			conflict := conflicts[0]
			currentRow = sh.AddRow()
			currentRow.AddCell()
			currentRow.AddCell().SetValue("Exact Match: Results in one claimed account")

			currentRow = sh.AddRow()
			currentRow.AddCell()
			currentRow.AddCell().SetValue("(result)")
			currentRow.AddCell()
			if !conflict.Patient.IsCustodial() {
				currentRow.AddCell().SetValue("Y")
			} else {
				currentRow.AddCell().SetValue("-")
			}
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.UserId))
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.FullName))
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.BirthDate))
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.Mrn))
			currentRow.AddCell().SetValue(strings.Join(patientPlan.PostMigrationTagNames, ", "))

			currentRow = sh.AddRow()
			currentRow.AddCell()
			currentRow.AddCell().SetValue("(combined)")
			currentRow.AddCell().SetValue(pointer.ToString(r.plan.Target.Name))
			if !conflict.Patient.IsCustodial() {
				currentRow.AddCell().SetValue("Y")
			} else {
				currentRow.AddCell().SetValue("-")
			}
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.UserId))
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.FullName))
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.BirthDate))
			currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.Mrn))
			currentRow.AddCell().SetValue(strings.Join(patientPlan.TargetTagNames, ", "))
			if conflict.Patient.Summary != nil {
				lastUpload := conflict.Patient.Summary.GetLastUploadDate()
				if !lastUpload.IsZero() {
					currentRow.AddCell().SetValue(lastUpload.Format(time.DateOnly))
				}
			}
		}

		conflictCategories := map[string]string{
			PatientConflictCategoryLikelyDuplicateAccounts: "Review likely duplicate(s):",
			PatientConflictCategoryNameOnlyMatch:           "Review duplicate name:",
			PatientConflictCategoryMRNOnlyMatch:            "Review duplicate MRN:",
		}
		for category, description := range conflictCategories {
			conflicts = patientPlan.Conflicts[category]
			if len(conflicts) == 0 {
				continue
			}

			currentRow = sh.AddRow()
			currentRow.AddCell()
			currentRow.AddCell().SetValue(description)

			for _, conflict := range conflicts {
				currentRow = sh.AddRow()
				currentRow.AddCell()
				currentRow.AddCell().SetValue("(retained)")
				currentRow.AddCell().SetValue(pointer.ToString(r.plan.Target.Name))
				if !conflict.Patient.IsCustodial() {
					currentRow.AddCell().SetValue("Y")
				} else {
					currentRow.AddCell().SetValue("-")
				}
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.UserId))
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.FullName))
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.BirthDate))
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.Mrn))
				currentRow.AddCell().SetValue(strings.Join(getPatientTagNames(conflict.Patient, targetTags), ", "))
				if conflict.Patient.Summary != nil {
					lastUpload := conflict.Patient.Summary.GetLastUploadDate()
					if !lastUpload.IsZero() {
						currentRow.AddCell().SetValue(lastUpload.Format(time.DateOnly))
					}
				}
			}
		}

		sh.AddRow()
		count += 1
	}

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
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.FullName))
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.BirthDate))
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.Mrn))
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.SourcePatient.Email))
				currentRow.AddCell().SetValue(strings.Join(patientPlan.SourceTagNames, ", "))

				currentRow = sh.AddRow()
				currentRow.AddCell().SetValue("Destination")
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.TargetPatient.FullName))
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.TargetPatient.BirthDate))
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.TargetPatient.Mrn))
				currentRow.AddCell().SetValue(pointer.ToString(patientPlan.TargetPatient.Email))
				currentRow.AddCell().SetValue(strings.Join(patientPlan.TargetTagNames, ", "))

				currentRow = sh.AddRow()
				currentRow.AddCell().SetValue("Resulting Account")
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.FullName))
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.BirthDate))
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.Mrn))
				currentRow.AddCell().SetValue(pointer.ToString(conflict.Patient.Email))
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
	currentRow.AddCell().SetValue(pointer.ToString(r.plan.Source.Name))
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Merging to Workspace 2 (Target)")
	currentRow.AddCell().SetValue(pointer.ToString(r.plan.Target.Name))
	sh.AddRow()

	return nil
}

func (r Report) addSettingsSummary(sh *xlsx.Sheet) error {
	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Settings ---")
	currentRow.AddCell().SetValue("Do they match? ---")
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", pointer.ToString(r.plan.Source.Name)))
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", pointer.ToString(r.plan.Target.Name)))
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(TaskTypeClinicSettingsHasPartialSSO)
	if r.plan.MembershipRestrictionsMergePlan.ValuesMatch() {
		currentRow.AddCell().SetValue("Yes")
	} else {
		currentRow.AddCell().SetValue("No")
	}
	currentRow.AddCell().SetValue(r.plan.MembershipRestrictionsMergePlan.GetSourceValue())
	currentRow.AddCell().SetValue(r.plan.MembershipRestrictionsMergePlan.GetTargetValue())

	for _, s := range r.plan.SettingsPlan {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(s.Name)
		if s.ValuesMatch() {
			currentRow.AddCell().SetValue("Yes")
		} else {
			currentRow.AddCell().SetValue("No")
		}
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
		currentRow.AddCell().SetValue(strings.Join(a.Workspaces, ", "))
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
		currentRow.AddCell().SetValue(strings.Join(plan.Workspaces, ", "))
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
		if plan.TagAction == TagActionCreate || plan.TagAction == TagActionRetain {
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
		if plan.TagAction == TagActionCreate || plan.TagAction == TagActionRetain {
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
	duplicateAccountsCounts := 0
	likelyDuplicateCount := 0
	duplicateMRNsCount := 0
	duplicateNamesCount := 0

	for _, plan := range r.plan.PatientsPlan {
		if plan.PatientAction == PatientActionMergeInto {
			continue
		}

		resultingPatientsCount++

		for _, conflicts := range plan.Conflicts {
			for _, conflict := range conflicts {
				switch conflict.Category {
				case PatientConflictCategoryDuplicateAccounts:
					duplicateAccountsCounts++
				case PatientConflictCategoryLikelyDuplicateAccounts:
					likelyDuplicateCount++
				case PatientConflictCategoryMRNOnlyMatch:
					duplicateMRNsCount++
				case PatientConflictCategoryNameOnlyMatch:
					duplicateNamesCount++
				}
			}
		}
	}

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Resulting Patient Accounts")
	currentRow.AddCell().SetValue(resultingPatientsCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate Accounts")
	currentRow.AddCell().SetValue(duplicateAccountsCounts)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Likely Duplicate Accounts")
	currentRow.AddCell().SetValue(likelyDuplicateCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate MRNs")
	currentRow.AddCell().SetValue(duplicateMRNsCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate Names")
	currentRow.AddCell().SetValue(duplicateNamesCount)

	return nil
}

func addDuplicatePatients(sh *xlsx.Sheet, clusters PatientClusters, clinic clinics.Clinic) {
	sh.AddRow().AddCell().SetValue(pointer.ToString(clinic.Name))
	sh.AddRow().AddCell().SetValue("Review Possible Duplicates")
	sh.AddRow().AddCell().SetValue("- Below we list groups of patients that appear to be duplicate accounts. For each patient in the group, you can see their connection with the other patients in the \"Likely Duplicates\", \"Name Only Matches\" and \"MRN Only Matches\" columns.")
	sh.AddRow().AddCell().SetValue("- If the patient matches another patient in the group on 2 or more of the following -- MRN, DOB or name -- then we will list that matching patient's ID in the \"Likely Duplicates\" column. If the patient matches another on name only, or MRN only, we list those IDs in the corresponding columns.")
	sh.AddRow().AddCell().SetValue("- If evaluating a patient account with an associated ID abc1234def, you will see this URL when you view their data: https://app.tidepool.org/patients/abc1234def/data. This URL configuration will help you quickly confirm which patient account you wish to keep, and which you wish to remove.")
	sh.AddRow().AddCell().SetValue("- Each patient appears only once, so after you resolve each group you don't have to backtrack.")
	sh.AddRow()

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

	tags := buildTagsMap(clinic.PatientTags)
	count := 1
	for _, cluster := range clusters {
		if len(cluster.Patients) > 1 {
			currentRow = sh.AddRow()
			currentRow.AddCell().SetValue("Review " + strconv.Itoa(count))

			count += 1
			for _, p := range cluster.Patients {
				currentRow = sh.AddRow()
				currentRow.AddCell()
				currentRow.AddCell().SetValue(pointer.ToString(p.Patient.FullName))
				if !p.Patient.IsCustodial() {
					currentRow.AddCell().SetValue("Y")
				} else {
					currentRow.AddCell().SetValue("-")
				}
				currentRow.AddCell().SetValue(pointer.ToString(p.Patient.UserId))
				currentRow.AddCell().SetValue(pointer.ToString(p.Patient.BirthDate))
				currentRow.AddCell().SetValue(pointer.ToString(p.Patient.Mrn))
				currentRow.AddCell().SetValue(strings.Join(getPatientTagNames(p.Patient, tags), ", "))
				if p.Patient.Summary != nil && !p.Patient.Summary.GetLastUploadDate().IsZero() {
					currentRow.AddCell().SetValue(p.Patient.Summary.GetLastUploadDate().Format(time.RFC3339))
				} else {
					currentRow.AddCell()
				}
				currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryLikelyDuplicateAccounts], ", "))
				currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryNameOnlyMatch], ", "))
				currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryMRNOnlyMatch], ", "))
			}

			sh.AddRow()
		}
	}
}
