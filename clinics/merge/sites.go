package merge

import (
	"context"
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/sites"
)

type SiteAction string

const (
	SiteActionInvalid SiteAction = ""
	// SiteActionCreate the site in the target clinic, for the source patient site option.
	SiteActionCreate SiteAction = "CREATE"
	// SiteActionMove the site from the source clinic to the target clinic.
	SiteActionMove SiteAction = "MOVE"
	// SiteActionRetain the site in the target clinic.
	SiteActionRetain SiteAction = "RETAIN"
	// SiteActionRename the source clinic before moving it to the target clinic.
	SiteActionRename SiteAction = "RENAME"
)

type SitePlan struct {
	Site            sites.Site `bson:"site"`
	Action          SiteAction `bson:"siteAction"`
	SourceWorkspace string     `bson:"workspace"`
	ExpectedRename  string     `bson:"expectedRename"`

	SourceClinicId *primitive.ObjectID `bson:"sourceClinicId"`
	TargetClinicId *primitive.ObjectID `bson:"targetClinicId"`
}

func (s SitePlan) Name() string {
	if s.ExpectedRename != "" {
		return s.ExpectedRename
	}
	return s.Site.Name
}

// PreventsMerge implements [Plan].
func (s SitePlan) PreventsMerge() bool {
	return s.Action == SiteActionInvalid
}

// Errors implements [Plan].
func (s SitePlan) Errors() []ReportError {
	if s.Action == SiteActionInvalid {
		return []ReportError{
			{Message: "invalid site action for site: " + s.Name()},
		}
	}
	return nil
}

// SitesPlans aggregates multiple SitesPlans while implementing [Plan].
type SitesPlans []SitePlan

// PreventsMerge implements [Plan].
func (s SitesPlans) PreventsMerge() bool {
	return PlansPreventMerge(s)
}

// Errors implements [Plan].
func (s SitesPlans) Errors() []ReportError {
	return PlansErrors(s)
}

// GetResultingSitesCount is the number of sites expected to be present in the target clinic
// after a merge.
func (s SitesPlans) GetResultingSitesCount() int {
	return len(s)
}

// GetRenamedSitesCount is the number of sites from the source clinic that will be renamed
// during a merge.
func (s SitesPlans) GetRenamedSitesCount() int {
	count := 0
	for _, site := range s {
		if site.Action == SiteActionRename {
			count++
		}
	}
	return count
}

// sourceSiteNames maps each source site's id to its name in the target clinic after a merge.
// Site names are unique within a clinic, so a source site that has a target site's name is
// renamed to one that no other site in the merged clinic has.
func sourceSiteNames(sourceSites, targetSites []sites.Site) (map[primitive.ObjectID]string, error) {
	reserved := slices.Clone(targetSites)
	for _, site := range sourceSites {
		if !sites.SiteExistsWithName(targetSites, site.Name) {
			reserved = append(reserved, site)
		}
	}
	names := make(map[primitive.ObjectID]string, len(sourceSites))
	for _, site := range sourceSites {
		name, err := sites.MaybeRenameSite(site, reserved)
		if err != nil {
			return nil, err
		}
		if name != site.Name {
			reserved = append(reserved, sites.Site{Name: name})
		}
		names[site.Id] = name
	}
	return names, nil
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

func (s *SourceSiteMergePlanner) Plan(ctx context.Context) (SitePlan, error) {
	plan := SitePlan{
		Site:            s.site,
		Action:          SiteActionMove,
		SourceWorkspace: *s.source.Name,
		SourceClinicId:  s.source.Id,
		TargetClinicId:  s.target.Id,
	}

	if sites.SiteExistsWithName(s.target.Sites, s.site.Name) {
		names, err := sourceSiteNames(s.source.Sites, s.target.Sites)
		if err != nil {
			return SitePlan{}, err
		}
		plan.Action = SiteActionRename
		plan.ExpectedRename = names[s.site.Id]
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
	return SitePlan{
		Site:            t.site,
		Action:          SiteActionRetain,
		SourceWorkspace: *t.target.Name,
		TargetClinicId:  t.target.Id,
	}, nil
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
	logger := t.logger.With("site", plan.Site)
	switch plan.Action {
	case SiteActionCreate:
		_, err := t.clinicsService.CreateSiteIgnoringLimit(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		return err
	case SiteActionRetain:
		logger.Debug("retaining existing target site")
	case SiteActionMove:
		_, err := t.clinicsService.CreateSiteIgnoringLimit(ctx,
			plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}
		logger.Debug("created new target site")
	case SiteActionRename:
		prevName := plan.Site.Name
		plan.Site.Name = plan.ExpectedRename
		// Source patients pick up the new name when they are moved, after all sites
		_, err := t.clinicsService.CreateSiteIgnoringLimit(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}

		logger.Debugw("renamed source site", "from", prevName, "to", plan.Site.Name)
	default:
		return fmt.Errorf("unhandled site action for site %q: %s", plan.Name(), plan.Action)
	}
	return nil
}
