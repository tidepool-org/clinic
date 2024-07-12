package merge

import (
	"context"
	"github.com/tidepool-org/clinic/clinics"
	"sort"
)

const (
	TagActionSkip   = "SKIP"
	TagActionCreate = "CREATE"
	TagActionRetain = "RETAIN"
)

type TagPlan struct {
	Name       string
	TagAction  string
	Workspaces []string
	Merge      bool
}

func (t TagPlan) PreventsMerge() bool {
	return false
}

type TagPlans []TagPlan

func (t TagPlans) PreventsMerge() bool {
	return PlansPreventMerge(t)
}

func (t TagPlans) GetResultingTagsCount() int {
	count := 0
	for _, p := range t {
		if p.TagAction == TagActionCreate || p.TagAction == TagActionRetain {
			count++
		}
	}
	return count
}

func (t TagPlans) GetDuplicateTagsCount() int {
	count := 0
	for _, p := range t {
		if p.TagAction == TagActionSkip {
			count++
		}
	}
	return count
}

type SourceTagMergePlanner struct {
	tag clinics.PatientTag

	source clinics.Clinic
	target clinics.Clinic
}

func NewSourceTagMergePlanner(tag clinics.PatientTag, source, target clinics.Clinic) Planner[TagPlan] {
	return &SourceTagMergePlanner{
		tag:    tag,
		source: source,
		target: target,
	}
}

func (t *SourceTagMergePlanner) Plan(ctx context.Context) (TagPlan, error) {
	plan := TagPlan{
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
			break
		}
	}

	return plan, nil
}

type TargetTagMergePlanner struct {
	tag clinics.PatientTag

	source clinics.Clinic
	target clinics.Clinic
}

func NewTargetTagMergePlanner(tag clinics.PatientTag, source, target clinics.Clinic) Planner[TagPlan] {
	return &TargetTagMergePlanner{
		tag:    tag,
		source: source,
		target: target,
	}
}

func (t *TargetTagMergePlanner) Plan(ctx context.Context) (TagPlan, error) {
	plan := TagPlan{
		Name:       t.tag.Name,
		Workspaces: []string{*t.target.Name},
		TagAction:  TagActionRetain,
	}

	for _, tt := range t.source.PatientTags {
		if tt.Name == t.tag.Name {
			plan.Workspaces = append(plan.Workspaces, *t.source.Name)
			sort.Strings(plan.Workspaces)
			break
		}
	}
	if len(plan.Workspaces) > 1 {
		plan.Merge = true
	}

	return plan, nil
}
