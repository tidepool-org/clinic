package merge_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		duplicateTags = make(map[string]clinics.PatientTag)
		for _, tag := range clinicsTest.RandomTags(duplicateTagsCount) {
			duplicateTags[tag.Name] = tag
			randomTagId := primitive.NewObjectID()
			source.PatientTags = append(source.PatientTags, clinics.PatientTag{
				Id:   &randomTagId,
				Name: tag.Name,
			})
			randomTagId = primitive.NewObjectID()
			target.PatientTags = append(target.PatientTags, clinics.PatientTag{
				Id:   &randomTagId,
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
})
