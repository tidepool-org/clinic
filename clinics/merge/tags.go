package merge

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinics"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
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

	SourceClinicId *primitive.ObjectID
	TargetClinicId *primitive.ObjectID
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
		Name:           t.tag.Name,
		Workspaces:     []string{*t.source.Name},
		TagAction:      TagActionCreate,
		SourceClinicId: t.source.Id,
		TargetClinicId: t.target.Id,
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
		Name:           t.tag.Name,
		Workspaces:     []string{*t.target.Name},
		TagAction:      TagActionRetain,
		TargetClinicId: t.target.Id,
	}

	for _, tt := range t.source.PatientTags {
		if tt.Name == t.tag.Name {
			plan.Workspaces = append(plan.Workspaces, *t.source.Name)
			plan.SourceClinicId = t.source.Id
			sort.Strings(plan.Workspaces)
			break
		}
	}
	if len(plan.Workspaces) > 1 {
		plan.Merge = true
	}

	return plan, nil
}

type TagPlanExecutor struct {
	logger         *zap.SugaredLogger
	clinicsService clinics.Service
}

func NewTagPlanExecutor(logger *zap.SugaredLogger, clinicsService clinics.Service) *TagPlanExecutor {
	return &TagPlanExecutor{
		logger: logger,
		clinicsService: clinicsService,
	}
}

func (t *TagPlanExecutor) Execute(ctx context.Context, plan TagPlan) error {
	if plan.TagAction == TagActionSkip || plan.TagAction == TagActionRetain {
		t.logger.Debugw("skipping tag", "plan", plan)
		return nil
	} else if plan.TagAction == TagActionCreate {
		_, err := t.clinicsService.CreatePatientTag(ctx, plan.TargetClinicId.Hex(), plan.Name)
		return err
	} else {
		return fmt.Errorf("unexpected tag plan action %v for tag %s", plan.TagAction, plan.Name)
	}
}
