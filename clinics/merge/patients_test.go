package merge_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/rand"
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
	var targetPatients []patients.Patient
	var plans merge.PatientPlans

	BeforeEach(func() {
		source = *clinicsTest.RandomClinic()
		target = *clinicsTest.RandomClinic()
		sourcePatients = make([]patients.Patient, patientCount)
		targetPatients = make([]patients.Patient, patientCount)
		for i := 0; i < patientCount; i++ {
			sourcePatient := patientsTest.RandomPatient()
			sourcePatient.ClinicId = source.Id
			sourcePatients[i] = sourcePatient

			targetPatient := patientsTest.RandomPatient()
			targetPatient.ClinicId = target.Id
			targetPatients[i] = targetPatient
		}

		i := 0
		for j := 0; j < duplicateAccountsCount; j++ {
			targetPatient := duplicatePatientAccount(target.Id, sourcePatients[i])
			targetPatients = append(targetPatients, targetPatient)
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
