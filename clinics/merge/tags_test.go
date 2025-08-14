package merge_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/pointer"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"time"
)

const (
	duplicateTagsCount = 5
)

var _ = Describe("Tags", func() {
	var source clinics.Clinic
	var target clinics.Clinic
	var duplicateTags map[string]clinics.PatientTag

	BeforeEach(func() {
		source = *clinicsTest.RandomClinic()
		target = *clinicsTest.RandomClinic()
		uniqueTags := make(map[string]clinics.PatientTag)
		duplicateTags = make(map[string]clinics.PatientTag)
		for _, tag := range source.PatientTags {
			uniqueTags[tag.Name] = tag
		}
		for _, tag := range target.PatientTags {
			_, exists := uniqueTags[tag.Name]
			if exists {
				duplicateTags[tag.Name] = tag
			} else {
				uniqueTags[tag.Name] = tag
			}
		}
		for _, tag := range clinicsTest.RandomTags(duplicateTagsCount - len(duplicateTags)) {
			duplicateTags[tag.Name] = tag
			source.PatientTags = append(source.PatientTags, clinics.PatientTag{
				Id:   pointer.FromAny(primitive.NewObjectID()),
				Name: tag.Name,
			})
			target.PatientTags = append(target.PatientTags, clinics.PatientTag{
				Id:   pointer.FromAny(primitive.NewObjectID()),
				Name: tag.Name,
			})
		}
	})

	Describe("Source Tags Planner", func() {
		It("creates a correct plan", func() {
			for _, tag := range source.PatientTags {
				planner := merge.NewSourceTagMergePlanner(tag, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())

				_, isDuplicate := duplicateTags[tag.Name]
				expectedWorkspaces := []string{*source.Name}
				expectedTagAction := merge.TagActionCreate
				if isDuplicate {
					expectedTagAction = merge.TagActionSkip
					expectedWorkspaces = append(expectedWorkspaces, *target.Name)
				}

				Expect(plan.Name).To(Equal(tag.Name))
				Expect(plan.Merge).To(BeFalse())
				Expect(plan.PreventsMerge()).To(BeFalse())
				Expect(plan.TagAction).To(Equal(expectedTagAction))
				Expect(plan.Workspaces).To(ConsistOf(expectedWorkspaces))

			}
		})
	})

	Describe("Target Tags Planner", func() {
		It("creates a correct plan", func() {
			for _, tag := range target.PatientTags {
				planner := merge.NewTargetTagMergePlanner(tag, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())

				_, isDuplicate := duplicateTags[tag.Name]
				expectedWorkspaces := []string{*target.Name}
				expectedTagAction := merge.TagActionRetain
				expectedMerge := false
				if isDuplicate {
					expectedMerge = true
					expectedWorkspaces = append(expectedWorkspaces, *source.Name)
				}

				Expect(plan.Merge).To(Equal(expectedMerge))
				Expect(plan.Name).To(Equal(tag.Name))
				Expect(plan.PreventsMerge()).To(BeFalse())
				Expect(plan.TagAction).To(Equal(expectedTagAction))
				Expect(plan.Workspaces).To(ConsistOf(expectedWorkspaces))

			}
		})
	})

	Describe("Tag Plan Executor", func() {
		var plans []merge.TagPlan
		var executor *merge.TagPlanExecutor
		var clinicsService *clinicsTest.MockService
		var clinicsCtrl *gomock.Controller

		BeforeEach(func() {
			plans = []merge.TagPlan{}
			for _, tag := range source.PatientTags {
				planner := merge.NewSourceTagMergePlanner(tag, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())
				plans = append(plans, plan)
			}
			for _, tag := range target.PatientTags {
				planner := merge.NewTargetTagMergePlanner(tag, source, target)
				plan, err := planner.Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())
				plans = append(plans, plan)
			}

			clinicsCtrl = gomock.NewController(GinkgoT())
			clinicsService = clinicsTest.NewMockService(clinicsCtrl)
			executor = merge.NewTagPlanExecutor(zap.NewNop().Sugar(), clinicsService)
		})

		AfterEach(func() {
			clinicsCtrl.Finish()
		})

		It("creates a tag in the target clinic for all non-overlapping tags", func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second * 20)
			defer cancel()

			created := 0
			for _, tag := range source.PatientTags {
				if _, ok := duplicateTags[tag.Name]; ok {
					continue
				}
				clinicsService.EXPECT().
					CreatePatientTag(gomock.Any(), gomock.Eq(target.Id.Hex()), gomock.Eq(tag.Name)).
					Do(func(_ context.Context, _ string, _ string) {
						created++
					})
			}

			var errs []error
			for _, plan := range plans {
				if err := executor.Execute(ctx, plan); err != nil {
					errs = append(errs, err)
				}
			}
			Expect(errs).To(BeEmpty())

			expectedCreatedCount := len(source.PatientTags) - duplicateTagsCount
			Expect(created).To(Equal(expectedCreatedCount))
			Expect(created).To(BeNumerically(">", 0))
		})
	})
})
