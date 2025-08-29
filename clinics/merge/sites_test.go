package merge_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	sitesTest "github.com/tidepool-org/clinic/sites/test"
)

var _ = Describe("Sites", func() {
	Describe("Planner", func() {
		It("creates correct plans", func() {
			_, th := newSitesTestHelper(GinkgoT())
			th.createSourceSites("src1")
			th.createTargetSites("tgt1")
			th.createDuplicateSites("dup1", "dup2")
			plans := th.Plans()

			Expect(len(plans)).To(Equal(len(th.Source.Sites) + len(th.Target.Sites)))
			for _, plan := range plans {
				Expect(plan.PreventsMerge()).To(Equal(false))
				if th.isSourceSite(plan.Site) {
					if th.isDupSite(plan.Site) {
						Expect(plan.Name()).To(Equal(plan.Site.Name + " (2)"))
						Expect(plan.Action).To(Equal(merge.SiteActionRename))
					} else {
						Expect(plan.Name()).To(Equal(plan.Site.Name))
						Expect(plan.Action).To(Equal(merge.SiteActionMove))
					}

				} else if th.isTargetSite(plan.Site) {
					Expect(plan.Name()).To(Equal(plan.Site.Name))
					Expect(plan.Action).To(Equal(merge.SiteActionRetain))
				} else {
					Fail(fmt.Sprintf("unhandled action for site: %s", plan.Site))
				}
			}
		})

		It("counts renamed sites", func() {
			_, th := newSitesTestHelper(GinkgoT())
			th.createSourceSites("src1")
			th.createTargetSites("tgt1")
			dups := th.createDuplicateSites("dup1", "dup2")

			plans := merge.SitesPlans(th.Plans())
			Expect(plans.GetRenamedSitesCount()).To(Equal(len(dups)))
		})

		It("counts resulting sites", func() {
			_, th := newSitesTestHelper(GinkgoT())
			src := th.createSourceSites("src1")
			tgt := th.createTargetSites("tgt1", "tgt2", "tgt3")
			dups := th.createDuplicateSites("dup1", "dup2")
			numSitesTotal := (2 * len(dups)) + len(src) + len(tgt)
			plans := merge.SitesPlans(th.Plans())

			Expect(plans.GetResultingSitesCount()).To(Equal(numSitesTotal),
				fmt.Sprintf("expected %d", numSitesTotal))
		})

		It("reports no errors on success", func() {
			_, th := newSitesTestHelper(GinkgoT())
			th.createSourceSites("src1")
			plans := merge.SitesPlans(th.Plans())

			Expect(len(plans.Errors())).To(Equal(0))
		})

		It("reports invalid action errors", func() {
			// There are no expected cases where an action would remain invalid, so we have
			// to fake one.
			_, th := newSitesTestHelper(GinkgoT())
			th.createSourceSites("src1")
			plans := merge.SitesPlans(th.Plans())
			Expect(len(plans) > 0).To(Equal(true))
			plans[0].Action = merge.SiteActionInvalid

			errs := merge.GetUniqueErrorMessages(plans.Errors())
			Expect(len(errs)).To(Equal(1))
			hasPrefix := strings.HasPrefix(errs[0], "invalid site action for site: ")
			Expect(hasPrefix).To(Equal(true))
		})

		It("prevents merge on invalid actions", func() {
			// There are no expected cases where an action would remain invalid, so we have
			// to fake one.
			_, th := newSitesTestHelper(GinkgoT())
			th.createSourceSites("src1")
			plans := merge.SitesPlans(th.Plans())
			Expect(len(plans) > 0).To(Equal(true))
			plans[0].Action = merge.SiteActionInvalid

			Expect(plans.PreventsMerge()).To(Equal(true))
		})
	})

	Describe("Executor", func() {
		It("creates sites in the target clinic that don't exist", func() {
			ctx, th := newSitesTestHelper(GinkgoT())
			srcSite := th.createSourceSites("src1")[0]
			targetID := th.Target.Id.Hex()
			executor := merge.NewSitePlanExecutor(th.Logger, th.Clinics, th.Patients)

			th.Clinics.EXPECT().CreateSite(gomock.Any(), targetID, &srcSite)
			for _, plan := range th.Plans() {
				Expect(executor.Execute(ctx, plan)).To(Succeed(), plan.Name())
			}
		})

		It("renames sites from the source clinic that already exist", func() {
			ctx, th := newSitesTestHelper(GinkgoT())
			dups := th.createDuplicateSites("dup1")[0]
			targetID := th.Target.Id.Hex()
			executor := merge.NewSitePlanExecutor(th.Logger, th.Clinics, th.Patients)

			th.Clinics.EXPECT().Get(gomock.Any(), targetID).
				Return(th.Target, nil).AnyTimes()
			th.Clinics.EXPECT().CreateSite(gomock.Any(), targetID,
				incrementedSiteMatcher(dups.Target))
			for _, plan := range th.Plans() {
				Expect(executor.Execute(ctx, plan)).To(Succeed(), plan.Name())
			}
		})

		It("renames sites multiple times if necessary", func() {
			ctx, th := newSitesTestHelper(GinkgoT())
			dups := th.createDuplicateSites("dup1")[0]
			times := rand.IntN(20)
			for i := range times {
				th.createTargetSites(fmt.Sprintf("%s (%d)", dups.Target.Name, i+2))
			}
			targetID := th.Target.Id.Hex()
			executor := merge.NewSitePlanExecutor(th.Logger, th.Clinics, th.Patients)

			th.Clinics.EXPECT().Get(gomock.Any(), targetID).
				Return(th.Target, nil).AnyTimes()
			th.Clinics.EXPECT().CreateSite(gomock.Any(), targetID,
				incrementedSiteMatcherN(dups.Target, times+2))
			for _, plan := range th.Plans() {
				Expect(executor.Execute(ctx, plan)).To(Succeed(), plan.Name())
			}
		})
	})
})

