package merge_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/sites"
	sitesTest "github.com/tidepool-org/clinic/sites/test"
)

var _ = Describe("Sites", func() {
	Describe("Sites Planners", func() {
		It("creates correct plans", func() {
			Expect(true).To(BeTrue())
			_, th := newSitesTestHelper(GinkgoT())
			th.createDuplicateSites(2)
			th.createSourceSites(3)
			th.createTargetSites(3)

			plans := th.Plans()

			Expect(len(plans)).To(Equal(len(th.Source.Sites) + len(th.Target.Sites)))
			for _, plan := range plans {
				Expect(plan.Name()).To(Equal(plan.Site.Name))
				Expect(plan.PreventsMerge()).To(Equal(false))
				if th.isSourceSite(plan.Site) {
					if th.isDupSite(plan.Site) {
						Expect(plan.SiteAction).To(Equal(merge.SiteActionRename))
					} else {
						Expect(plan.SiteAction).To(Equal(merge.SiteActionCreate))
					}

				} else if th.isTargetSite(plan.Site) {
					Expect(plan.SiteAction).To(Equal(merge.SiteActionRetain))
				} else {
					Fail(fmt.Sprintf("unhandled action for site: %s", plan.Site))
				}
			}
		})
	})

	Describe("Site Plan Executor", func() {
		It("creates sites in the target clinic that don't exist", func() {
			ctx, th := newSitesTestHelper(GinkgoT())
			srcSite := th.createSourceSites(1)[0]
			targetID := th.Target.Id.Hex()

			executor := merge.NewSitePlanExecutor(th.Logger, th.Clinics)

			th.Clinics.EXPECT().CreateSite(gomock.Any(), targetID, &srcSite)
			for _, plan := range th.Plans() {
				Expect(executor.Execute(ctx, plan)).To(Succeed(), plan.Name())
			}
		})

		It("renames sites from the source clinic that already exist", func() {
			ctx, th := newSitesTestHelper(GinkgoT())
			targetSite := th.createDuplicateSites(1)[0]
			targetID := th.Target.Id.Hex()

			executor := merge.NewSitePlanExecutor(th.Logger, th.Clinics)

			th.Clinics.EXPECT().ListSites(gomock.Any(), targetID).
				Return(th.Target.Sites, nil).AnyTimes()
			th.Clinics.EXPECT().CreateSite(gomock.Any(), targetID, incrementedSiteMatcher(targetSite))
			for _, plan := range th.Plans() {
				Expect(executor.Execute(ctx, plan)).To(Succeed(), plan.Name())
			}
		})

		It("renames sites multiple times if necessary", func() {
			ctx, th := newSitesTestHelper(GinkgoT())
			targetSite := th.createDuplicateSites(1)[0]
			times := rand.IntN(20)
			for i := range times {
				th.createTargetSite(fmt.Sprintf("%s (%d)", targetSite.Name, i+2))
			}
			targetID := th.Target.Id.Hex()

			executor := merge.NewSitePlanExecutor(th.Logger, th.Clinics)

			th.Clinics.EXPECT().ListSites(gomock.Any(), targetID).
				Return(th.Target.Sites, nil).AnyTimes()
			th.Clinics.EXPECT().CreateSite(gomock.Any(), targetID, incrementedSiteMatcherN(targetSite, times+2))
			for _, plan := range th.Plans() {
				Expect(executor.Execute(ctx, plan)).To(Succeed(), plan.Name())
			}
		})
	})
})

type sitesTestHelper struct {
	Source  *clinics.Clinic
	Target  *clinics.Clinic
	Clinics *clinicsTest.MockService
	Logger  *zap.SugaredLogger

	t FullGinkgoTInterface
}

func newSitesTestHelper(t FullGinkgoTInterface) (context.Context, *sitesTestHelper) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(GinkgoWriter), zapcore.DebugLevel)
	logger := zap.New(core).Sugar()
	return ctx, &sitesTestHelper{
		Source:  clinicsTest.RandomClinic(),
		Target:  clinicsTest.RandomClinic(),
		Clinics: clinicsTest.NewMockService(ctrl),
		Logger:  logger,
		t:       t,
	}
}

