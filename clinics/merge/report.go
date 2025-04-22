package merge

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tealeg/xlsx/v3"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/pointer"
	"github.com/tidepool-org/clinic/sites"
)

const (
	ReportSheetNameSummary                     = "Summary"
	ReportSheetPatientsInSourceClinic          = "Patients in Source Clinic"
	ReportSheetPatientsInTargetClinic          = "Patients in Target Clinic"
	ReportSheetDuplicatesInSourceClinic        = "Duplicates in Source Clinic"
	ReportSheetDuplicatesInTargetClinic        = "Duplicates in Target Clinic"
	ReportSheetNameDuplicatesInMergedWorkspace = "Duplicates in Merged Workspace"
	ReportTimeFormat                           = "January _2 2006 15:04:05 MST"
	LastUploadTimeFormat                       = time.DateTime
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
		r.addSourcePatients,
		r.addTargetPatients,
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
		r.addMeasuresSummary,
		r.addClinicianSummary,
		r.addTagsSummary,
		r.addSitesSummary,
	}
	for _, fn := range components {
		if err := fn(sh); err != nil {
			return err
		}
	}

	return nil
}

func (r Report) addSourcePatients(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetPatientsInSourceClinic)
	if err != nil {
		return err
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Name ---")
	currentRow.AddCell().SetValue("Claimed ---")
	currentRow.AddCell().SetValue("UUID ---")
	currentRow.AddCell().SetValue("DOB ---")
	currentRow.AddCell().SetValue("MRN ---")
	currentRow.AddCell().SetValue("Tags ---")
	currentRow.AddCell().SetValue("Sites ---")
	currentRow.AddCell().SetValue("Latest Upload ---")
	sh.AddRow()

	for _, plan := range r.plan.PatientPlans.GetSourcePatientPlans() {
		sourceSiteNames := []string{}
		for _, site := range (*plan.SourcePatient).Sites {
			sourceSiteNames = append(sourceSiteNames, site.Name)
		}
		addPatientDetails(sh.AddRow(), *plan.SourcePatient, plan.SourceTagNames, r.plan.Target.Sites)
	}
	return nil
}

func (r Report) addTargetPatients(report *xlsx.File) error {
	sh, err := report.AddSheet(ReportSheetPatientsInTargetClinic)
	if err != nil {
		return err
	}

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Name ---")
	currentRow.AddCell().SetValue("Claimed ---")
	currentRow.AddCell().SetValue("UUID ---")
	currentRow.AddCell().SetValue("DOB ---")
	currentRow.AddCell().SetValue("MRN ---")
	currentRow.AddCell().SetValue("Tags ---")
	currentRow.AddCell().SetValue("Sites ---")
	currentRow.AddCell().SetValue("Latest Upload ---")
	sh.AddRow()

	for _, plan := range r.plan.PatientPlans.GetTargetPatientPlans() {
		addPatientDetails(sh.AddRow(), *plan.TargetPatient, plan.TargetTagNames, r.plan.Target.Sites)
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
	currentRow.AddCell().SetValue("Name ---")
	currentRow.AddCell().SetValue("Claimed ---")
	currentRow.AddCell().SetValue("UUID ---")
	currentRow.AddCell().SetValue("DOB ---")
	currentRow.AddCell().SetValue("MRN ---")
	currentRow.AddCell().SetValue("Tags ---")
	currentRow.AddCell().SetValue("Sites ---")
	currentRow.AddCell().SetValue("Latest Upload ---")

	count := 1
	for _, patientPlan := range r.plan.PatientPlans {
		if patientPlan.SourcePatient == nil || !patientPlan.HasConflicts() {
			continue
		}

		status := "(retained)"
		if patientPlan.PreventsMerge() {
			status = "(ERROR)"
		} else if patientPlan.PatientAction == PatientActionMerge {
			status = "(combined)"
		}
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(fmt.Sprintf("Patient %v", count))
		currentRow.AddCell().SetValue(status)
		currentRow.AddCell().SetValue(pointer.ToString(r.plan.Source.Name))
		addPatientDetails(currentRow, *patientPlan.SourcePatient, patientPlan.SourceTagNames, r.plan.Target.Sites)

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
			addPatientDetails(currentRow, conflict.Patient, patientPlan.PostMigrationTagNames, r.plan.Target.Sites)

			currentRow = sh.AddRow()
			currentRow.AddCell()
			currentRow.AddCell().SetValue("(combined)")
			currentRow.AddCell().SetValue(pointer.ToString(r.plan.Target.Name))
			addPatientDetails(currentRow, conflict.Patient, patientPlan.TargetTagNames, r.plan.Target.Sites)
		}

		conflictCategories := map[string]string{
			PatientConflictCategoryLikelyDuplicateAccounts: "Review likely duplicate(s)",
			PatientConflictCategoryNameOnlyMatch:           "Review duplicate name",
			PatientConflictCategoryMRNOnlyMatch:            "Review duplicate MRN",
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
				addPatientDetails(currentRow, conflict.Patient, getUniquePatientTagNames(conflict.Patient, targetTags), r.plan.Target.Sites)
			}
		}

		sh.AddRow()
		count += 1
	}

	return nil
}