type sitesTestHelper struct {
	Source   *clinics.Clinic
	Target   *clinics.Clinic
	Clinics  *clinicsTest.MockService
	Patients *patientsTest.MockService

	Logger *zap.SugaredLogger

	t FullGinkgoTInterface
}

func newSitesTestHelper(t FullGinkgoTInterface) (context.Context, *sitesTestHelper) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(GinkgoWriter), zapcore.DebugLevel)
	logger := zap.New(core).Sugar()

	// RandomClinic() usually returns some number of sites, but those aren't useful here.
	sourceClinic := clinicsTest.RandomClinic()
	sourceClinic.Sites = []sites.Site{}
	targetClinic := clinicsTest.RandomClinic()
	targetClinic.Sites = []sites.Site{}
	return ctx, &sitesTestHelper{
		Source:   sourceClinic,
		Target:   targetClinic,
		Clinics:  clinicsTest.NewMockService(ctrl),
		Patients: patientsTest.NewMockService(ctrl),
		Logger:   logger,
		t:        t,
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

type sitePair struct {
	Source sites.Site
	Target sites.Site
}

func (s *sitesTestHelper) createDuplicateSites(names ...string) []sitePair {
	pairs := []sitePair{}
	for _, name := range names {
		srcSite := sitesTest.Random()
		srcSite.Name = name
		s.Source.Sites = append(s.Source.Sites, srcSite)
		tgtSite := srcSite
		tgtSite.Id = primitive.NewObjectID()
		s.Target.Sites = append(s.Target.Sites, tgtSite)
		pairs = append(pairs, sitePair{Source: srcSite, Target: tgtSite})
	}
	return pairs
}

func (s *sitesTestHelper) createSourceSites(names ...string) []sites.Site {
	newSites := []sites.Site{}
	for _, name := range names {
		site := sitesTest.Random()
		site.Name = name
		newSites = append(newSites, site)
	}
	s.Source.Sites = slices.Concat(s.Source.Sites, newSites)
	return newSites
}

func (s *sitesTestHelper) createTargetSites(names ...string) []sites.Site {
	newSites := []sites.Site{}
	for _, name := range names {
		site := sitesTest.Random()
		site.Name = name
		newSites = append(newSites, site)
	}
	s.Target.Sites = slices.Concat(s.Target.Sites, newSites)
	return newSites
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
