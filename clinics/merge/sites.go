package merge

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
)

type SiteAction string

const (
	SiteActionInvalid SiteAction = ""
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

	for _, tgtSite := range s.target.Sites {
		if tgtSite.Name == s.site.Name {
			plan.Action = SiteActionRename
			newName, err := sites.MaybeRenameSite(plan.Site, s.target.Sites)
			if err != nil {
				return SitePlan{}, err
			}
			plan.ExpectedRename = newName
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
	return SitePlan{
		Site:            t.site,
		Action:          SiteActionRetain,
		SourceWorkspace: *t.target.Name,
		TargetClinicId:  t.target.Id,
	}, nil
}

type SitePlanExecutor struct {
	logger          *zap.SugaredLogger
	clinicsService  clinics.Service
	patientsService patients.Service
}

func NewSitePlanExecutor(logger *zap.SugaredLogger, clinicsService clinics.Service, patientsService patients.Service) *SitePlanExecutor {
	return &SitePlanExecutor{
		logger:          logger,
		clinicsService:  clinicsService,
		patientsService: patientsService,
	}
}

func (t *SitePlanExecutor) Execute(ctx context.Context, plan SitePlan) error {
	logger := t.logger.With("site", plan.Site)
	switch plan.Action {
	case SiteActionRetain:
		logger.Debug("retaining existing target site")
	case SiteActionMove:
		_, err := t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			if errors.Is(err, clinics.ErrMaximumSitesExceeded) {
				if err := t.patientsService.DeleteSites(ctx, plan.SourceClinicId.Hex(),
					plan.Site.Id.Hex()); err != nil {
					logger.Warnw("unable to delete source patient site", "error", err)
					return err
				}
				msg := fmt.Sprintf("clinic site creation failed: %s, deleted", err)
				logger.Warnw(msg, "site name", plan.Site.Name)
			} else {
				return err
			}
		} else {
			logger.Debug("created new target site")
		}
	case SiteActionRename:
		targetClinic, err := t.clinicsService.Get(ctx, plan.TargetClinicId.Hex())
		if err != nil {
			return fmt.Errorf("unable to get target clinic for site %s: %s", plan.Name(), err)
		}
		targetSites := targetClinic.Sites
		newName, err := sites.MaybeRenameSite(plan.Site, targetSites)
		if err != nil {
			return err
		}
		if newName == plan.Site.Name {
			logger.Debug("a site marked RENAME didn't need a rename; strange")
			return nil
		}
		prevName := plan.Site.Name
		plan.Site.Name = newName
		_, err = t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}

		err = t.patientsService.UpdateSites(ctx, plan.SourceClinicId.Hex(),
			plan.Site.Id.Hex(), &plan.Site)
		if err != nil {
			return err
		}

		logger.Debugw("renamed source site", "from", prevName, "to", plan.Site.Name)
	default:
		return fmt.Errorf("unhandled site action for site %q: %s", plan.Name(), plan.Action)
	}
	return nil
}