func (r Report) addSummaryHeader(sh *xlsx.Sheet) error {
	sh.AddRow().AddCell().SetValue("SUMMARY")
	sh.AddRow()

	var currentRow *xlsx.Row
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Report Generated")
	currentRow.AddCell().SetValue(r.plan.CreatedTime.Format(ReportTimeFormat))
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Merging from Workspace 1 (Source)")
	currentRow.AddCell().SetValue(pointer.ToString(r.plan.Source.Name))
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Merging to Workspace 2 (Target)")
	currentRow.AddCell().SetValue(pointer.ToString(r.plan.Target.Name))
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Can execute merge plan?")
	if r.plan.PreventsMerge() {
		currentRow.AddCell().SetValue("No")
		errs := GetUniqueErrorMessages(r.plan.Errors())
		errMessage := fmt.Sprintf("No. %s", strings.Join(errs, "; "))
		currentRow.AddCell().SetValue(errMessage)
	} else {
		currentRow.AddCell().SetValue("Yes")
	}
	sh.AddRow()

	return nil
}

func (r Report) addSettingsSummary(sh *xlsx.Sheet) error {
	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue("Settings ---")
	currentRow.AddCell().SetValue("Do they match? ---")
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", pointer.ToString(r.plan.Source.Name)))
	currentRow.AddCell().SetValue(fmt.Sprintf("%s ---", pointer.ToString(r.plan.Target.Name)))

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(TaskTypeClinicSettingsHasPartialSSO)
	if r.plan.MembershipRestrictionsMergePlan.ValuesMatch() {
		currentRow.AddCell().SetValue("Yes")
	} else {
		currentRow.AddCell().SetValue("No")
	}
	currentRow.AddCell().SetValue(r.plan.MembershipRestrictionsMergePlan.GetSourceValue())
	currentRow.AddCell().SetValue(r.plan.MembershipRestrictionsMergePlan.GetTargetValue())

	for _, s := range r.plan.SettingsPlans {
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

	for _, c := range r.plan.ClinicianPlans {
		if c.ClinicianAction == ClinicianActionMergeInto {
			// Results will be reported by the corresponding source merge task
			continue
		}
		if slices.Contains(c.ResultingRoles, clinicians.RoleClinicAdmin) {
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
		currentRow.AddCell().SetValue(a.GetClinicianName())
		currentRow.AddCell().SetValue(strings.Join(a.Workspaces, ", "))
		currentRow.AddCell().SetValue(a.GetClinicianEmail())
	}
	sh.AddRow()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Members (%v)", len(nonAdminTasks)))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Email ---")
	currentRow.AddCell().SetValue("Downgrade (Only if the person is an Admin at Workspace 1 but a Member at Workspace 2) ----")
	for _, plan := range nonAdminTasks {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(plan.GetClinicianName())
		currentRow.AddCell().SetValue(strings.Join(plan.Workspaces, ", "))
		currentRow.AddCell().SetValue(plan.GetClinicianEmail())
		if plan.Downgraded {
			currentRow.AddCell().SetValue("Yes")
		}
	}
	sh.AddRow()

	pendingInvites := 0
	for _, count := range r.plan.ClinicianPlans.PendingInvitesByWorkspace() {
		pendingInvites += count
	}

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Pending Invites (%v)", pendingInvites))
	currentRow.AddCell().SetValue("Workspace ---")
	for workspace, count := range r.plan.ClinicianPlans.PendingInvitesByWorkspace() {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(count)
		currentRow.AddCell().SetValue(workspace)
	}
	sh.AddRow()

	return nil
}

func (r Report) addTagsSummary(sh *xlsx.Sheet) error {
	resultingTagsCount := r.plan.TagsPlans.GetResultingTagsCount()

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Tags (%v) ---", resultingTagsCount))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Merge ---")

	for _, plan := range r.plan.TagsPlans {
		if plan.TagAction == TagActionSkip {
			continue
		}

		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue(plan.Name)
		currentRow.AddCell().SetValue(strings.Join(plan.Workspaces, ", "))
		if plan.Merge {
			currentRow.AddCell().SetValue("Yes")
		}
	}
	sh.AddRow()

	return nil
}

