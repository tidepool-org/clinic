package merge

import (
	mapset "github.com/deckarep/golang-set/v2"
)

type ReportError struct {
	Message string
}

func NewReportError(message string) ReportError {
	return ReportError{Message: message}
}

var (
	ErrorDuplicateMRNInSourceWorkspace             = NewReportError("MRN uniqueness error(s) for duplicate accounts. View error(s) on the 'Duplicates in Source Clinic' tab")
	ErrorDuplicateMRNInTargetWorkspace             = NewReportError("MRN uniqueness error(s) for duplicate accounts. View error(s) on the 'Duplicates in Merged Workspace' tab")
	ErrorMRNRequiredInTargetWorkspace              = NewReportError("Target workspace requires MRNs")
	ErrorCannotMergeWorkspaceWithPendingInvites    = NewReportError("Pending invites in source workspace")
	ErrorPartialSSOSettingsPreventMovingClinicians = NewReportError("The partial SSO settings of the target clinic prevent moving source clinicians to target clinic. All source members which are not already members of the target clinic should be removed from the source clinic.")
	ErrorWorkspaceSettingsMismatch                 = NewReportError("Settings mismatch")
)

func GetUniqueErrorMessages(errs []ReportError) []string {
	set := mapset.NewSet[string]()
	for _, r := range errs {
		set.Add(r.Message)
	}
	return set.ToSlice()
}
