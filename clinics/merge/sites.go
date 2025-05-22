package merge

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"

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
	SiteActionRename SiteAction = "RENAME"
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
			plan.SiteAction = SiteActionRename
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
	switch plan.SiteAction {
	case SiteActionRetain, SiteActionSkip:
		t.logger.Debugw("skipping site", "plan", plan)
	case SiteActionCreate:
		t.logger.Debugw("creating target site", "plan", plan.Name(), "site", plan.Site)
		err := t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}
	case SiteActionRename:
		targetSites, err := t.clinicsService.ListSites(ctx, plan.TargetClinicId.Hex())
		if err != nil {
			return fmt.Errorf("unable to list target clinic sites for site %s: %s", plan.Name(), err)
		}
		proposedName := plan.Site.Name
		for sites.SiteExistsWithName(targetSites, proposedName) {
			incremented, err := incNumericSuffix(plan.Site)
			if err != nil {
				return err
			}
			proposedName = incremented
		}
		if proposedName == plan.Site.Name {
			t.logger.Debugw("a site marked RENAME didn't need it; strange", "plan", plan)
			return nil
		}
		plan.Site.Name = proposedName
		t.logger.Debugw("creating target site", "plan", plan.Name(), "site", plan.Site)
		err = t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected site plan action %v for site %s", plan.SiteAction, plan.Name())
	}
	return nil
}

var siteNameSuffix = regexp.MustCompile(` \((\d+)\)$`)

func incNumericSuffix(s sites.Site) (string, error) {
	matches := siteNameSuffix.FindStringSubmatch(s.Name)
	if len(matches) != 2 {
		// It has no numeric suffix, so add " (2)".
		return s.Name + " (2)", nil
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		// This can only happen if siteNameSuffix is faulty.
		return "", fmt.Errorf("highly strange error in incNumericSuffix")
	}
	base := s.Name[:len(s.Name)-len(matches[1])]
	return fmt.Sprintf("%s (%d)", base, n+1), nil
}