func (r Report) addSitesSummary(sh *xlsx.Sheet) error {
	resultingSitesCount := r.plan.SitesPlans.GetResultingSitesCount()

	currentRow := sh.AddRow()
	currentRow.AddCell().SetValue(fmt.Sprintf("Resulting Sites (%d) ---", resultingSitesCount))
	currentRow.AddCell().SetValue("Workspace ---")
	currentRow.AddCell().SetValue("Action ---")

	slices.SortFunc(r.plan.SitesPlans, func(i, j SitePlan) int {
		return cmp.Compare(strings.ToLower(i.Name()), strings.ToLower(j.Name()))
	})
	for _, plan := range r.plan.SitesPlans {
		currentRow = sh.AddRow()
		switch plan.Action {
		case SiteActionMove:
			currentRow.AddCell().SetValue(plan.Name())
			currentRow.AddCell().SetValue(plan.SourceWorkspace)
			currentRow.AddCell().SetValue("Create")
		case SiteActionRename:
			currentRow.AddCell().SetValue(plan.Name())
			currentRow.AddCell().SetValue(plan.SourceWorkspace)
			currentRow.AddCell().SetValue("Rename")
		case SiteActionRetain:
			currentRow.AddCell().SetValue(plan.Name())
			currentRow.AddCell().SetValue(plan.SourceWorkspace)
			currentRow.AddCell().SetValue("Retain")
		default:
			return fmt.Errorf("unhandled site action %s: %s", plan.Name(), plan.Action)
		}
	}
	sh.AddRow()

	return nil
}

