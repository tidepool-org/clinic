package patients_test

import (
	"context"
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"strings"
	"time"
)

var DemoPatientId = "demo"

var _ = Describe("Patients Repository", func() {
	var cfg *config.Config
	var repo patients.Repository
	var database *mongo.Database
	var collection *mongo.Collection

	BeforeEach(func() {
		var err error
		cfg = &config.Config{ClinicDemoPatientUserId: DemoPatientId}
		database = dbTest.GetTestDatabase()
		collection = database.Collection("patients")
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repo, err = patients.NewRepository(cfg, database, zap.NewNop().Sugar(), lifecycle)
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
			for i := range documents {
				patient := patientsTest.RandomPatient()
				documents[i] = patient
				allPatients[i] = patient
			}
			result, err := collection.InsertMany(context.Background(), documents)
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
			result, err := collection.DeleteMany(context.Background(), selector)
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
				result, err := collection.DeleteOne(context.Background(), selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(result.DeletedCount)).To(Equal(1))
			})

			It("returns the created patient", func() {
				result, err := repo.Create(context.Background(), patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				Expect(*result).To(matchPatientFields)
			})

			It("inserts the patient in the collection", func() {
				result, err := repo.Create(context.Background(), patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				var inserted patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{"_id": result.Id}).Decode(&inserted)
				Expect(err).ToNot(HaveOccurred())
				Expect(inserted).To(matchPatientFields)
			})

			It("successfully inserts a patient with duplicate mrn if uniqueness is not enabled", func() {
				patient.RequireUniqueMrn = false
				result, err := repo.Create(context.Background(), patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.Mrn = patient.Mrn
				secondPatient.RequireUniqueMrn = false

				result, err = repo.Create(context.Background(), secondPatient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				_, err = collection.DeleteOne(context.Background(), primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("successfully inserts multiple patients without mrns when uniqueness is enabled", func() {
				patient.Mrn = nil
				patient.RequireUniqueMrn = true
				result, err := repo.Create(context.Background(), patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.Mrn = nil
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(context.Background(), secondPatient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				_, err = collection.DeleteOne(context.Background(), primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error when a user with duplicate mrn is created", func() {
				patient.RequireUniqueMrn = true
				result, err := repo.Create(context.Background(), patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.Mrn = patient.Mrn
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(context.Background(), secondPatient)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("returns an error when a patient is updated with a duplicated mrn", func() {
				patient.RequireUniqueMrn = true
				result, err := repo.Create(context.Background(), patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(context.Background(), secondPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				result.Mrn = patient.Mrn
				updated, err := repo.Update(context.Background(), patients.PatientUpdate{
					ClinicId:  result.ClinicId.Hex(),
					UserId:    *result.UserId,
					Patient:   *result,
				})
				Expect(err).To(HaveOccurred())
				Expect(updated).To(BeNil())

				_, err = collection.DeleteOne(context.Background(), primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("successfully updates the mrn if it is not a duplicate", func() {
				patient.RequireUniqueMrn = true
				result, err := repo.Create(context.Background(), patient)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				patient.Id = result.Id

				secondPatient := patientsTest.RandomPatient()
				secondPatient.ClinicId = patient.ClinicId
				secondPatient.RequireUniqueMrn = true

				result, err = repo.Create(context.Background(), secondPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				result.Mrn = patientsTest.RandomPatient().Mrn
				result, err = repo.Update(context.Background(), patients.PatientUpdate{
					ClinicId:  result.ClinicId.Hex(),
					UserId:    *result.UserId,
					Patient:   *result,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				_, err = collection.DeleteOne(context.Background(), primitive.M{"_id": result.Id})
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates legacy clinician ids if the patient exists already", func() {
				result, err := repo.Create(context.Background(), patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				patient.Id = result.Id
				patient.LegacyClinicianIds = []string{test.Faker.UUID().V4()}
				matchPatientFields = patientFieldsMatcher(patient)

				result, err = repo.Create(context.Background(), patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var inserted patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{"_id": result.Id}).Decode(&inserted)
				Expect(err).ToNot(HaveOccurred())
				Expect(inserted).To(matchPatientFields)
			})

			It("to fail if patient exists already and legacy clinician ids is not set", func() {
				result, err := repo.Create(context.Background(), patient)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				patient.Id = result.Id

				result, err = repo.Create(context.Background(), patient)
				Expect(err).To(Equal(patients.ErrDuplicatePatient))
				Expect(result).To(BeNil())
			})
		})

		Describe("Get", func() {
			It("returns the correct patient", func() {
				result, err := repo.Get(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId)

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
					Id:               randomPatient.Id,
					ClinicId:         randomPatient.ClinicId,
					UserId:           randomPatient.UserId,
					BirthDate:        update.Patient.BirthDate,
					Email:            update.Patient.Email,
					FullName:         update.Patient.FullName,
					Mrn:              update.Patient.Mrn,
					Tags:             update.Patient.Tags,
					TargetDevices:    update.Patient.TargetDevices,
					Permissions:      update.Patient.Permissions,
					IsMigrated:       randomPatient.IsMigrated,
					DataSources:      update.Patient.DataSources,
					EHRSubscriptions: update.Patient.EHRSubscriptions,
				}
				matchPatientFields = patientFieldsMatcher(expected)
			})

			It("updates the patient in the collection", func() {
				update.ClinicId = randomPatient.ClinicId.Hex()
				update.UserId = *randomPatient.UserId
				result, err := repo.Update(context.Background(), update)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var updated patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{"_id": result.Id}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(matchPatientFields)
			})

			It("returns the updated patient", func() {
				update.ClinicId = randomPatient.ClinicId.Hex()
				update.UserId = *randomPatient.UserId
				result, err := repo.Update(context.Background(), update)
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
					Id:               randomPatient.Id,
					ClinicId:         randomPatient.ClinicId,
					UserId:           randomPatient.UserId,
					BirthDate:        randomPatient.BirthDate,
					Email:            update.Patient.Email,
					FullName:         randomPatient.FullName,
					Mrn:              randomPatient.Mrn,
					Tags:             randomPatient.Tags,
					TargetDevices:    randomPatient.TargetDevices,
					Permissions:      randomPatient.Permissions,
					IsMigrated:       randomPatient.IsMigrated,
					DataSources:      randomPatient.DataSources,
					EHRSubscriptions: randomPatient.EHRSubscriptions,
				}
				matchPatientFields = patientFieldsMatcher(expected)
			})

			It("updates the email", func() {
				update.UserId = *randomPatient.UserId
				err := repo.UpdateEmail(context.Background(), *randomPatient.UserId, update.Patient.Email)
				Expect(err).ToNot(HaveOccurred())

				var updated patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{
					"userId": randomPatient.UserId,
				}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(matchPatientFields)
			})

			It("removes the email", func() {
				update.UserId = *randomPatient.UserId
				err := repo.UpdateEmail(context.Background(), *randomPatient.UserId, nil)
				Expect(err).ToNot(HaveOccurred())

				var updated patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{
					"userId": randomPatient.UserId,
				}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated.Email).To(BeNil())
			})
		})

		Describe("Remove", func() {
			It("removes the correct patient from the collection", func() {
				err := repo.Remove(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, nil)
				Expect(err).ToNot(HaveOccurred())

				res := collection.FindOne(context.Background(), bson.M{"$and": []bson.M{{"userId": randomPatient.UserId}, {"clinicId": randomPatient.ClinicId}}})
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
				_, err := collection.InsertOne(context.Background(), patient)
				Expect(err).ToNot(HaveOccurred())
				count += 1

				clinicIds, err := repo.DeleteFromAllClinics(context.Background(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())
				Expect(clinicIds).To(ConsistOf(randomPatient.ClinicId.Hex(), patient.ClinicId.Hex()))
				count -= 2

				res, err := collection.CountDocuments(context.Background(), bson.M{})
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))

				selector := bson.M{
					"$or": []bson.M{
						{"$and": []bson.M{{"userId": patient.UserId}, {"clinicId": patient.ClinicId}}},
						{"$and": []bson.M{{"userId": randomPatient.UserId}, {"clinicId": randomPatient.ClinicId}}},
					},
				}
				res, err = collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))
			})

			It("deletes no patients", func() {
				unusedUserId := *patientsTest.RandomPatient().UserId

				clinicIds, err := repo.DeleteFromAllClinics(context.Background(), unusedUserId)
				Expect(err).ToNot(HaveOccurred())
				Expect(clinicIds).To(BeEmpty())

				res, err := collection.CountDocuments(context.Background(), bson.M{})
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))
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

					_, err := collection.UpdateOne(context.Background(), selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// All patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))

				// Perform the delete operation
				err = repo.DeletePatientTagFromClinicPatients(context.Background(), clinicId.Hex(), newPatientTag.Hex(), nil)
				Expect(err).ToNot(HaveOccurred())

				// No patients should have matching tag
				res, err = collection.CountDocuments(context.Background(), selector)
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

					_, err := collection.UpdateOne(context.Background(), selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// All patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count)))

				patientIds := []string{
					*allPatients[0].UserId,
					*allPatients[1].UserId,
				}

				// Perform the delete operation
				err = repo.DeletePatientTagFromClinicPatients(context.Background(), clinicId.Hex(), newPatientTag.Hex(), patientIds)
				Expect(err).ToNot(HaveOccurred())

				// All but 2 patients should have matching tag
				res, err = collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(count - 2)))
			})
		})

		Describe("Assign tag to a subset of clinic patients", func() {
			It("assigns the correct patient tag, but only to the specified patients", func() {
				newPatientTag := primitive.NewObjectID()
				clinicId := primitive.NewObjectID()

				// Сet common clinic ID for all patients
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

					_, err := collection.UpdateOne(context.Background(), selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// No patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))

				patientIds := []string{
					*allPatients[0].UserId,
					*allPatients[1].UserId,
				}

				// Perform the assign operation
				err = repo.AssignPatientTagToClinicPatients(context.Background(), clinicId.Hex(), newPatientTag.Hex(), patientIds)
				Expect(err).ToNot(HaveOccurred())

				// Two patients should have matching tag
				res, err = collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(2)))
			})

			It("assigns the correct patient tag to all patients", func() {
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

					_, err := collection.UpdateOne(context.Background(), selector, update)
					Expect(err).ToNot(HaveOccurred())
				}

				selector := bson.M{
					"tags": newPatientTag,
				}

				// No patients should be returned when querying for the new tag
				res, err := collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))

				// Nil array will apply the tag to all patients
				var patientIds []string

				// Perform the assign operation
				err = repo.AssignPatientTagToClinicPatients(context.Background(), clinicId.Hex(), newPatientTag.Hex(), patientIds)
				Expect(err).ToNot(HaveOccurred())

				// All patients should have matching tag
				res, err = collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(len(allPatients))))
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
				for i := range custodial {
					patient := patientsTest.RandomPatient()
					patient.ClinicId = &clinicId
					patient.Permissions = &patients.CustodialAccountPermissions
					custodial[i] = patient
				}
				for i := range nonCustodial {
					patient := patientsTest.RandomPatient()
					patient.ClinicId = &clinicId
					patient.Permissions = &perms
					nonCustodial[i] = patient
				}
				_, err := collection.InsertMany(context.Background(), custodial)
				Expect(err).ToNot(HaveOccurred())
				count += len(custodial)

				_, err = collection.InsertMany(context.Background(), nonCustodial)
				Expect(err).ToNot(HaveOccurred())
				count += len(nonCustodial)
			})

			AfterEach(func() {
				res, err := collection.DeleteMany(context.Background(), bson.M{"clinicId": clinicId})
				Expect(err).ToNot(HaveOccurred())
				count -= int(res.DeletedCount)
			})

			It("deletes non-custodial patients", func() {
				deleted, err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId.Hex())
				Expect(err).ToNot(HaveOccurred())
				Expect(deleted).To(BeTrue())
				count -= len(nonCustodial)

				ids := make([]interface{}, len(nonCustodial))
				for i := range ids {
					patient := nonCustodial[i].(patients.Patient)
					ids[i] = bson.M{"$and": []bson.M{{"userId": patient.UserId}, {"clinicId": patient.ClinicId}}}
				}

				selector := bson.M{"$or": ids}
				res, err := collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(0)))
			})

			It("does not delete custodial patients", func() {
				deleted, err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId.Hex())
				Expect(err).ToNot(HaveOccurred())
				Expect(deleted).To(BeTrue())
				count -= len(nonCustodial)

				ids := make([]interface{}, len(custodial))
				for i := range ids {
					patient := custodial[i].(patients.Patient)
					ids[i] = bson.M{"$and": []bson.M{{"userId": patient.UserId}, {"clinicId": patient.ClinicId}}}
				}

				selector := bson.M{"$or": ids}
				res, err := collection.CountDocuments(context.Background(), selector)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(len(ids))))
			})

			It("does not delete any patients for other clinic id", func() {
				otherClinicId := primitive.NewObjectID()

				deleted, err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), otherClinicId.Hex())
				Expect(err).ToNot(HaveOccurred())
				Expect(deleted).To(BeFalse())

				res, err := collection.CountDocuments(context.Background(), bson.M{"clinicId": clinicId})
				Expect(err).To(BeNil())
				Expect(res).To(Equal(int64(len(custodial) + len(nonCustodial))))
			})
		})

		Describe("Count", func() {
			It("returns the expected count with no filter", func() {
				filter := &patients.Filter{}
				count, err := repo.Count(context.Background(), filter)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(10))
			})

			It("returns the expected count with a user id filter", func() {
				filter := &patients.Filter{
					UserId: allPatients[0].UserId,
				}
				count, err := repo.Count(context.Background(), filter)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(1))
			})

			It("returns the expected count with an exclude demo filter", func() {
				filter := &patients.Filter{
					ExcludeDemo: true,
				}
				count, err := repo.Count(context.Background(), filter)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(10))
			})

			When("there is a demo patient", func() {
				var demoPatient patients.Patient

				BeforeEach(func() {
					demoPatient = patientsTest.RandomPatient()
					demoPatient.UserId = &DemoPatientId
					result, err := collection.InsertOne(context.Background(), demoPatient)
					Expect(err).ToNot(HaveOccurred())
					id := result.InsertedID.(primitive.ObjectID)
					demoPatient.Id = &id
				})

				AfterEach(func() {
					selector := primitive.M{
						"_id": demoPatient.Id,
					}
					result, err := collection.DeleteOne(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(int(result.DeletedCount)).To(Equal(1))
				})

				It("returns the expected count with no filter", func() {
					filter := &patients.Filter{}
					count, err := repo.Count(context.Background(), filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(11))
				})

				It("returns the expected count with a user id filter", func() {
					filter := &patients.Filter{
						UserId: demoPatient.UserId,
					}
					count, err := repo.Count(context.Background(), filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(1))
				})

				It("returns the expected count with an exclude demo filter", func() {
					filter := &patients.Filter{
						ExcludeDemo: true,
					}
					count, err := repo.Count(context.Background(), filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(10))
				})

				It("returns the expected count with a user id and an exclude demo filter", func() {
					filter := &patients.Filter{
						UserId:      allPatients[0].UserId,
						ExcludeDemo: true,
					}
					count, err := repo.Count(context.Background(), filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(1))
				})

				It("returns the expected count with the demo user id and an exclude demo filter", func() {
					filter := &patients.Filter{
						UserId:      demoPatient.UserId,
						ExcludeDemo: true,
					}
					count, err := repo.Count(context.Background(), filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(0))
				})
			})
		})

		Describe("List", func() {
			It("returns all patients", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(context.Background(), &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).To(HaveLen(count))
			})

			It("applies pagination limit correctly", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  2,
				}
				result, err := repo.List(context.Background(), &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).To(HaveLen(2))
			})

			It("applies pagination offset correctly", func() {
				filter := patients.Filter{}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  2,
				}
				result, err := repo.List(context.Background(), &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).To(HaveLen(2))

				pagination.Offset = 1
				offsetResults, err := repo.List(context.Background(), &filter, pagination, nil)
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(Equal(randomPatient.UserId))
				}
			})

			It("filters by mrn correctly", func() {
				filter := patients.Filter{
					Mrn: randomPatient.Mrn,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.Mrn).To(PointTo(Equal(*randomPatient.Mrn)))
				}
			})

			It("filters users without MRN correctly when hasMRN=false", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$unset": bson.M{"mrn": 1}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasMRN := false
				filter := patients.Filter{
					HasMRN: &hasMRN,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with empty MRN correctly when hasMRN=false", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"mrn": ""}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasMRN := false
				filter := patients.Filter{
					HasMRN: &hasMRN,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with null MRN correctly when hasMRN=false", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"mrn": nil}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasMRN := false
				filter := patients.Filter{
					HasMRN: &hasMRN,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users without MRN correctly when hasMRN=true", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$unset": bson.M{"mrn": 1}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasMRN := true
				filter := patients.Filter{
					HasMRN: &hasMRN,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).ToNot(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with empty MRN correctly when hasMRN=true", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"mrn": ""}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasMRN := true
				filter := patients.Filter{
					HasMRN: &hasMRN,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).ToNot(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with null MRN correctly when hasMRN=true", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"mrn": nil}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasMRN := true
				filter := patients.Filter{
					HasMRN: &hasMRN,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).ToNot(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users without EHR subscriptions correctly when HasSubscription=false", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$unset": bson.M{"ehrSubscriptions": 1}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasSubscriptions := false
				filter := patients.Filter{
					HasSubscription: &hasSubscriptions,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with empty EHR subscriptions correctly when HasSubscription=false", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"ehrSubscriptions": bson.M{}}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasSubscriptions := false
				filter := patients.Filter{
					HasSubscription: &hasSubscriptions,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with null EHR subscriptions correctly when HasSubscription=false", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"ehrSubscriptions": nil}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasSubscriptions := false
				filter := patients.Filter{
					HasSubscription: &hasSubscriptions,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).To(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users without EHR subscriptions correctly when HasSubscription=true", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$unset": bson.M{"ehrSubscriptions": 1}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasSubscriptions := true
				filter := patients.Filter{
					HasSubscription: &hasSubscriptions,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).ToNot(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with empty EHR subscriptions correctly when HasSubscription=true", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"ehrSubscriptions": bson.M{}}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasSubscriptions := true
				filter := patients.Filter{
					HasSubscription: &hasSubscriptions,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).ToNot(PointTo(Equal(*randomPatient.UserId)))
				}
			})

			It("filters users with null EHR subscriptions correctly when HasSubscription=true", func() {
				_, err := collection.UpdateOne(
					nil,
					bson.M{"userId": randomPatient.UserId},
					bson.M{"$set": bson.M{"ehrSubscriptions": nil}},
				)
				Expect(err).ToNot(HaveOccurred())

				hasSubscriptions := true
				filter := patients.Filter{
					HasSubscription: &hasSubscriptions,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(nil, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.UserId).ToNot(PointTo(Equal(*randomPatient.UserId)))
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(patient.ClinicId.Hex()).To(Equal(randomPatient.ClinicId.Hex()))
				}
			})

			It("filters by clinic ids correctly", func() {
				clinicIds := []string{
					allPatients[1].ClinicId.Hex(),
					allPatients[2].ClinicId.Hex(),
				}
				clinicIdsMap := map[string]struct{}{}
				for _, id := range clinicIds {
					clinicIdsMap[id] = struct{}{}
				}

				filter := patients.Filter{
					ClinicIds: clinicIds,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  10,
				}
				result, err := repo.List(context.Background(), &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					Expect(clinicIdsMap).To(HaveKey(patient.ClinicId.Hex()))
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
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
				result, err := repo.List(context.Background(), &filter, pagination, nil)
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
				result, err := repo.UpdatePermissions(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, &permissions)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var updated patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{"_id": result.Id}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(*updated.Permissions).To(Equal(permissions))
			})

			It("returns the updated permissions", func() {
				permissions := patientsTest.RandomPermissions()
				randomPatient.Permissions = &permissions
				matchPatientFields = patientFieldsMatcher(randomPatient)

				result, err := repo.UpdatePermissions(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, &permissions)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(*result).To(matchPatientFields)
			})
		})

		Describe("Delete Permissions", func() {
			It("removes the permission from the patient record", func() {
				// make sure all permissions are set
				_, err := repo.UpdatePermissions(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, &patients.CustodialAccountPermissions)
				Expect(err).ToNot(HaveOccurred())

				permission := patientsTest.RandomPermission()
				result, err := repo.DeletePermission(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, permission)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				path := fmt.Sprintf("permissions.%s", permission)
				res := collection.FindOne(context.Background(), primitive.M{"_id": result.Id, path: primitive.M{"$exists": "true"}})
				Expect(res).ToNot(BeNil())
				Expect(res.Err()).To(MatchError(mongo.ErrNoDocuments))
			})

			It("returns an error if a permissions is not set", func() {
				// make sure all permissions are set
				_, err := repo.UpdatePermissions(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, &patients.CustodialAccountPermissions)
				Expect(err).ToNot(HaveOccurred())

				permission := patientsTest.RandomPermission()
				result, err := repo.DeletePermission(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, permission)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				result, err = repo.DeletePermission(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, permission)
				Expect(err).To(MatchError(patients.ErrPermissionNotFound))
				Expect(result).To(BeNil())
			})
		})

		Describe("Add Reviews", func() {
			It("correctly adds review", func() {
				clinicianId := test.Faker.UUID().V4()
				ts := time.Now().UTC().Truncate(time.Millisecond)

				review := patients.Review{
					ClinicianId: clinicianId,
					Time:        ts,
				}

				reviews, err := repo.AddReview(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, review)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(reviews)).To(Equal(1))
				Expect(reviews[0]).To(BeComparableTo(review))
			})

			It("correctly adds multiple reviews", func() {
				limit := 2
				clinicianId := test.Faker.UUID().V4()
				ts := time.Now().UTC().Truncate(time.Millisecond)

				reviews := make([]patients.Review, limit+1)
				for i := 0; i < len(reviews); i++ {
					By(fmt.Sprintf("inserting review %d", i+1))
					reviews[i] = patients.Review{
						ClinicianId: clinicianId,
						Time:        ts.Add(time.Hour * time.Duration(i)),
					}

					results, err := repo.AddReview(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, reviews[i])
					Expect(err).ToNot(HaveOccurred())
					Expect(results[0]).To(BeComparableTo(reviews[i]))

					if i+1 >= limit {
						Expect(len(results)).To(Equal(limit))
					} else {
						Expect(len(results)).To(Equal(i + 1))
					}
				}
			})

			Describe("Delete Reviews", func() {
				It("correctly deletes review", func() {
					clinicianId := test.Faker.UUID().V4()
					ts := time.Now().UTC().Truncate(time.Millisecond)

					reviews := make([]patients.Review, 2)
					for i := 0; i < len(reviews); i++ {
						By(fmt.Sprintf("inserting review %d", i+1))
						reviews[i] = patients.Review{
							ClinicianId: clinicianId,
							Time:        ts.Add(time.Hour * time.Duration(i)),
						}

						_, err := repo.AddReview(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, reviews[i])
						Expect(err).ToNot(HaveOccurred())
					}

					results, err := repo.DeleteReview(context.Background(), randomPatient.ClinicId.Hex(), clinicianId, *randomPatient.UserId)
					Expect(err).ToNot(HaveOccurred())
					Expect(len(results)).To(Equal(1))
					Expect(results[0]).To(Equal(reviews[0]))
				})

				It("correctly fails to delete non-owner review", func() {
					clinicianId := test.Faker.UUID().V4()
					ts := time.Now().UTC().Truncate(time.Millisecond)

					review := patients.Review{
						ClinicianId: clinicianId,
						Time:        ts,
					}

					results, err := repo.AddReview(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, review)
					Expect(err).ToNot(HaveOccurred())
					Expect(len(results)).To(Equal(1))

					results, err = repo.DeleteReview(context.Background(), randomPatient.ClinicId.Hex(), "nobody", *randomPatient.UserId)
					Expect(err).To(HaveOccurred())
					Expect(errors.Is(err, patients.ErrReviewNotOwner)).To(BeTrue())
				})
			})
		})

		Describe("Add provider connection request", func() {
			BeforeEach(func() {
				dataSources := patients.DataSources{{
					ProviderName: patients.DexcomDataSourceProviderName,
					State:        "pending",
				}}
				err := repo.UpdatePatientDataSources(context.Background(), *randomPatient.UserId, &dataSources)
				Expect(err).ToNot(HaveOccurred())
			})

			It("correctly updates the datasource to pending reconnect when the data source already exists", func() {
				id := primitive.NewObjectID()
				modifiedTime := time.Now()
				dataSources := patients.DataSources{{
					DataSourceId: &id,
					ModifiedTime: &modifiedTime,
					ProviderName: patients.DexcomDataSourceProviderName,
					State:        "pending",
				}}
				err := repo.UpdatePatientDataSources(context.Background(), *randomPatient.UserId, &dataSources)
				Expect(err).ToNot(HaveOccurred())

				request := patients.ConnectionRequest{
					ProviderName: patients.DexcomDataSourceProviderName,
					CreatedTime:  time.Now().UTC().Truncate(time.Millisecond),
				}

				err = repo.AddProviderConnectionRequest(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, request)
				Expect(err).ToNot(HaveOccurred())

				patient, err := repo.Get(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())
				Expect(patient).ToNot(BeNil())
				Expect(patient.DataSources).ToNot(BeNil())
				Expect(*patient.DataSources).To(HaveLen(1))
				Expect((*patient.DataSources)[0].State).To(Equal("pendingReconnect"))
			})

			It("correctly adds multiple requests", func() {
				request := patients.ConnectionRequest{
					ProviderName: patients.DexcomDataSourceProviderName,
					CreatedTime:  time.Now().Truncate(time.Millisecond),
				}

				err := repo.AddProviderConnectionRequest(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, request)
				Expect(err).ToNot(HaveOccurred())

				err = repo.AddProviderConnectionRequest(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, request)
				Expect(err).ToNot(HaveOccurred())

				patient, err := repo.Get(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())

				Expect(patient).ToNot(BeNil())
				Expect(patient.ProviderConnectionRequests).To(HaveKey("dexcom"))

				dexcom := patient.ProviderConnectionRequests["dexcom"]
				Expect(dexcom).To(HaveLen(2))
				Expect(dexcom[0]).To(BeComparableTo(request))
				Expect(dexcom[1]).To(BeComparableTo(request))
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
		"Reviews":                        Ignore(),
		"ProviderConnectionRequests":     Equal(patient.ProviderConnectionRequests),
		"LastUploadReminderTime":         Equal(patient.LastUploadReminderTime),
		"LastRequestedDexcomConnectTime": Equal(patient.LastRequestedDexcomConnectTime),
		"DataSources":                    PointTo(Equal(*patient.DataSources)),
		"RequireUniqueMrn":               Equal(patient.RequireUniqueMrn),
		"EHRSubscriptions":               Equal(patient.EHRSubscriptions),
	})
}
