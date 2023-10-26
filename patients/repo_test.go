package patients_test

import (
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

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
		repo, err = patients.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
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

			It("successfully inserts a patient with duplicate mrn if uniqueness is not enabled", func() {
				patient.RequireUniqueMrn = false
				result, err := repo.Create(nil, patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.Mrn = patient.Mrn
				secondPatient.RequireUniqueMrn = false

				result, err = repo.Create(nil, secondPatient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				_, err = collection.DeleteOne(nil, primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("successfully inserts multiple patients without mrns when uniqueness is enabled", func() {
				patient.Mrn = nil
				patient.RequireUniqueMrn = true
				result, err := repo.Create(nil, patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.Mrn = nil
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(nil, secondPatient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				_, err = collection.DeleteOne(nil, primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error when a user with duplicate mrn is created", func() {
				patient.RequireUniqueMrn = true
				result, err := repo.Create(nil, patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.Mrn = patient.Mrn
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(nil, secondPatient)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("returns an error when a patient is updated with a duplicated mrn", func() {
				patient.RequireUniqueMrn = true
				result, err := repo.Create(nil, patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(nil, secondPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				result.Mrn = patient.Mrn
				updated, err := repo.Update(nil, patients.PatientUpdate{
					ClinicId:  result.ClinicId.Hex(),
					UserId:    *result.UserId,
					Patient:   *result,
					UpdatedBy: "12345",
				})
				Expect(err).To(HaveOccurred())
				Expect(updated).To(BeNil())

				_, err = collection.DeleteOne(nil, primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("successfully updates the mrn if it is not a duplicate", func() {
				patient.RequireUniqueMrn = true
				result, err := repo.Create(nil, patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(nil, secondPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				result.Mrn = patientsTest.RandomPatient().Mrn
				result, err = repo.Update(nil, patients.PatientUpdate{
					ClinicId:  result.ClinicId.Hex(),
					UserId:    *result.UserId,
					Patient:   *result,
					UpdatedBy: "12345",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				_, err = collection.DeleteOne(nil, primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates legacy clinician ids if the patient exists already", func() {
				result, err := repo.Create(nil, patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				patient.Id = result.Id
				patient.LegacyClinicianIds = []string{test.Faker.UUID().V4()}
				matchPatientFields = patientFieldsMatcher(patient)

				result, err = repo.Create(nil, patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var inserted patients.Patient
				err = collection.FindOne(nil, primitive.M{"_id": result.Id}).Decode(&inserted)
				Expect(err).ToNot(HaveOccurred())
				Expect(inserted).To(matchPatientFields)
			})

			It("to fail if patient exists already and legacy clinician ids is not set", func() {
				result, err := repo.Create(nil, patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				patient.Id = result.Id

				result, err = repo.Create(nil, patient)
				Expect(err).To(Equal(patients.ErrDuplicatePatient))
				Expect(result).To(BeNil())
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
			var update patients.PatientUpdate

			BeforeEach(func() {
				update = patientsTest.RandomPatientUpdate()
				expected := patients.Patient{
					Id:                             randomPatient.Id,
					ClinicId:                       randomPatient.ClinicId,
					UserId:                         randomPatient.UserId,
					BirthDate:                      update.Patient.BirthDate,
					Email:                          update.Patient.Email,
					FullName:                       update.Patient.FullName,
					Mrn:                            update.Patient.Mrn,
					Tags:                           update.Patient.Tags,
					TargetDevices:                  update.Patient.TargetDevices,
					Permissions:                    update.Patient.Permissions,
					IsMigrated:                     randomPatient.IsMigrated,
					LastRequestedDexcomConnectTime: update.Patient.LastRequestedDexcomConnectTime,
					DataSources:                    update.Patient.DataSources,
				}
				matchPatientFields = patientFieldsMatcher(expected)
			})

			It("updates the patient in the collection", func() {
				update.ClinicId = randomPatient.ClinicId.Hex()
				update.UserId = *randomPatient.UserId
				result, err := repo.Update(nil, update)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var updated patients.Patient
				err = collection.FindOne(nil, primitive.M{"_id": result.Id}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(matchPatientFields)
			})

			It("returns the updated patient", func() {
				update.ClinicId = randomPatient.ClinicId.Hex()
				update.UserId = *randomPatient.UserId
				result, err := repo.Update(nil, update)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result).To(matchPatientFields)
			})
		})

		Describe("Update", func() {
			var update patients.PatientUpdate

			BeforeEach(func() {
				update = patientsTest.RandomPatientUpdate()
				expected := patients.Patient{
					Id:            randomPatient.Id,
					ClinicId:      randomPatient.ClinicId,
					UserId:        randomPatient.UserId,
					BirthDate:     randomPatient.BirthDate,
					Email:         update.Patient.Email,
					FullName:      randomPatient.FullName,
					Mrn:           randomPatient.Mrn,
					Tags:          randomPatient.Tags,
					TargetDevices: randomPatient.TargetDevices,
					Permissions:   randomPatient.Permissions,
					IsMigrated:    randomPatient.IsMigrated,
					DataSources:   randomPatient.DataSources,
				}
				matchPatientFields = patientFieldsMatcher(expected)
			})

			It("updates the email", func() {
				update.UserId = *randomPatient.UserId
				err := repo.UpdateEmail(nil, *randomPatient.UserId, update.Patient.Email)
				Expect(err).ToNot(HaveOccurred())

				var updated patients.Patient
				err = collection.FindOne(nil, primitive.M{
					"userId": randomPatient.UserId,
				}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(matchPatientFields)
			})

			It("removes the email", func() {
				update.UserId = *randomPatient.UserId
				err := repo.UpdateEmail(nil, *randomPatient.UserId, nil)
				Expect(err).ToNot(HaveOccurred())

				var updated patients.Patient
				err = collection.FindOne(nil, primitive.M{
					"userId": randomPatient.UserId,
				}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated.Email).To(BeNil())
			})
		})

		Describe("Remove", func() {
			It("removes the correct patient from the collection", func() {
				err := repo.Remove(nil, randomPatient.ClinicId.Hex(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())

				res := collection.FindOne(nil, bson.M{"$and": []bson.M{{"userId": randomPatient.UserId}, {"clinicId": randomPatient.ClinicId}}})
				Expect(res).ToNot(BeNil())
				Expect(res.Err()).ToNot(BeNil())
				Expect(res.Err()).To(MatchError(mongo.ErrNoDocuments))
				count -= 1
			})
		})

		Describe("Delete from all clinics", func() {
			It("deletes the correct patients", func() {
				// Add the same user to  a different clinic
				patient := patientsTest.RandomPatient()
				patient.UserId = randomPatient.UserId
				_, err := collection.InsertOne(nil, patient)
				Expect(err).ToNot(HaveOccurred())
				count += 1

				err = repo.DeleteFromAllClinics(nil, *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())
				count -= 2

				res, err := collection.CountDocuments(nil, bson.M{})
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))

				selector := bson.M{
					"$or": []bson.M{
						{"$and": []bson.M{{"userId": patient.UserId}, {"clinicId": patient.ClinicId}}},
						{"$and": []bson.M{{"userId": randomPatient.UserId}, {"clinicId": randomPatient.ClinicId}}},
					},
				}
				res, err = collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))
			})
		})

		Describe("Delete tag from all clinic patients", func() {
			It("deletes the correct patient tags", func() {
				newPatientTag := primitive.NewObjectID()
				clinicId := primitive.NewObjectID()

				// Add new patient tag and set common clinic ID for all patients
				for _, patient := range allPatients {
					selector := bson.M{
						"clinicId": patient.ClinicId,
						"userId":   patient.UserId,
					}

					newTags := append(*patient.Tags, newPatientTag)
					update := bson.M{
						"$set": bson.M{
							"tags":     append(newTags, newPatientTag),
							"clinicId": clinicId,
						},
					}

					_, err := collection.UpdateOne(nil, selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// All patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))

				// Perform the delete operation
				err = repo.DeletePatientTagFromClinicPatients(nil, clinicId.Hex(), newPatientTag.Hex(), nil)
				Expect(err).ToNot(HaveOccurred())

				// No patients should have matching tag
				res, err = collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))
			})
		})

		Describe("Delete tag from subset of clinic patients", func() {
			It("deletes the correct patient tag, but only from the specified patients", func() {
				newPatientTag := primitive.NewObjectID()
				clinicId := primitive.NewObjectID()

				// Add new patient tag and set common clinic ID for all patients
				for _, patient := range allPatients {
					selector := bson.M{
						"clinicId": patient.ClinicId,
						"userId":   patient.UserId,
					}

					newTags := append(*patient.Tags, newPatientTag)
					update := bson.M{
						"$set": bson.M{
							"tags":     append(newTags, newPatientTag),
							"clinicId": clinicId,
						},
					}

					_, err := collection.UpdateOne(nil, selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// All patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))

				patientIds := []string{
					*allPatients[0].UserId,
					*allPatients[1].UserId,
				}

				// Perform the delete operation
				err = repo.DeletePatientTagFromClinicPatients(nil, clinicId.Hex(), newPatientTag.Hex(), patientIds)
				Expect(err).ToNot(HaveOccurred())

				// All but 2 patients should have matching tag
				res, err = collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count - 2)))
			})
		})

		Describe("Assign tag to a subset of clinic patients", func() {
			It("assigns the correct patient tag, but only to the specified patients", func() {
				newPatientTag := primitive.NewObjectID()
				clinicId := primitive.NewObjectID()

				// Add set common clinic ID for all patients
				for _, patient := range allPatients {
					selector := bson.M{
						"clinicId": patient.ClinicId,
						"userId":   patient.UserId,
					}

					update := bson.M{
						"$set": bson.M{
							"clinicId": clinicId,
						},
					}

					_, err := collection.UpdateOne(nil, selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// No patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))

				patientIds := []string{
					*allPatients[0].UserId,
					*allPatients[1].UserId,
				}

				// Perform the assign operation
				err = repo.AssignPatientTagToClinicPatients(nil, clinicId.Hex(), newPatientTag.Hex(), patientIds)
				Expect(err).ToNot(HaveOccurred())

				// Two patients should have matching tag
				res, err = collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(2)))
			})
		})

		Describe("Delete non-custodial patients", func() {
			var clinicId primitive.ObjectID
			custodial := make([]interface{}, 5)
			nonCustodial := make([]interface{}, 5)

			BeforeEach(func() {
				clinicId = primitive.NewObjectID()
				perms := patients.Permissions{
					View: &patients.Permission{},
				}
				for i, _ := range custodial {
					patient := patientsTest.RandomPatient()
					patient.ClinicId = &clinicId
					patient.Permissions = &patients.CustodialAccountPermissions
					custodial[i] = patient
				}
				for i, _ := range nonCustodial {
					patient := patientsTest.RandomPatient()
					patient.ClinicId = &clinicId
					patient.Permissions = &perms
					nonCustodial[i] = patient
				}
				_, err := collection.InsertMany(nil, custodial)
				Expect(err).ToNot(HaveOccurred())
				count += len(custodial)

				_, err = collection.InsertMany(nil, nonCustodial)
				Expect(err).ToNot(HaveOccurred())
				count += len(nonCustodial)
			})

			AfterEach(func() {
				res, err := collection.DeleteMany(nil, bson.M{"clinicId": clinicId})
				Expect(err).ToNot(HaveOccurred())
				count -= int(res.DeletedCount)
			})

			It("deletes non-custodial patients", func() {
				err := repo.DeleteNonCustodialPatientsOfClinic(nil, clinicId.Hex())
				Expect(err).ToNot(HaveOccurred())
				count -= len(nonCustodial)

				ids := make([]interface{}, len(nonCustodial))
				for i := range ids {
					patient := nonCustodial[i].(patients.Patient)
					ids[i] = bson.M{"$and": []bson.M{{"userId": patient.UserId}, {"clinicId": patient.ClinicId}}}
				}

				selector := bson.M{"$or": ids}
				res, err := collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))
			})

			It("does not delete custodial patients", func() {
				err := repo.DeleteNonCustodialPatientsOfClinic(nil, clinicId.Hex())
				Expect(err).ToNot(HaveOccurred())
				count -= len(nonCustodial)

				ids := make([]interface{}, len(custodial))
				for i := range ids {
					patient := custodial[i].(patients.Patient)
					ids[i] = bson.M{"$and": []bson.M{{"userId": patient.UserId}, {"clinicId": patient.ClinicId}}}
				}

				selector := bson.M{"$or": ids}
				res, err := collection.CountDocuments(nil, selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(len(ids))))
			})
		})

		Describe("List", func() {
			It("returns all patients", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).To(HaveLen(count))
			})

			It("applies pagination limit correctly", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  2,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).To(HaveLen(2))
			})

			It("applies pagination offset correctly", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  2,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).To(HaveLen(2))

				pagination.Offset = 1
				offsetResults, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(offsetResults.Patients).To(HaveLen(2))
				Expect(*offsetResults.Patients[0]).To(patientFieldsMatcher(*result.Patients[1]))
			})

			It("filters by users id correctly", func() {
				filter := patients.Filter{
					UserId: randomPatient.UserId,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(Equal(randomPatient.UserId))
				}
			})

			It("filters by full name correctly", func() {
				filter := patients.Filter{
					FullName: randomPatient.FullName,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
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
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.ClinicId.Hex()).To(Equal(randomPatient.ClinicId.Hex()))
				}
			})

			It("filters by patient tag correctly", func() {
				randomPatientTags := *randomPatient.Tags
				tags := []string{randomPatientTags[0].Hex()}
				filter := patients.Filter{
					Tags: &tags,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					patientTags := *patient.Tags
					Expect(patientTags).To(ContainElement(randomPatientTags[0]))
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
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				found := false
				for _, patient := range result.Patients {
					if *patient.UserId == *randomPatient.UserId && patient.ClinicId.Hex() == randomPatient.ClinicId.Hex() {
						found = true
						break
					}
				}

				Expect(found).To(BeTrue())
			})

			It("supports searching by patient name", func() {
				clinicId := randomPatient.ClinicId.Hex()
				names := strings.Split(*randomPatient.FullName, " ")
				filter := patients.Filter{
					ClinicId: &clinicId,
					Search:   &names[0],
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				found := false
				for _, patient := range result.Patients {
					if *patient.UserId == *randomPatient.UserId && patient.ClinicId.Hex() == randomPatient.ClinicId.Hex() {
						found = true
						break
					}
				}

				Expect(found).To(BeTrue())
			})

			It("supports searching by patient email", func() {
				clinicId := randomPatient.ClinicId.Hex()
				filter := patients.Filter{
					ClinicId: &clinicId,
					Search:   randomPatient.Email,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				found := false
				for _, patient := range result.Patients {
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
		"Id":                             PointTo(Not(BeEmpty())),
		"UserId":                         PointTo(Equal(*patient.UserId)),
		"ClinicId":                       PointTo(Equal(*patient.ClinicId)),
		"BirthDate":                      PointTo(Equal(*patient.BirthDate)),
		"Email":                          PointTo(Equal(*patient.Email)),
		"FullName":                       PointTo(Equal(*patient.FullName)),
		"Mrn":                            PointTo(Equal(*patient.Mrn)),
		"Tags":                           PointTo(Equal(*patient.Tags)),
		"TargetDevices":                  PointTo(Equal(*patient.TargetDevices)),
		"Permissions":                    PointTo(Equal(*patient.Permissions)),
		"IsMigrated":                     Equal(patient.IsMigrated),
		"LegacyClinicianIds":             ConsistOf(patient.LegacyClinicianIds),
		"UpdatedTime":                    Ignore(),
		"CreatedTime":                    Ignore(),
		"InvitedBy":                      Ignore(),
		"Summary":                        Ignore(),
		"LastUploadReminderTime":         Equal(patient.LastUploadReminderTime),
		"LastRequestedDexcomConnectTime": Equal(patient.LastRequestedDexcomConnectTime),
		"DataSources":                    PointTo(Equal(*patient.DataSources)),
		"RequireUniqueMrn":               Equal(patient.RequireUniqueMrn),
		"EHRSubscriptions":               Equal(patient.EHRSubscriptions),
	})
}
