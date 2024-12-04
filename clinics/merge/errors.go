package merge

import (
	"errors"
	mapset "github.com/deckarep/golang-set/v2"
)

type Error error

var (
	ErrorDuplicateMRNInTargetWorkspace          Error = errors.New("MRN uniqueness error(s) for duplicate accounts. View error(s) on the 'Duplicates in Merged Workspace' tab")
	ErrorMRNRequiredInTargetWorkspace           Error = errors.New("Target workspace requires MRNs")
	ErrorCannotMergeWorkspaceWithPendingInvites Error = errors.New("Pending invites is source workspace")
	ErrorWorkspaceSettingsMismatch              Error = errors.New("Settings mismatch")
)

func GetUniqueErrorMessages(errs []Error) []string {
	set := mapset.NewSet[string]()
	for _, r := range errs {
		set.Add(r.Error())
	}
	return set.ToSlice()
}
