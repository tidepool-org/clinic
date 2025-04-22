package merge_test

import (
	"context"
	"slices"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	mergeTest "github.com/tidepool-org/clinic/clinics/merge/test"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store/test"
)

const (
	patientCount                 = 50
	duplicateAccountsCount       = 10
	likelyDuplicateAccountsCount = 9
	nameOnlyMatchAccountsCount   = 8
	mrnOnlyMatchAccountsCount    = 7
)

var _ = Describe("New Merge Planner", func() {
	var source clinics.Clinic
	var sourcePatients []patients.Patient
	var target clinics.Clinic
	var targetPatients []patients.Patient
	var targetPatientsWithDuplicates map[string]patients.Patient
	var plans merge.PatientPlans
	var planner *merge.PatientMergePlanner

	BeforeEach(func() {
		data := mergeTest.RandomData(mergeTest.Params{
			UniquePatientCount:           patientCount,
			DuplicateAccountsCount:       duplicateAccountsCount,
			LikelyDuplicateAccountsCount: likelyDuplicateAccountsCount,
			NameOnlyMatchAccountsCount:   nameOnlyMatchAccountsCount,
			MrnOnlyMatchAccountsCount:    mrnOnlyMatchAccountsCount,
		})
		source = data.Source
		sourcePatients = data.SourcePatients
		target = data.Target
		targetPatients = data.TargetPatients
		targetPatientsWithDuplicates = data.TargetPatientsWithDuplicates

		var err error
		planner, err = merge.NewPatientMergePlanner(source, target, sourcePatients, targetPatients)
		Expect(err).ToNot(HaveOccurred())
		plans, err = planner.Plan(context.Background())
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Plans", func() {
		It("have the expected number of conflicts", func() {
			conflicts := plans.GetConflictCounts()
			Expect(conflicts).To(Equal(map[string]int{
				merge.PatientConflictCategoryDuplicateAccounts:       duplicateAccountsCount,
				merge.PatientConflictCategoryLikelyDuplicateAccounts: likelyDuplicateAccountsCount,
				merge.PatientConflictCategoryMRNOnlyMatch:            mrnOnlyMatchAccountsCount,
				merge.PatientConflictCategoryNameOnlyMatch:           nameOnlyMatchAccountsCount,
			}))
		})

		It("have the expected number of resulting patients", func() {
			count := plans.GetResultingPatientsCount()
			expected := len(sourcePatients) + len(targetPatients) - len(targetPatientsWithDuplicates)

			Expect(count).To(Equal(expected))
		})

		It("produces correct plans", func() {
			for _, plan := range plans {
				switch plan.PatientAction {
				case merge.PatientActionRetain:
					// Retain target account - this action is produced for each target patient which doesn't have conflicts
					Expect(plan.SourcePatient).To(BeNil())
					Expect(plan.Conflicts).To(BeEmpty())
				case merge.PatientActionMerge:
					// Duplicate account - this action is produced for each source patient which has a duplicate in the target clinic
					Expect(plan.TargetPatient).ToNot(BeNil())
					Expect(plan.TargetPatient.UserId).To(PointTo(Equal(*plan.SourcePatient.UserId)))
					Expect(plan.Conflicts).ToNot(BeEmpty())
					Expect(plan.Conflicts[merge.PatientConflictCategoryDuplicateAccounts]).ToNot(BeEmpty())
					Expect(plan.SourceTagNames).ToNot(BeEmpty())
					Expect(plan.PostMigrationTagNames).To(ContainElements(plan.SourceTagNames))
					Expect(plan.TargetTagNames).ToNot(BeEmpty())
					Expect(plan.PostMigrationTagNames).To(ContainElements(plan.TargetTagNames))
				case merge.PatientActionMergeInto:
					// Duplicate account - this action is produced for each target patient which has a duplicate in the source clinic
					Expect(plan.SourcePatient).To(BeNil())
					Expect(plan.TargetPatient).ToNot(BeNil())
					Expect(plan.Conflicts).To(BeEmpty())
				case merge.PatientActionMove:
					// Move source patient to target. There may be conflicts.
					Expect(plan.SourcePatient).ToNot(BeNil())
					Expect(plan.TargetPatient).To(BeNil())
					Expect(plan.SourceTagNames).ToNot(BeEmpty())
					Expect(plan.PostMigrationTagNames).To(ConsistOf(plan.SourceTagNames))
				default:
					Fail("unexpected merge plan action")
				}
			}
		})

		It("can be executed", func() {
			for _, plan := range plans {
				Expect(plan.PreventsMerge()).To(BeFalse())
				switch plan.PatientAction {
				case merge.PatientActionRetain:
					// Retain target account - this action is produced for each target patient which doesn't have conflicts
					Expect(plan.SourcePatient).To(BeNil())
					Expect(plan.Conflicts).To(BeEmpty())
				case merge.PatientActionMerge:
					// Duplicate account - this action is produced for each source patient which has a duplicate in the target clinic
					Expect(plan.TargetPatient).ToNot(BeNil())
					Expect(plan.TargetPatient.UserId).To(PointTo(Equal(*plan.SourcePatient.UserId)))
					Expect(plan.Conflicts).ToNot(BeEmpty())
					Expect(plan.Conflicts[merge.PatientConflictCategoryDuplicateAccounts]).ToNot(BeEmpty())
				case merge.PatientActionMergeInto:
					// Duplicate account - this action is produced for each target patient which has a duplicate in the source clinic
					Expect(plan.SourcePatient).To(BeNil())
					Expect(plan.TargetPatient).ToNot(BeNil())
					Expect(plan.Conflicts).To(BeEmpty())
				case merge.PatientActionMove:
					// Move source patient to target. There may be conflicts.
					Expect(plan.SourcePatient).ToNot(BeNil())
					Expect(plan.TargetPatient).To(BeNil())
				default:
					Fail("unexpected merge plan action")
				}
			}
		})

		It("fails for merge plans with MRN conflicts when the target workspace requires unique MRNs", func() {
			target.MRNSettings = &clinics.MRNSettings{
				Unique: true,
			}
			planner, err := merge.NewPatientMergePlanner(source, target, sourcePatients, targetPatients)
			Expect(err).ToNot(HaveOccurred())

			patientPlans, err := planner.Plan(context.Background())
			Expect(err).ToNot(HaveOccurred())

			Expect(patientPlans.PreventsMerge()).To(BeTrue())
			Expect(patientPlans.Errors()).ToNot(BeEmpty())

			for _, plan := range patientPlans {
				if plan.PatientAction != merge.PatientActionMerge {
					continue
				}
				if len(plan.Conflicts[merge.PatientConflictCategoryMRNOnlyMatch]) == 0 {
					continue
				}

				Expect(plan.Error).To(Equal(merge.ErrorDuplicateMRNInTargetWorkspace))
				Expect(plan.PreventsMerge()).To(BeTrue())
			}
		})
	})

	Describe("Executor", func() {
		var executor *merge.PatientPlanExecutor
		var collection *mongo.Collection
		var updated clinics.Clinic

		BeforeEach(func() {
			db := test.GetTestDatabase()

			clinicsCtrl := gomock.NewController(GinkgoT())
			clinicsService := clinicsTest.NewMockService(clinicsCtrl)

			executor = merge.NewPatientPlanExecutor(zap.NewNop().Sugar(), clinicsService, db)
			collection = db.Collection("patients")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
			defer cancel()

			documents := make([]interface{}, 0, len(sourcePatients)+len(targetPatients))
			for _, p := range sourcePatients {
				documents = append(documents, p)
			}
			for _, p := range targetPatients {
				documents = append(documents, p)
			}

			res, err := collection.InsertMany(ctx, documents)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.InsertedIDs).To(HaveLen(len(documents)))

			// The executor expects tags to be migrated before the patients
			updated = target
			updated.PatientTags = nil

			// Retain target tag ids
			for _, tag := range target.PatientTags {
				updated.PatientTags = append(updated.PatientTags, tag)
			}

			// Recreate source tags in target clinic as if they were migrated
			for _, tag := range source.PatientTags {
				id := primitive.NewObjectID()
				updated.PatientTags = append(updated.PatientTags, clinics.PatientTag{Id: &id, Name: tag.Name})
			}

			clinicsService.EXPECT().
				Get(gomock.Any(), target.Id.Hex()).
				Return(&updated, nil).
				AnyTimes()

			var errs []error
			for _, plan := range plans {
				if err := executor.Execute(ctx, plan, source, target); err != nil {
					errs = append(errs, err)
				}
			}
			Expect(errs).To(BeEmpty())
		})

		AfterEach(func() {

		})

		It("moves all source patients which don't have duplicates", func() {
			for _, patient := range sourcePatients {
				if _, ok := targetPatientsWithDuplicates[*patient.UserId]; ok {
					// Skip source patients which have duplicates
					continue
				}
				count, err := collection.CountDocuments(context.Background(), bson.M{
					"userId":   *patient.UserId,
					"clinicId": *target.Id,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeEquivalentTo(1))
			}
		})

		It("merges tags source tags and retains target tags of all source patients which have duplicates", func() {
			targetTagsById := make(map[string]clinics.PatientTag)
			for _, tag := range target.PatientTags {
				targetTagsById[tag.Id.Hex()] = tag
			}
			sourceTagsById := make(map[string]clinics.PatientTag)
			for _, tag := range source.PatientTags {
				sourceTagsById[tag.Id.Hex()] = tag
			}
			updatedClinicTagsById := make(map[string]clinics.PatientTag)
			for _, tag := range updated.PatientTags {
				updatedClinicTagsById[tag.Id.Hex()] = tag
			}

			for _, patient := range sourcePatients {
				targetPatient, ok := targetPatientsWithDuplicates[*patient.UserId]
				if !ok {
					// Only process patients with duplicates
					continue
				}

				var result patients.Patient
				err := collection.FindOne(context.Background(), bson.M{
					"userId":   *patient.UserId,
					"clinicId": *target.Id,
				}).Decode(&result)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Tags).ToNot(BeNil())

				expectedTagNames := mapset.NewSet[string]()
				for _, tagId := range *patient.Tags {
					expectedTagNames.Append(sourceTagsById[tagId.Hex()].Name)
				}
				for _, tagId := range *targetPatient.Tags {
					expectedTagNames.Append(targetTagsById[tagId.Hex()].Name)
				}

				resultTagNames := make([]string, 0, len(*result.Tags))
				for _, tagId := range *result.Tags {
					resultTagNames = append(resultTagNames, updatedClinicTagsById[tagId.Hex()].Name)
				}

				Expect(resultTagNames).To(ConsistOf(expectedTagNames.ToSlice()))
			}
		})

		Context("a patient that is retained", func() {
			retained := func() (*patients.Patient, []sites.Site) {
				for _, plan := range plans {
					if plan.PatientAction == merge.PatientActionRetain && len(plan.TargetPatient.Sites) > 0 {
						res := collection.FindOne(context.Background(), bson.M{
							"clinicId": plan.TargetClinicId,
							"userId":   *plan.TargetPatient.UserId,
						})
						Expect(res.Err()).To(Succeed())
						p := &patients.Patient{}
						Expect(res.Decode(p)).To(Succeed())
						return p, plan.TargetPatient.Sites
					}
				}
				Fail("no suitable retained patients found")
				return nil, nil
			}

			It("keeps its sites", func() {
				patient, sites := retained()
				Expect(patient.Sites).To(Equal(sites))
			})
		})

		Context("a patient that is moved", func() {
			moved := func() (*patients.Patient, []sites.Site) {
				for _, plan := range plans {
					if plan.PatientAction == merge.PatientActionMove && len(plan.SourcePatient.Sites) > 0 {
						res := collection.FindOne(context.Background(), bson.M{
							"clinicId": plan.TargetClinicId,
							"userId":   *plan.SourcePatient.UserId,
						})
						Expect(res.Err()).To(Succeed())
						p := &patients.Patient{}
						Expect(res.Decode(p)).To(Succeed())
						return p, plan.SourcePatient.Sites
					}
				}
				Fail("no suitable moved patients found")
				return nil, nil
			}

			It("keeps its sites", func() {
				patient, sites := moved()
				Expect(patient.Sites).To(Equal(sites))
			})
		})

		Context("a patient that is merged", func() {
			merged := func() (*patients.Patient, *patients.Patient, *patients.Patient) {
				for _, plan := range plans {
					if plan.PatientAction != merge.PatientActionMerge {
						continue
					}
					if len(plan.SourcePatient.Sites) < 1 {
						continue
					}
					if len(plan.TargetPatient.Sites) < 1 {
						continue
					}
					res := collection.FindOne(context.Background(), bson.M{
						"clinicId": plan.TargetClinicId,
						"userId":   *plan.TargetPatient.UserId,
					})
					if err := res.Err(); err != nil {
						continue
					}
					p := &patients.Patient{}
					if err := res.Decode(p); err != nil {
						continue
					}
					return p, plan.SourcePatient, plan.TargetPatient
				}
				Fail("no suitable merged patients found")
				return nil, nil, nil
			}

			It("has a union of its sites, including duplicates", func() {
				// Note: The execution of a patient plan is not responsible for renaming
				// duplicate site names, so any duplicates will remain.
				patient, srcPatient, targetPatient := merged()
				Expect(patient.Sites).To(ConsistOf(slices.Concat(srcPatient.Sites, targetPatient.Sites)))
			})
		})

		Context("a patient that is merged into", func() {
			mergedInto := func() (*patients.Patient, *patients.Patient) {
				for _, plan := range plans {
					if plan.PatientAction != merge.PatientActionMergeInto {
						continue
					}
					if len(plan.TargetPatient.Sites) < 1 {
						continue
					}
					res := collection.FindOne(context.Background(), bson.M{
						"clinicId": plan.TargetClinicId,
						"userId":   *plan.TargetPatient.UserId,
					})
					if err := res.Err(); err != nil {
						continue
					}
					p := &patients.Patient{}
					if err := res.Decode(p); err != nil {
						continue
					}
					return p, plan.TargetPatient
				}
				Fail("no suitable merged into patients found")
				return nil, nil
			}

			It("doesn't lose its original sites", func() {
				// Note: The execution of a patient plan is not responsible for renaming
				// duplicate site names, so any duplicates will remain.
				//
				// The "merge into" action can't see that the resulting sites are 100%
				// accurate, because there's no record of the source patient in a "merge
				// info" action. However, that behavior is covered in the "merge" action's
				// tests.
				patient, targetPatient := mergedInto()
				Expect(patient.Sites).To(ContainElements(targetPatient.Sites))
			})
		})

		It("removes source patients which have duplicates from the source clinic", func() {
			for _, patient := range sourcePatients {
				if _, ok := targetPatientsWithDuplicates[*patient.UserId]; !ok {
					// Only process patients with duplicates
					continue
				}
				count, err := collection.CountDocuments(context.Background(), bson.M{
					"userId":   *patient.UserId,
					"clinicId": *source.Id,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeEquivalentTo(0))
			}
		})
	})
})
