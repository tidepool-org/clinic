package merge

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/sites"
)

type SiteAction string

const (
	SiteActionInvalid SiteAction = ""
	// SiteActionCreate indicates that the target clinic will have a new site created.
	SiteActionCreate SiteAction = "CREATE"
	// SiteActionRetain indicates that the target clinic will keep it's current site.
	SiteActionRetain SiteAction = "RETAIN"
	// SiteActionRename indicates that the source clinic's site will be renamed when created
	// in the target clinic.
	SiteActionRename SiteAction = "RENAME"
)

type SitePlan struct {
	Site       sites.Site `bson:"site"`
	SiteAction SiteAction `bson:"siteAction"`

	SourceClinicId *primitive.ObjectID `bson:"sourceClinicId"`
	TargetClinicId *primitive.ObjectID `bson:"targetClinicId"`
}

func (s SitePlan) Name() string {
	return s.Site.Name
}

// PreventsMerge implements [Plan].
func (s SitePlan) PreventsMerge() bool {
	return s.SiteAction == SiteActionInvalid
}

// Errors implements [Plan].
func (s SitePlan) Errors() []ReportError {
	if s.SiteAction == SiteActionInvalid {
		return []ReportError{
			{Message: "invalid site action for site: " + s.Name()},
		}
	}
	return nil
}

// SitePlans aggregates multiple SitePlans while implementing [Plan].
type SitePlans []SitePlan

// PreventsMerge implements [Plan].
func (s SitePlans) PreventsMerge() bool {
	return PlansPreventMerge(s)
}

// Errors implements [Plan].
func (s SitePlans) Errors() []ReportError {
	return PlansErrors(s)
}

// GetResultingSitesCount is the number of sites expected to be present in the target clinic
// after a merge.
func (s SitePlans) GetResultingSitesCount() int {
	count := 0
	for _, p := range s {
		if p.SiteAction == SiteActionCreate || p.SiteAction == SiteActionRetain {
			count++
		}
	}
	return count
}

// GetRenamedSitesCount is the number of sites from the source clinic that will be renamed
// during a merge.
func (s SitePlans) GetRenamedSitesCount() int {
	count := 0
	for _, site := range s {
		if site.SiteAction == SiteActionRename {
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
		Site:           s.site,
		SiteAction:     SiteActionCreate,
		SourceClinicId: s.source.Id,
		TargetClinicId: s.target.Id,
	}

	for _, tgtSite := range s.target.Sites {
		if tgtSite.Name == s.site.Name {
			plan.SiteAction = SiteActionRename
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
		Site:           t.site,
		SiteAction:     SiteActionRetain,
		TargetClinicId: t.target.Id,
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
	//logger := t.logger.With("plan", plan, "site", plan.Site)
	logger := t.logger.With("site", plan.Site)
	switch plan.SiteAction {
	case SiteActionRetain:
		logger.Debug("retaining existing target site")
	case SiteActionCreate:
		err := t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}
		logger.Debug("creating new target site")
	case SiteActionRename:
		targetSites, err := t.clinicsService.ListSites(ctx, plan.TargetClinicId.Hex())
		if err != nil {
			return fmt.Errorf("unable to list target clinic sites for site %s: %s", plan.Name(), err)
		}
		proposedName := plan.Site.Name
		for sites.SiteExistsWithName(targetSites, proposedName) {
			incremented, err := incNumericSuffix(proposedName)
			if err != nil {
				return err
			}
			proposedName = incremented
		}
		if proposedName == plan.Site.Name {
			logger.Debug("a site marked RENAME didn't need it; strange")
			return nil
		}
		prevName := plan.Site.Name
		plan.Site.Name = proposedName
		err = t.clinicsService.CreateSite(ctx, plan.TargetClinicId.Hex(), &plan.Site)
		if err != nil {
			return err
		}
		logger.Debugw("renaming source site to target site", "from", prevName, "to", plan.Site.Name)
	default:
		return fmt.Errorf("invalid site action for site %s", plan.Name())
	}
	return nil
}

var siteNameSuffix = regexp.MustCompile(` \((\d+)\)$`)

// func incNumericSuffix(s sites.Site) (string, error) {
// 	matches := siteNameSuffix.FindStringSubmatch(s.Name)
// 	if len(matches) != 2 {
// 		// It has no numeric suffix, so add " (2)".
// 		return s.Name + " (2)", nil
// 	}
// 	n, err := strconv.Atoi(matches[1])
// 	if err != nil {
// 		// This can only happen if siteNameSuffix is faulty.
// 		return "", fmt.Errorf("highly strange error in incNumericSuffix")
// 	}
// 	base := s.Name[:len(s.Name)-len(matches[1])]
// 	return fmt.Sprintf("%s (%d)", base, n+1), nil
// }

func incNumericSuffix(name string) (string, error) {
	matches := siteNameSuffix.FindStringSubmatch(name)
	if len(matches) != 2 {
		// It has no numeric suffix, so add " (2)".
		return name + " (2)", nil
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		// This can only happen if siteNameSuffix is faulty.
		return "", fmt.Errorf("highly strange error in incNumericSuffix")
	}
	base := name[:len(name)-len(matches[0])]
	return fmt.Sprintf("%s (%d)", base, n+1), nil
}
