package merge

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/sites"
)

type SiteAction string

const (
	SiteActionSkip   SiteAction = "SKIP"
	SiteActionCreate SiteAction = "CREATE"
	SiteActionRetain SiteAction = "RETAIN"
)

type SitePlan struct {
	Site       sites.Site `bson:"site"`
	SiteAction SiteAction `bson:"siteAction"`
	Workspaces []string   `bson:"workspaces"`
	Merge      bool       `bson:"merge"`

	SourceClinicId *primitive.ObjectID `bson:"sourceClinicId"`
	TargetClinicId *primitive.ObjectID `bson:"targetClinicId"`
}

func (t SitePlan) Name() string {
	return t.Site.Name
}

func (t SitePlan) PreventsMerge() bool {
	return false
}

func (t SitePlan) Errors() []ReportError {
	return nil
}

type SitePlans []SitePlan

func (t SitePlans) PreventsMerge() bool {
	return PlansPreventMerge(t)
}

func (t SitePlans) Errors() []ReportError {
	return PlansErrors(t)
}

func (t SitePlans) GetResultingSitesCount() int {
	count := 0
	for _, p := range t {
		if p.SiteAction == SiteActionCreate || p.SiteAction == SiteActionRetain {
			count++
		}
	}
	return count
}

func (t SitePlans) GetDuplicateSitesCount() int {
	count := 0
	for _, p := range t {
		if p.SiteAction == SiteActionSkip {
			count++
		}
	}
	return count
}

type SourceSiteMergePlanner struct {
	site sites.Site

	source clinics.Clinic
	target clinics.Clinic
}

func NewSourceSiteMergePlanner(site sites.Site, source, target clinics.Clinic) Planner[SitePlan] {
	return &SourceSiteMergePlanner{
		site:   site,
		source: source,
		target: target,
	}
}

func (t *SourceSiteMergePlanner) Plan(ctx context.Context) (SitePlan, error) {
	plan := SitePlan{
		Site:           t.site,
		Workspaces:     []string{*t.source.Name},
		SiteAction:     SiteActionCreate,
		SourceClinicId: t.source.Id,
		TargetClinicId: t.target.Id,
	}

	for _, targetSite := range t.target.Sites {
		if targetSite.Name == t.site.Name {
			// Site already exists in target workspace, do nothing
			plan.SiteAction = SiteActionSkip
			plan.Workspaces = append(plan.Workspaces, *t.target.Name)
			sort.Strings(plan.Workspaces)
			break
		}
	}

	return plan, nil
}

type TargetSiteMergePlanner struct {
	site sites.Site

	source clinics.Clinic
	target clinics.Clinic
}

func NewTargetSiteMergePlanner(site sites.Site, source, target clinics.Clinic) Planner[SitePlan] {
	return &TargetSiteMergePlanner{
		site:   site,
		source: source,
		target: target,
	}
}

func (t *TargetSiteMergePlanner) Plan(ctx context.Context) (SitePlan, error) {
	plan := SitePlan{
		Site:           t.site,
		Workspaces:     []string{*t.target.Name},
		SiteAction:     SiteActionRetain,
		TargetClinicId: t.target.Id,
	}

	for _, sourceSite := range t.source.Sites {
		if sourceSite.Name == t.site.Name {
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

type SitePlanExecutor struct {
	logger         *zap.SugaredLogger
	clinicsService clinics.Service
}

func NewSitePlanExecutor(logger *zap.SugaredLogger, clinicsService clinics.Service) *SitePlanExecutor {
	return &SitePlanExecutor{
		logger:         logger,
		clinicsService: clinicsService,
	}
}

func (t *SitePlanExecutor) Execute(ctx context.Context, plan SitePlan) error {
	if plan.SiteAction == SiteActionSkip || plan.SiteAction == SiteActionRetain {
		t.logger.Debugw("skipping site", "plan", plan)
		return nil
	} else if plan.SiteAction == SiteActionCreate {
		err := t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}
		return nil
	} else {
		return fmt.Errorf("unexpected site plan action %v for site %s", plan.SiteAction, plan.Name())
	}
}
