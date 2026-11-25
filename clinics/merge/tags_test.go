package merge_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/pointer"
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

	Describe("Tag Plans", func() {
		tagNames := func(prefix string, count int) []string {
			names := make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("%s %d", prefix, i)
			}
			return names
		}

		// plansFor gives the clinics the named tags and runs both tag planners over them.
		plansFor := func(sourceNames, targetNames []string) merge.TagPlans {
			tags := func(names []string) []clinics.PatientTag {
				result := make([]clinics.PatientTag, len(names))
				for i, name := range names {
					result[i] = clinics.PatientTag{Id: pointer.FromAny(primitive.NewObjectID()), Name: name}
				}
				return result
			}
			source.PatientTags, target.PatientTags = tags(sourceNames), tags(targetNames)

			plans := merge.TagPlans{}
			for _, tag := range source.PatientTags {
				plan, err := merge.NewSourceTagMergePlanner(tag, source, target).Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())
				plans = append(plans, plan)
			}
			for _, tag := range target.PatientTags {
				plan, err := merge.NewTargetTagMergePlanner(tag, source, target).Plan(context.Background())
				Expect(err).ToNot(HaveOccurred())
				plans = append(plans, plan)
			}
			return plans
		}

		It("allows a merge that results in the maximum number of tags", func() {
			plans := plansFor(tagNames("source", 10), tagNames("target", clinics.MaximumPatientTags-10))

			Expect(plans.GetResultingTagsCount()).To(Equal(clinics.MaximumPatientTags))
			Expect(plans.PreventsMerge()).To(BeFalse())
			Expect(plans.Errors()).To(BeEmpty())
		})

		It("prevents a merge that results in more than the maximum number of tags", func() {
			plans := plansFor(tagNames("source", 11), tagNames("target", clinics.MaximumPatientTags-10))

			Expect(plans.PreventsMerge()).To(BeTrue())
			Expect(plans.Errors()).To(ConsistOf(HaveField("Message", SatisfyAll(
				ContainSubstring(fmt.Sprint(clinics.MaximumPatientTags+1)),
				ContainSubstring(fmt.Sprint(clinics.MaximumPatientTags)),
			))))
		})

		It("prevents a merge with a source tag that differs from a target tag only in case", func() {
			plans := plansFor([]string{"type 1"}, []string{"Type 1"})

			Expect(plans).To(ContainElement(SatisfyAll(
				HaveField("Name", "type 1"), HaveField("TagAction", merge.TagActionCreate))))
			Expect(plans.PreventsMerge()).To(BeTrue())
			Expect(plans.Errors()).To(ConsistOf(HaveField("Message", SatisfyAll(
				ContainSubstring(`"type 1"`), ContainSubstring(`"Type 1"`),
				ContainSubstring(*source.Name), ContainSubstring(*target.Name),
			))))
		})

		It("skips a source tag that has the name of a target tag", func() {
			plans := plansFor([]string{"Type 1"}, []string{"Type 1"})

			Expect(plans).To(ContainElement(SatisfyAll(
				HaveField("Workspaces", ContainElement(*source.Name)), HaveField("TagAction", merge.TagActionSkip))))
			Expect(plans.PreventsMerge()).To(BeFalse())
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
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