func (s *sitesTestHelper) Plans() []merge.SitePlan {
	ctx := context.Background()
	plans := []merge.SitePlan{}
	for _, site := range s.Source.Sites {
		plan, err := merge.NewSourceSiteMergePlanner(site, *s.Source, *s.Target).Plan(ctx)
		if err != nil {
			s.t.Fatalf("failed to plan source plan for site %q: %s", site.Name, err)
		}
		plans = append(plans, plan)
	}
	for _, site := range s.Target.Sites {
		plan, err := merge.NewTargetSiteMergePlanner(site, *s.Source, *s.Target).Plan(ctx)
		if err != nil {
			s.t.Fatalf("failed to plan target plan for site %q: %s", site.Name, err)
		}
		plans = append(plans, plan)
	}
	return plans
}

func (s *sitesTestHelper) createTargetSite(name string) sites.Site {
	if sites.SiteExistsWithName(s.Target.Sites, name) {
		s.t.Fatalf("site with name %s already exists", name)
	}
	site := sites.Site{
		Id:   primitive.NewObjectID(),
		Name: name,
	}
	s.Target.Sites = append(s.Target.Sites, site)
	return site
}

func (s *sitesTestHelper) createDuplicateSites(n int) []sites.Site {
	newSites := []sites.Site{}
	for len(newSites) < n {
		site := sitesTest.Random()
		if !s.siteExists(site) {
			newSites = append(newSites, site)
			s.Target.Sites = append(s.Target.Sites, site)
			site.Id = primitive.NewObjectID()
			s.Source.Sites = append(s.Source.Sites, site)
		}
	}
	return newSites
}

func (s *sitesTestHelper) createSourceSites(n int) []sites.Site {
	newSites := []sites.Site{}
	for len(newSites) < n {
		site := sitesTest.Random()
		if !s.siteExists(site) {
			newSites = append(newSites, site)
		}
	}
	s.Source.Sites = slices.Concat(s.Source.Sites, newSites)
	return newSites
}

func (s *sitesTestHelper) createTargetSites(n int) []sites.Site {
	newSites := []sites.Site{}
	for len(newSites) < n {
		site := sitesTest.Random()
		if !s.siteExists(site) {
			newSites = append(newSites, site)
		}
	}
	s.Target.Sites = slices.Concat(s.Target.Sites, newSites)
	return newSites
}

func (s *sitesTestHelper) siteExists(site sites.Site) bool {
	return sites.SiteExistsWithName(s.Target.Sites, site.Name) ||
		sites.SiteExistsWithName(s.Source.Sites, site.Name)
}

func (s *sitesTestHelper) isDupSite(site sites.Site) bool {
	return sites.SiteExistsWithName(s.Source.Sites, site.Name) &&
		sites.SiteExistsWithName(s.Target.Sites, site.Name)
}

func (s *sitesTestHelper) isSourceSite(site sites.Site) bool {
	for _, srcSite := range s.Source.Sites {
		if srcSite.Id.Hex() == site.Id.Hex() {
			return true
		}
	}
	return false
}

func (s *sitesTestHelper) isTargetSite(site sites.Site) bool {
	for _, tgtSite := range s.Target.Sites {
		if tgtSite.Id.Hex() == site.Id.Hex() {
			return true
		}
	}
	return false
}

func incrementedSiteMatcher(toMatch sites.Site) gomock.Matcher {
	return incrementedSiteMatcherN(toMatch, 2)
}

func incrementedSiteMatcherN(toMatch sites.Site, n int) gomock.Matcher {
	return &condSiteMatcher{name: fmt.Sprintf("%s (%d)", toMatch.Name, n)}
}

type condSiteMatcher struct {
	name string
}

func (c *condSiteMatcher) Matches(x any) bool {
	if s, ok := x.(*sites.Site); ok {
		return s.Name == c.name
	}
	return false
}

func (c *condSiteMatcher) String() string {
	return "is a site with name \"" + c.name + "\""
}