func (r Report) addMeasuresSummary(sh *xlsx.Sheet) error {
	adminTasks := make([]ClinicianPlan, 0)
	nonAdminTasks := make([]ClinicianPlan, 0)
	membersDowngraded := r.plan.ClinicianPlans.GetDowngradedMembersCount()

	for _, plan := range r.plan.ClinicianPlans {
		if plan.ClinicianAction == ClinicianActionMergeInto {
			// Results will be reported by the corresponding source merge task
			continue
		}
		if slices.Contains(plan.ResultingRoles, clinicians.RoleClinicAdmin) {
			adminTasks = append(adminTasks, plan)
		} else {
			nonAdminTasks = append(nonAdminTasks, plan)
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

	resultingTagsCount := r.plan.TagsPlans.GetResultingTagsCount()
	duplicateTagsCount := r.plan.TagsPlans.GetDuplicateTagsCount()

	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Resulting Tags")
	currentRow.AddCell().SetValue(resultingTagsCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate tags that will be merged")
	currentRow.AddCell().SetValue(duplicateTagsCount)

	resultingSitesCount := r.plan.SitesPlans.GetResultingSitesCount()
	renamedSitesCount := r.plan.SitesPlans.GetRenamedSitesCount()
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("Resulting Sites")
	currentRow.AddCell().SetValue(resultingSitesCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate sites that will be renamed")
	currentRow.AddCell().SetValue(renamedSitesCount)

	resultingPatientsCount := r.plan.PatientPlans.GetResultingPatientsCount()
	duplicateAccountsCounts := r.plan.PatientPlans.GetConflictCounts()[PatientConflictCategoryDuplicateAccounts]
	likelyDuplicateCount := r.plan.PatientPlans.GetConflictCounts()[PatientConflictCategoryLikelyDuplicateAccounts]
	duplicateMRNsCount := r.plan.PatientPlans.GetConflictCounts()[PatientConflictCategoryMRNOnlyMatch]
	duplicateNamesCount := r.plan.PatientPlans.GetConflictCounts()[PatientConflictCategoryNameOnlyMatch]

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
	currentRow.AddCell().SetValue("- Duplicate MRN Only")
	currentRow.AddCell().SetValue(duplicateMRNsCount)
	currentRow = sh.AddRow()
	currentRow.AddCell().SetValue("- Duplicate Name Only")
	currentRow.AddCell().SetValue(duplicateNamesCount)
	sh.AddRow()

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
	currentRow.AddCell().SetValue("Sites ---")
	currentRow.AddCell().SetValue("Latest Upload ---")
	currentRow.AddCell().SetValue("Likely Duplicates ---")
	currentRow.AddCell().SetValue("Name Only Matches ---")
	currentRow.AddCell().SetValue("MRN Only Matches ---")

	tags := buildTagsMap(clinic.PatientTags)
	count := 1
	for _, cluster := range clusters {
		currentRow = sh.AddRow()
		currentRow.AddCell().SetValue("Review " + strconv.Itoa(count))

		count += 1
		for _, p := range cluster.Patients {
			currentRow = sh.AddRow()
			currentRow.AddCell()

			addPatientDetails(currentRow, p.Patient, getUniquePatientTagNames(p.Patient, tags), clinic.Sites) // TODO(EAW): not sure clinic.Sitesxo is correct

			currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryLikelyDuplicateAccounts], ", "))
			currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryNameOnlyMatch], ", "))
			currentRow.AddCell().SetValue(strings.Join(p.Conflicts[PatientConflictCategoryMRNOnlyMatch], ", "))
		}

		sh.AddRow()
	}
}

func addPatientDetails(row *xlsx.Row, patient patients.Patient, tags []string, sites []sites.Site) {
	row.AddCell().SetValue(pointer.ToString(patient.FullName))
	if !patient.IsCustodial() {
		row.AddCell().SetValue("Y")
	} else {
		row.AddCell().SetValue("-")
	}
	row.AddCell().SetValue(pointer.ToString(patient.UserId))
	row.AddCell().SetValue(pointer.ToString(patient.BirthDate))
	row.AddCell().SetValue(pointer.ToString(patient.Mrn))
	row.AddCell().SetValue(strings.Join(tags, ", "))
	siteNames := []string{}
	for _, site := range patient.Sites {
		renamed, err := maybeRenameSite(site, sites)
		if err != nil {
			continue
		}
		siteNames = append(siteNames, renamed)
	}
	slices.SortFunc(siteNames, func(i, j string) int {
		return cmp.Compare(strings.ToLower(i), strings.ToLower(j))
	})
	row.AddCell().SetValue(strings.Join(siteNames, ", "))
	if patient.Summary != nil && !patient.Summary.GetLastUploadDate().IsZero() {
		row.AddCell().SetValue(patient.Summary.GetLastUploadDate().Format(LastUploadTimeFormat))
	} else {
		row.AddCell()
	}
}
