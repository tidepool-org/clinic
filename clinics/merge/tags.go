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

type TagsPlan struct {
	Name       string
	TagAction  string
	Workspaces []string
	Merge      bool
}

func (t TagsPlan) PreventsMerge() bool {
	return false
}

type SourceTagMergePlanner struct {
	tag clinics.PatientTag

	source clinics.Clinic
	target clinics.Clinic
}

func NewSourceTagMergePlanner(tag clinics.PatientTag, source, target clinics.Clinic) Planner[TagsPlan] {
	return &SourceTagMergePlanner{
		tag:    tag,
		source: source,
		target: target,
	}
}

func (t *SourceTagMergePlanner) CanRun() bool {
	return true
}

func (t *SourceTagMergePlanner) Plan(ctx context.Context) (TagsPlan, error) {
	plan := TagsPlan{
		Name:       t.tag.Name,
		Workspaces: []string{*t.source.Name},
		TagAction:  TagActionCreate,
	}

	for _, tt := range t.target.PatientTags {
		if tt.Name == t.tag.Name {
			// Tag already exist in target workspace, do nothing
			plan.TagAction = TagActionSkip
			plan.Workspaces = append(plan.Workspaces, *t.target.Name)
			sort.Strings(plan.Workspaces)
		}
	}

	return plan, nil
}

type TargetTagMergePlanner struct {
	tag clinics.PatientTag

	source clinics.Clinic
	target clinics.Clinic
}

func NewTargetTagMergePlanner(tag clinics.PatientTag, source, target clinics.Clinic) Planner[TagsPlan] {
	return &TargetTagMergePlanner{
		tag:    tag,
		source: source,
		target: target,
	}
}

func (t *TargetTagMergePlanner) Plan(ctx context.Context) (TagsPlan, error) {
	plan := TagsPlan{
		Name:       t.tag.Name,
		Workspaces: []string{*t.target.Name},
		TagAction:  TagActionKeep,
	}

	for _, tt := range t.source.PatientTags {
		if tt.Name == t.tag.Name {
			plan.Workspaces = append(plan.Workspaces, *t.source.Name)
			sort.Strings(plan.Workspaces)
		}
	}
	if len(plan.Workspaces) > 1 {
		plan.Merge = true
	}

	return plan, nil
}
