package merge_test

import (
	"context"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
)

//// ClinicianActionRetain is used for target clinicians when there's no corresponding clinician in the source clinic
//ClinicianActionRetain = "KEEP"
//// ClinicianActionMerge is used when the source clinician will be merged to a target clinician record
//ClinicianActionMerge = "MERGE"
//// ClinicianActionMergeInto is when the target record will be the recipient of a merge
//ClinicianActionMergeInto = "MERGE_INTO"
//// ClinicianActionMove is used when the source clinician will be moved to the target clinic
//ClinicianActionMove = "MOVE"

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
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(nil, clinicians.ErrNotFound)

			planner := merge.NewSourceClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionMove))
			Expect(plan.Email).To(Equal(*clinician.Email))
			Expect(plan.Name).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*source.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})

		It("returns a plan to merge the clinician if the clinician is a duplicate", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id

			duplicate := cliniciansTest.RandomClinician()
			duplicate.ClinicId = target.Id
			duplicate.Roles = clinician.Roles

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(duplicate, nil)

			planner := merge.NewSourceClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionMerge))
			Expect(plan.Email).To(Equal(*clinician.Email))
			Expect(plan.Name).To(Equal(*clinician.Name))
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
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(nil, clinicians.ErrNotFound)

			planner := merge.NewTargetClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionRetain))
			Expect(plan.Email).To(Equal(*clinician.Email))
			Expect(plan.Name).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*target.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})

		It("returns a plan to merge the clinician if the clinician is a duplicate", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = source.Id

			duplicate := cliniciansTest.RandomClinician()
			duplicate.ClinicId = target.Id
			duplicate.Roles = clinician.Roles

			cliniciansService.EXPECT().
				Get(gomock.Any(), target.Id.Hex(), *clinician.UserId).
				Return(duplicate, nil)

			planner := merge.NewTargetClinicianMergePlanner(*clinician, source, target, cliniciansService)
			plan, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(plan.ClinicianAction).To(Equal(merge.ClinicianActionMergeInto))
			Expect(plan.Email).To(Equal(*clinician.Email))
			Expect(plan.Name).To(Equal(*clinician.Name))
			Expect(plan.Workspaces).To(ConsistOf(*source.Name, *target.Name))
			Expect(plan.ResultingRoles).To(ConsistOf(clinician.Roles))
			Expect(plan.PreventsMerge()).To(BeFalse())
		})
	})
})
