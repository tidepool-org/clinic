package merge_test

import (
	"context"
	mapset "github.com/deckarep/golang-set/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"math/rand"
	"time"
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
	var target clinics.Clinic
	var sourcePatients []patients.Patient
	var targetPatientsWithDuplicates map[string]patients.Patient
	var targetPatients []patients.Patient
	var plans merge.PatientPlans

	BeforeEach(func() {
		source = *clinicsTest.RandomClinic()
		target = *clinicsTest.RandomClinic()
		sourcePatients = make([]patients.Patient, patientCount)
		targetPatients = make([]patients.Patient, patientCount)
		targetPatientsWithDuplicates = make(map[string]patients.Patient)

		for i := 0; i < patientCount; i++ {
			sourcePatient := patientsTest.RandomPatient()
			sourcePatient.ClinicId = source.Id
			sourcePatient.Tags = randomTagIds(len(source.PatientTags) - 1, source.PatientTags)
			sourcePatients[i] = sourcePatient

			targetPatient := patientsTest.RandomPatient()
			targetPatient.ClinicId = target.Id
			targetPatient.Tags = randomTagIds(len(target.PatientTags) - 1, target.PatientTags)
			targetPatients[i] = targetPatient
		}

		i := 0
		for j := 0; j < duplicateAccountsCount; j++ {
			targetPatient := duplicatePatientAccount(target.Id, sourcePatients[i])
			targetPatient.Tags = randomTagIds(len(target.PatientTags) - 1, target.PatientTags)
			targetPatients = append(targetPatients, targetPatient)
			targetPatientsWithDuplicates[*sourcePatients[i].UserId] = targetPatient
			i++
		}
		for j := 0; j < likelyDuplicateAccountsCount; j++ {
			targetPatient := likelyDuplicatePatientAccount(target.Id, sourcePatients[i])
			targetPatients = append(targetPatients, targetPatient)
			i++
		}
		for j := 0; j < nameOnlyMatchAccountsCount; j++ {
			targetPatient := nameOnlyMatchPatientAccount(target.Id, sourcePatients[i])
			targetPatients = append(targetPatients, targetPatient)
			i++
		}
		for j := 0; j < mrnOnlyMatchAccountsCount; j++ {
			targetPatient := mrnOnlyMatchPatientAccount(target.Id, sourcePatients[i])
			targetPatients = append(targetPatients, targetPatient)
			i++
		}

		planner, err := merge.NewPatientMergePlanner(source, target, sourcePatients, targetPatients)
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
			expected := len(sourcePatients) + len(targetPatients) - duplicateAccountsCount

			Expect(count).To(Equal(expected))
		})

		It("produces correct plans", func() {
			for _, plan := range plans {
				switch plan.PatientAction{
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
	})

	Describe("Executor", func() {
		var executor *merge.PatientPlanExecutor
		var collection *mongo.Collection

		BeforeEach(func() {
			db := test.GetTestDatabase()
			executor = merge.NewPatientPlanExecutor(zap.NewNop().Sugar(), db)
			collection = db.Collection("patients")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second * 20)
			defer cancel()

			documents := make([]interface{}, 0, len(sourcePatients) + len(targetPatients))
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
			for _, tag := range source.PatientTags {
				id := primitive.NewObjectID()
				target.PatientTags = append(target.PatientTags, clinics.PatientTag{Id: &id, Name: tag.Name})
			}

			var errs []error
			for _, plan := range plans {
				if err := executor.Execute(ctx, plan, source, target); err != nil {
					errs = append(errs, err)
				}
			}
			Expect(errs).To(BeEmpty())
		})

		It("moves all source patients which don't have duplicates", func() {
			for _, patient := range sourcePatients {
				if _, ok := targetPatientsWithDuplicates[*patient.UserId]; ok {
					// Skip source patients which have duplicates
					continue
				}
				count, err := collection.CountDocuments(context.Background(), bson.M{
					"userId": *patient.UserId,
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
			for _, patient := range sourcePatients {
				targetPatient, ok := targetPatientsWithDuplicates[*patient.UserId]
				if !ok {
					// Only process patients with duplicates
					continue
				}

				var result patients.Patient
				err := collection.FindOne(context.Background(), bson.M{
					"userId": *patient.UserId,
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

				resultTagNames := make([]string, 0, len(*patient.Tags))
				for _, tagId := range *result.Tags {
					resultTagNames = append(resultTagNames, targetTagsById[tagId.Hex()].Name)
				}

				Expect(resultTagNames).To(ConsistOf(expectedTagNames.ToSlice()))
			}
		})

		It("removes source patients which have duplicates from the source clinic", func() {
			for _, patient := range sourcePatients {
				if _, ok := targetPatientsWithDuplicates[*patient.UserId]; !ok {
					// Only process patients with duplicates
					continue
				}
				count, err := collection.CountDocuments(context.Background(), bson.M{
					"userId": *patient.UserId,
					"clinicId": *source.Id,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeEquivalentTo(0))
			}
		})
	})
})


func duplicatePatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId
	duplicate.UserId = patient.UserId
	return duplicate
}

func mrnOnlyMatchPatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId
	duplicate.Mrn = patient.Mrn
	return duplicate
}

func nameOnlyMatchPatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId
	duplicate.FullName = patient.FullName
	return duplicate
}

func likelyDuplicatePatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId

	r := rand.Intn(3)
	if r == 0 {
		duplicate.FullName = patient.FullName
		duplicate.BirthDate = patient.BirthDate
	} else if r == 1 {
		duplicate.FullName = patient.FullName
		duplicate.Mrn = patient.Mrn
	} else if r == 2 {
		duplicate.BirthDate = patient.BirthDate
		duplicate.Mrn = patient.Mrn
	}

	return duplicate
}

func randomTagIds(count int, tags []clinics.PatientTag) *[]primitive.ObjectID {
	if count > len(tags) {
		count = len(tags)
	}
	rand.Shuffle(len(tags), func(i, j int) {
		tags[i], tags[j] = tags[j], tags[i]
	})
	result := make([]primitive.ObjectID, count)
	for i := range result {
		result[i] = *tags[i].Id
	}
	return &result
}
