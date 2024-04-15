package merge

import (
	"context"
	"github.com/tidepool-org/clinic/clinics"
	"sort"
)

const (
	TagActionCreate = "CREATE"
	TagActionSkip   = "SKIP"
	TagActionKeep   = "KEEP"
)

type TagReportDetails struct {
	Name       string
	TagAction  string
	Workspaces []string
	Merge      bool
}

type SourceTagMergeTask struct {
	tag clinics.PatientTag

	source clinics.Clinic
	target clinics.Clinic

	result TaskResult[TagReportDetails]
	err    error
}

func NewSourceTagMergeTask(tag clinics.PatientTag, source, target clinics.Clinic) Task[TagReportDetails] {
	return &SourceTagMergeTask{
		tag:    tag,
		source: source,
		target: target,
	}
}

func (t *SourceTagMergeTask) CanRun() bool {
	return true
}

func (t *SourceTagMergeTask) DryRun(ctx context.Context) error {
	t.result = TaskResult[TagReportDetails]{
		ReportDetails: TagReportDetails{
			Name:       t.tag.Name,
			Workspaces: []string{*t.source.Name},
			TagAction:  TagActionCreate,
		},
		PreventsMerge: !t.CanRun(),
	}

	for _, tt := range t.target.PatientTags {
		if tt.Name == t.tag.Name {
			// Tag already exist in target workspace, do nothing
			t.result.ReportDetails.TagAction = TagActionSkip
			t.result.ReportDetails.Workspaces = append(t.result.ReportDetails.Workspaces, *t.target.Name)
			sort.Strings(t.result.ReportDetails.Workspaces)
		}
	}

	return nil
}

func (t *SourceTagMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (t *SourceTagMergeTask) GetResult() (TaskResult[TagReportDetails], error) {
	return t.result, t.err
}

type TargetTagMergeTask struct {
	tag clinics.PatientTag

	source clinics.Clinic
	target clinics.Clinic

	result TaskResult[TagReportDetails]
	err    error
}

func NewTargetTagMergeTask(tag clinics.PatientTag, source, target clinics.Clinic) Task[TagReportDetails] {
	return &TargetTagMergeTask{
		tag:    tag,
		source: source,
		target: target,
	}
}

func (t *TargetTagMergeTask) CanRun() bool {
	return true
}

func (t *TargetTagMergeTask) DryRun(ctx context.Context) error {
	t.result = TaskResult[TagReportDetails]{
		ReportDetails: TagReportDetails{
			Name:       t.tag.Name,
			Workspaces: []string{*t.target.Name},
			TagAction:  TagActionKeep,
		},
		PreventsMerge: !t.CanRun(),
	}

	for _, tt := range t.source.PatientTags {
		if tt.Name == t.tag.Name {
			t.result.ReportDetails.Workspaces = append(t.result.ReportDetails.Workspaces, *t.source.Name)
			sort.Strings(t.result.ReportDetails.Workspaces)
		}
	}
	if len(t.result.ReportDetails.Workspaces) > 1 {
		t.result.ReportDetails.Merge = true
	}

	return nil
}

func (t *TargetTagMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (t *TargetTagMergeTask) GetResult() (TaskResult[TagReportDetails], error) {
	return t.result, t.err
}
