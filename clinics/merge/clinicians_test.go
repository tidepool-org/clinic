package merge_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"time"
)

var _ = Describe("Clinicians", func() {
	var source clinics.Clinic
	var target clinics.Clinic
	var cliniciansCtrl *gomock.Controller
	var cliniciansService *cliniciansTest.MockService

	BeforeEach(func() {
		source = *clinicsTest.RandomClinic()
		target = *clinicsTest.RandomClinic()

		tb := GinkgoT()
		cliniciansCtrl = gomock.NewController(tb)
		cliniciansService = cliniciansTest.NewMockService(cliniciansCtrl)
	})

	AfterEach(func() {
		cliniciansCtrl.Finish()
	})

	Describe("Source Clinician Merge Planner", func() {
		It("returns a plan to move the clinician if the clinician is not a duplicate", func() {
			id := primitive.NewObjectID()
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id
			clinician.Id = &id

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(nil, clinicians.ErrNotFound)

			planner := merge.NewSourceClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionMove))
			Expect(plan.GetClinicianEmail()).To(Equal(*clinician.Email))
			Expect(plan.GetClinicianName()).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*source.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})

		It("returns a plan to merge the clinician if the clinician is a duplicate", func() {
			id := primitive.NewObjectID()
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id
			clinician.Id = &id

			dupId := primitive.NewObjectID()
			duplicate := cliniciansTest.RandomClinician()
			duplicate.UserId = clinician.UserId
			duplicate.ClinicId = target.Id
			duplicate.Roles = clinician.Roles
			duplicate.Id = &dupId

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(duplicate, nil)

			planner := merge.NewSourceClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionMerge))
			Expect(plan.GetClinicianEmail()).To(Equal(*clinician.Email))
			Expect(plan.GetClinicianName()).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*source.Name, *target.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})

		It("returns a plan to merge the clinician if the clinician is a duplicate", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			duplicate := cliniciansTest.RandomClinician()
			duplicate.ClinicId = target.Id
			duplicate.Roles = []string{"CLINIC_MEMBER"}

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(duplicate, nil)

			planner := merge.NewSourceClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ResultingRoles).To(ConsistOf(duplicate.Roles))
			Expect(plan.Downgraded).To(BeTrue())
		})
	})

	Describe("Target Clinician Merge Planner", func() {
		It("returns a plan to retain the clinician if the clinician is not a duplicate", func() {
			id := primitive.NewObjectID()
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id
			clinician.Id = &id

			cliniciansService.EXPECT().
				Get(gomock.Any(), source.Id.Hex(), *clinician.UserId).
				Return(nil, clinicians.ErrNotFound)

			planner := merge.NewTargetClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionRetain))
			Expect(plan.GetClinicianEmail()).To(Equal(*clinician.Email))
			Expect(plan.GetClinicianName()).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*target.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})

		It("returns a plan to merge the clinician if the clinician is a duplicate", func() {
			id := primitive.NewObjectID()
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id
			clinician.Id = &id

			dupId := primitive.NewObjectID()
			duplicate := cliniciansTest.RandomClinician()
			duplicate.ClinicId = target.Id
			duplicate.Roles = clinician.Roles
			duplicate.Id = &dupId

			cliniciansService.EXPECT().
				Get(gomock.Any(), source.Id.Hex(), *clinician.UserId).
				Return(duplicate, nil)

			planner := merge.NewTargetClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionMergeInto))
			Expect(plan.GetClinicianEmail()).To(Equal(*clinician.Email))
			Expect(plan.GetClinicianName()).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*source.Name, *target.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})
	})

	Describe("Clinician Plan Executor", func() {
		var sourceClinician clinicians.Clinician
		var targetClinician clinicians.Clinician
		var collection *mongo.Collection
		var executor *merge.ClinicianPlanExecutor

		BeforeEach(func() {
			collection = test.GetTestDatabase().Collection("clinicians")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second * 20)
			defer cancel()

			executor = merge.NewClinicianPlanExecutor(zap.NewNop().Sugar(), test.GetTestDatabase())

			sourceId := primitive.NewObjectID()
			sourceClinician = *cliniciansTest.RandomClinician()
			sourceClinician.ClinicId = source.Id
			sourceClinician.Id = &sourceId

			targetId := primitive.NewObjectID()
			targetClinician = *cliniciansTest.RandomClinician()
			targetClinician.ClinicId = target.Id
			targetClinician.Id = &targetId

			res, err := collection.InsertMany(ctx, []interface{}{sourceClinician, targetClinician})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.InsertedIDs).To(HaveLen(2))
		})

		It("moves clinician from source to target clinics", func() {
			plan := merge.ClinicianPlan{
				Clinician: sourceClinician,
				ClinicianAction: merge.ClinicianActionMove,
			}
			err := executor.Execute(context.Background(), plan, target)
			Expect(err).ToNot(HaveOccurred())

			Expect(clinicianExists(collection, *source.Id, *plan.Clinician.UserId)).To(BeFalse())
			Expect(clinicianExists(collection, *target.Id, *plan.Clinician.UserId)).To(BeTrue())
		})

		It("retains target clinician when action is retain", func() {
			plan := merge.ClinicianPlan{
				Clinician: targetClinician,
				ClinicianAction: merge.ClinicianActionRetain,
			}
			err := executor.Execute(context.Background(), plan, target)
			Expect(err).ToNot(HaveOccurred())

			Expect(clinicianExists(collection, *target.Id, *plan.Clinician.UserId)).To(BeTrue())
		})

		It("retains target clinician when action is 'merge into'", func() {
			plan := merge.ClinicianPlan{
				Clinician: targetClinician,
				ClinicianAction: merge.ClinicianActionMergeInto,
			}
			err := executor.Execute(context.Background(), plan, target)
			Expect(err).ToNot(HaveOccurred())

			Expect(clinicianExists(collection, *target.Id, *plan.Clinician.UserId)).To(BeTrue())
		})

		When("there is a duplicate", func() {
			BeforeEach(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second * 20)
				defer cancel()

				selector := bson.M{
					"clinicId": *targetClinician.ClinicId,
					"userId": *targetClinician.UserId,
				}
				update := bson.M{
					"$set": bson.M{
						"userId": sourceClinician.UserId,
					},
				}
				res, err := collection.UpdateOne(ctx, selector, update)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.ModifiedCount).To(BeEquivalentTo(1))
			})

			It("removes clinician from the source clinic", func() {
				plan := merge.ClinicianPlan{
					Clinician: sourceClinician,
					ClinicianAction: merge.ClinicianActionMerge,
				}
				err := executor.Execute(context.Background(), plan, target)
				Expect(err).ToNot(HaveOccurred())

				Expect(clinicianExists(collection, *source.Id, *plan.Clinician.UserId)).To(BeFalse())
			})

			It("retains the clinician from the target clinic", func() {
				plan := merge.ClinicianPlan{
					Clinician: sourceClinician,
					ClinicianAction: merge.ClinicianActionMerge,
				}
				err := executor.Execute(context.Background(), plan, target)
				Expect(err).ToNot(HaveOccurred())

				Expect(clinicianExists(collection, *source.Id, *plan.Clinician.UserId)).To(BeFalse())
			})
		})
	})
})

func clinicianExists(collection *mongo.Collection, clinicId primitive.ObjectID, userId string) bool {
	count, err := collection.CountDocuments(context.Background(), bson.M{
		"clinicId": clinicId,
		"userId": userId,
	})
	Expect(err).ToNot(HaveOccurred())
	return count > 0
}
