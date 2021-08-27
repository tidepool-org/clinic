package patients_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
)

var _ = Describe("Patients Repository", func() {
	var repo patients.Repository
	var database *mongo.Database
	var collection *mongo.Collection

	BeforeEach(func() {
		var err error
		database = dbTest.GetTestDatabase()
		collection = database.Collection("patients")
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repo, err = patients.NewRepository(database, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	Context("with random data", func() {
		var allPatientIds []interface{}
		var allPatients []patients.Patient
		var randomPatient patients.Patient
		var matchPatientFields types.GomegaMatcher
		var count int

		BeforeEach(func() {
			count = 10
			documents := make([]interface{}, count)
			allPatients = make([]patients.Patient, count)
			for i, _ := range documents {
				patient := patientsTest.RandomPatient()
				documents[i] = patient
				allPatients[i] = patient
			}
			result, err := collection.InsertMany(nil, documents)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.InsertedIDs).To(HaveLen(count))
			allPatientIds = result.InsertedIDs

			randomPatient = documents[dbTest.Faker.IntBetween(0, count-1)].(patients.Patient)
			matchPatientFields = patientFieldsMatcher(randomPatient)
		})

		AfterEach(func() {
			selector := primitive.M{
				"_id": primitive.M{
					"$in": allPatientIds,
				},
			}
			result, err := collection.DeleteMany(nil, selector)
			Expect(err).ToNot(HaveOccurred())
			Expect(int(result.DeletedCount)).To(Equal(count))
		})

		Describe("Create", func() {
			var patient patients.Patient

			BeforeEach(func() {
				patient = patientsTest.RandomPatient()
				matchPatientFields = patientFieldsMatcher(patient)
			})

			AfterEach(func() {
				selector := primitive.M{
					"_id": patient.Id,
				}
				result, err := collection.DeleteOne(nil, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(result.DeletedCount)).To(Equal(1))
			})

			It("returns the created patient", func() {
				result, err := repo.Create(nil, patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				Expect(*result).To(matchPatientFields)
			})

			It("inserts the patient in the collection", func() {
				result, err := repo.Create(nil, patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				var inserted patients.Patient
				err = collection.FindOne(nil, primitive.M{"_id": result.Id}).Decode(&inserted)
				Expect(err).ToNot(HaveOccurred())
				Expect(inserted).To(matchPatientFields)

			})
		})

		Describe("Get", func() {
			It("returns the correct patient", func() {
				result, err := repo.Get(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result).To(matchPatientFields)
			})
		})

		Describe("Update", func() {
			var update patients.Patient

			BeforeEach(func() {
				update = patientsTest.RandomPatientUpdate()
				expected := patients.Patient{
					Id:            randomPatient.Id,
					ClinicId:      randomPatient.ClinicId,
					UserId:        randomPatient.UserId,
					BirthDate:     update.BirthDate,
					Email:         update.Email,
					FullName:      update.FullName,
					Mrn:           update.Mrn,
					TargetDevices: update.TargetDevices,
					Permissions:   update.Permissions,
					IsMigrated:    randomPatient.IsMigrated,
				}
				matchPatientFields = patientFieldsMatcher(expected)
			})

			It("updates the patient in the collection", func() {
				result, err := repo.Update(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, update)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var updated patients.Patient
				err = collection.FindOne(nil, primitive.M{"_id": result.Id}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(matchPatientFields)
			})

			It("returns the updated patient", func() {
				result, err := repo.Update(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, update)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result).To(matchPatientFields)
			})
		})

		Describe("Remove", func() {
			It("removes the correct patient from the collection", func() {
				err := repo.Remove(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())

				res := collection.FindOne(nil, primitive.M{"_id": randomPatient.Id})
				Expect(res).ToNot(BeNil())
				Expect(res.Err()).ToNot(BeNil())
				Expect(res.Err()).To(MatchError(mongo.ErrNoDocuments))
				count -= 1
			})
		})

		Describe("List", func() {
			It("returns all patients", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(count))
			})

			It("applies pagination limit correctly", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  2,
				}
				result, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))
			})

			It("applies pagination offset correctly", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  2,
				}
				result, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))

				pagination.Offset = 1
				offsetResults, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(offsetResults).To(HaveLen(2))
				Expect(*offsetResults[0]).To(patientFieldsMatcher(*result[1]))
			})

			It("filters by users id correctly", func() {
				filter := patients.Filter{
					UserId: randomPatient.UserId,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(HaveLen(0))

				for _, patient := range result {
					Expect(patient.UserId).To(Equal(randomPatient.UserId))
				}
			})

			It("filters by clinic id correctly", func() {
				clinicId := randomPatient.ClinicId.Hex()
				filter := patients.Filter{
					ClinicId: &clinicId,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(HaveLen(0))

				for _, patient := range result {
					Expect(patient.ClinicId.Hex()).To(Equal(randomPatient.ClinicId.Hex()))
				}
			})

			It("supports searching by mrn", func() {
				clinicId := randomPatient.ClinicId.Hex()
				filter := patients.Filter{
					ClinicId: &clinicId,
					Search:   randomPatient.Mrn,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(HaveLen(0))

				found := false
				for _, patient := range result {
					if *patient.UserId == *randomPatient.UserId && patient.ClinicId.Hex() == randomPatient.ClinicId.Hex() {
						found = true
						break
					}
				}

				Expect(found).To(BeTrue())
			})
		})

		Describe("Update Permissions", func() {
			It("updates the permissions of patient in the collection", func() {
				permissions := patientsTest.RandomPermissions()
				result, err := repo.UpdatePermissions(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, &permissions)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var updated patients.Patient
				err = collection.FindOne(nil, primitive.M{"_id": result.Id}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(*updated.Permissions).To(Equal(permissions))
			})

			It("returns the updated permissions", func() {
				permissions := patientsTest.RandomPermissions()
				randomPatient.Permissions = &permissions
				matchPatientFields = patientFieldsMatcher(randomPatient)

				result, err := repo.UpdatePermissions(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, &permissions)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result).To(matchPatientFields)
			})
		})

		Describe("Delete Permissions", func() {
			It("removes the permission from the patient record", func() {
				// make sure all permissions are set
				_, err := repo.UpdatePermissions(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, &patients.CustodialAccountPermissions)
				Expect(err).ToNot(HaveOccurred())

				permission := patientsTest.RandomPermission()
				result, err := repo.DeletePermission(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, permission)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				path := fmt.Sprintf("permissions.%s", permission)
				res := collection.FindOne(nil, primitive.M{"_id": result.Id, path: primitive.M{"$exists": "true"}})
				Expect(res).ToNot(BeNil())
				Expect(res.Err()).To(MatchError(mongo.ErrNoDocuments))
			})

			It("returns an error if a permissions is not set", func() {
				// make sure all permissions are set
				_, err := repo.UpdatePermissions(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, &patients.CustodialAccountPermissions)
				Expect(err).ToNot(HaveOccurred())

				permission := patientsTest.RandomPermission()
				result, err := repo.DeletePermission(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, permission)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				result, err = repo.DeletePermission(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId, permission)
				Expect(err).To(MatchError(patients.ErrPermissionNotFound))
			})
		})
	})

})

func patientFieldsMatcher(patient patients.Patient) types.GomegaMatcher {
	return MatchAllFields(Fields{
		"Id":            PointTo(Not(BeEmpty())),
		"UserId":        PointTo(Equal(*patient.UserId)),
		"ClinicId":      PointTo(Equal(*patient.ClinicId)),
		"BirthDate":     PointTo(Equal(*patient.BirthDate)),
		"Email":         PointTo(Equal(*patient.Email)),
		"FullName":      PointTo(Equal(*patient.FullName)),
		"Mrn":           PointTo(Equal(*patient.Mrn)),
		"TargetDevices": PointTo(Equal(*patient.TargetDevices)),
		"Permissions":   PointTo(Equal(*patient.Permissions)),
		"IsMigrated":    Equal(patient.IsMigrated),
	})
}
