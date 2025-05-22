package merge_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/sites"
	sitesTest "github.com/tidepool-org/clinic/sites/test"
)

const (
	duplicateSitesCount = 5
)

var _ = Describe("Sites", func() {
	var source clinics.Clinic
	var target clinics.Clinic
	var duplicateSites map[string]sites.Site

	BeforeEach(func() {
		source = *clinicsTest.RandomClinic()
		target = *clinicsTest.RandomClinic()
		duplicateSites = make(map[string]sites.Site)
		for _, site := range sitesTest.RandomSlice(duplicateSitesCount) {
			duplicateSites[site.Name] = site
			source.Sites = append(source.Sites, site)
			tSite := site
			tSite.Id = primitive.NewObjectID()
			target.Sites = append(target.Sites, tSite)
		}
	})

	Describe("Source Sites Planner", func() {
		It("creates a correct plan", func() {
			for _, site := range source.Sites {
				planner := merge.NewSourceSiteMergePlanner(site, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())

				_, isDuplicate := duplicateSites[site.Name]
				expectedWorkspaces := []string{*source.Name}
				expectedSiteAction := merge.SiteActionCreate
				if isDuplicate {
					expectedSiteAction = merge.SiteActionRename
					expectedWorkspaces = append(expectedWorkspaces, *target.Name)
				}

				Expect(plan.Name()).To(Equal(site.Name))
				Expect(plan.Merge).To(BeFalse())
				Expect(plan.PreventsMerge()).To(BeFalse())
				Expect(plan.SiteAction).To(Equal(expectedSiteAction))
				Expect(plan.Workspaces).To(ConsistOf(expectedWorkspaces))

			}
		})
	})

	Describe("Target Sites Planner", func() {
		It("creates a correct plan", func() {
			for _, site := range target.Sites {
				planner := merge.NewTargetSiteMergePlanner(site, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())

				_, isDuplicate := duplicateSites[site.Name]
				expectedWorkspaces := []string{*target.Name}
				expectedSiteAction := merge.SiteActionRetain
				expectedMerge := false
				if isDuplicate {
					expectedMerge = true
					expectedWorkspaces = append(expectedWorkspaces, *source.Name)
				}

				Expect(plan.Merge).To(Equal(expectedMerge))
				Expect(plan.Name()).To(Equal(site.Name))
				Expect(plan.PreventsMerge()).To(BeFalse())
				Expect(plan.SiteAction).To(Equal(expectedSiteAction))
				Expect(plan.Workspaces).To(ConsistOf(expectedWorkspaces))

			}
		})
	})

	Describe("Site Plan Executor", func() {
		var plans []merge.SitePlan
		var executor *merge.SitePlanExecutor
		var clinicsService *clinicsTest.MockService
		var clinicsCtrl *gomock.Controller

		BeforeEach(func() {
			nSites := len(source.Sites)
			for len(source.Sites) < nSites+3 {
				rSite := sitesTest.Random()
				if !sites.SiteExistsWithName(source.Sites, rSite.Name) {
					source.Sites = append(source.Sites, rSite)
				}
			}

			for _, site := range source.Sites {
				planner := merge.NewSourceSiteMergePlanner(site, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())
				plans = append(plans, plan)
			}
			for _, site := range target.Sites {
				planner := merge.NewTargetSiteMergePlanner(site, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())
				plans = append(plans, plan)
			}

			clinicsCtrl = gomock.NewController(GinkgoT())
			clinicsService = clinicsTest.NewMockService(clinicsCtrl)
			// zap.Must(zap.NewDevelopment()).Sugar()
			executor = merge.NewSitePlanExecutor(zap.NewNop().Sugar(), clinicsService)
		})

		AfterEach(func() {
			clinicsCtrl.Finish()
		})

		It("increments the suffix on duplicate sites", func() {
			targetID := target.Id.Hex()
			GinkgoT().Logf("sourceID is %s; targetID is %s", source.Id.Hex(), targetID)
			GinkgoT().Logf("source has these sites: %+v", source.Sites)
			for _, site := range source.Sites {
				if _, found := duplicateSites[site.Name]; found {
					clinicsService.EXPECT().ListSites(gomock.Any(), targetID).
						Return(target.Sites, nil)
					clinicsService.EXPECT().CreateSite(gomock.Any(), targetID, incrementedSiteMatcher(site)).
						Return(nil)
				} else {
					clinicsService.EXPECT().CreateSite(gomock.Any(), targetID, siteMatcher(site)).
						Return(nil)
				}
			}
			ctx := context.Background()
			var errs []error
			for _, plan := range plans {
				if err := executor.Execute(ctx, plan); err != nil {
					errs = append(errs, err)
				}
			}
			Expect(errs).To(BeEmpty())
		})

		It("creates a site in the target clinic for all non-overlapping sites", func() {
			ctx := context.Background()
			GinkgoT().Logf("sourceID is %s; targetID is %s", source.Id.Hex(), target.Id.Hex())
			GinkgoT().Logf("source has these sites: %+v", source.Sites)
			created := 0
			targetID := target.Id.Hex()
			for _, site := range source.Sites {
				if _, found := duplicateSites[site.Name]; found {
					clinicsService.EXPECT().ListSites(gomock.Any(), targetID).
						Return(target.Sites, nil)
					clinicsService.EXPECT().CreateSite(gomock.Any(), targetID, incrementedSiteMatcher(site)).
						Return(nil)
					GinkgoT().Logf("duplicate site will be incremented %s", site)
				} else {
					clinicsService.EXPECT().CreateSite(gomock.Any(), targetID, siteMatcher(site)).
						Return(nil)
					GinkgoT().Logf("will create site %s", site)
					created++
				}
			}

			var errs []error
			for _, plan := range plans {
				if err := executor.Execute(ctx, plan); err != nil {
					errs = append(errs, err)
				}
			}
			Expect(errs).To(BeEmpty())

			Expect(created).To(Equal(len(source.Sites) - len(duplicateSites)))
		})
	})
})

func siteMatcher(toMatch sites.Site) gomock.Matcher {
	return &condSiteMatcher{name: toMatch.Name}
}

func incrementedSiteMatcher(toMatch sites.Site) gomock.Matcher {
	return &condSiteMatcher{name: toMatch.Name + " (2)"}
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
