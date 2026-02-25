package repository_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/onsi/gomega/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	sitesTest "github.com/tidepool-org/clinic/sites/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
)

var DemoPatientId = "demo"

var _ = Describe("Patients Repository", func() {
	var cfg *config.Config
	var repo patients.Repository
	var database *mongo.Database
	var collection *mongo.Collection
	var deletionsCollection *mongo.Collection

	BeforeEach(func() {
		var err error
		cfg = &config.Config{ClinicDemoPatientUserId: DemoPatientId}
		database = dbTest.GetTestDatabase()
		collection = database.Collection("patients")
		deletionsCollection = database.Collection("patient_deletions")
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repo, err = patientsRepository.NewRepository(cfg, database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	Context("with random data", func() {
		var clinicId primitive.ObjectID
		var clinicIdString string
		var allPatientIds []interface{}
		var allPatients []patients.Patient
		var randomPatient patients.Patient
		var matchPatientFields types.GomegaMatcher
		var count int

		BeforeEach(func() {
			clinicId = primitive.NewObjectID()
			clinicIdString = clinicId.Hex()
			count = 10
			documents := make([]interface{}, count)
			allPatients = make([]patients.Patient, count)
			for i := range documents {
				patient := patientsTest.RandomPatient()
				if i%2 == 0 {
					patient.ClinicId = &clinicId
				}
				if i%3 == 0 {
					patient.DataSources = &[]patients.DataSource{{ProviderName: "twiist", State: "disconnected"}}
				}
				documents[i] = patient
				allPatients[i] = patient
			}
			result, err := collection.InsertMany(context.Background(), documents)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.InsertedIDs).To(HaveLen(count))
			allPatientIds = result.InsertedIDs

			randomPatient = documents[test.Faker.IntBetween(0, count-1)].(patients.Patient)
			matchPatientFields = patientsTest.PatientFieldsMatcher(randomPatient)
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

			_, err = deletionsCollection.DeleteMany(context.Background(), bson.M{})
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("Create", func() {
			var patient patients.Patient

			BeforeEach(func() {
				patient = patientsTest.RandomPatient()
				matchPatientFields = patientsTest.PatientFieldsMatcher(patient)
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
					ClinicId: result.ClinicId.Hex(),
					UserId:   *result.UserId,
					Patient:  *result,
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
					ClinicId: result.ClinicId.Hex(),
					UserId:   *result.UserId,
					Patient:  *result,
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
				matchPatientFields = patientsTest.PatientFieldsMatcher(patient)

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

			It("stores the diagnosis type", func() {
				ctx := context.Background()
				result, err := repo.Create(ctx, patient)
				patient.Id = result.Id
				Expect(err).To(Succeed())

				got, err := repo.Get(ctx, patient.ClinicId.Hex(), *patient.UserId)
				Expect(err).To(Succeed())
				Expect(got.DiagnosisType).To(Equal(patient.DiagnosisType))
			})

			It("stores preset glycemic ranges", func() {
				patient.GlycemicRanges = patientsTest.RandomGlycemicRangesPreset()
				ctx := context.Background()
				result, err := repo.Create(ctx, patient)
				patient.Id = result.Id
				Expect(err).To(Succeed())

				got, err := repo.Get(ctx, patient.ClinicId.Hex(), *patient.UserId)
				Expect(err).To(Succeed())
				Expect(got.GlycemicRanges).To(Equal(patient.GlycemicRanges))
			})

			It("stores custom glycemic ranges", func() {
				patient.GlycemicRanges = patientsTest.RandomGlycemicRangesCustom()
				ctx := context.Background()
				result, err := repo.Create(ctx, patient)
				patient.Id = result.Id
				Expect(err).To(Succeed())

				got, err := repo.Get(ctx, patient.ClinicId.Hex(), *patient.UserId)
				Expect(err).To(Succeed())
				Expect(got.GlycemicRanges).To(Equal(patient.GlycemicRanges))
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

		Describe("ClinicIds", func() {
			var sameUserDifferentClinicPatient patients.Patient
			var differentUserSameClinicPatient patients.Patient

			BeforeEach(func() {
				sameUserDifferentClinicPatient = patientsTest.RandomPatient()
				sameUserDifferentClinicPatient.UserId = randomPatient.UserId
				result, err := collection.InsertOne(context.Background(), sameUserDifferentClinicPatient)
				Expect(err).ToNot(HaveOccurred())
				sameUserId := result.InsertedID.(primitive.ObjectID)
				sameUserDifferentClinicPatient.Id = &sameUserId

				differentUserSameClinicPatient = patientsTest.RandomPatient()
				differentUserSameClinicPatient.ClinicId = randomPatient.ClinicId
				result, err = collection.InsertOne(context.Background(), differentUserSameClinicPatient)
				Expect(err).ToNot(HaveOccurred())
				differentUserId := result.InsertedID.(primitive.ObjectID)
				differentUserSameClinicPatient.Id = &differentUserId
			})

			AfterEach(func() {
				selector := primitive.M{
					"_id": primitive.M{"$in": []primitive.ObjectID{*sameUserDifferentClinicPatient.Id, *differentUserSameClinicPatient.Id}},
				}
				result, err := collection.DeleteMany(context.Background(), selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(int(result.DeletedCount)).To(Equal(2))
			})

			It("returns only clinic ids associated with the patient", func() {
				result, err := repo.ClinicIds(context.Background(), *randomPatient.UserId)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result).To(ConsistOf(randomPatient.ClinicId.Hex(), sameUserDifferentClinicPatient.ClinicId.Hex()))
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
					Sites:            update.Patient.Sites,
					GlycemicRanges:   update.Patient.GlycemicRanges,
					DiagnosisType:    update.Patient.DiagnosisType,
				}
				matchPatientFields = patientsTest.PatientFieldsMatcher(expected)
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

			It("updates the patient's sites", func() {
				update.Patient = randomPatient
				update.Patient.Sites = &[]sites.Site{{Name: "New York", Id: primitive.NewObjectID()}}
				update.ClinicId = randomPatient.ClinicId.Hex()
				update.UserId = *randomPatient.UserId
				result, err := repo.Update(context.Background(), update)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				var updated patients.Patient
				err = collection.FindOne(context.Background(), primitive.M{"_id": result.Id}).Decode(&updated)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated.Sites).ToNot(BeNil())
				Expect(len(*updated.Sites)).To(Equal(1))
				Expect((*updated.Sites)[0].Name).To(Equal("New York"))
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
					Sites:            randomPatient.Sites,
					GlycemicRanges:   randomPatient.GlycemicRanges,
					DiagnosisType:    randomPatient.DiagnosisType,
				}
				matchPatientFields = patientsTest.PatientFieldsMatcher(expected)
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

		Describe("UpdateEHRSubscriptions", func() {
			It("updates the patient's updatedTime", func() {
				var matchedMessages int
				for _, sub := range randomPatient.EHRSubscriptions {
					matchedMessages += len(sub.MatchedMessages)
				}

				err := repo.UpdateEHRSubscription(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, patients.SubscriptionUpdate{
					Name:     "summaryAndReports",
					Provider: "redox",
					Active:   true,
					MatchedMessage: patients.MatchedMessage{
						DocumentId: primitive.NewObjectID(),
						DataModel:  "Order",
						EventType:  "New",
					},
				})
				Expect(err).ToNot(HaveOccurred())

				updated, err := repo.Get(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated.UpdatedTime).To(BeTemporally(">", randomPatient.UpdatedTime))

				var matchedMessagesAfterUpdate int
				for _, sub := range updated.EHRSubscriptions {
					matchedMessagesAfterUpdate += len(sub.MatchedMessages)
				}
				Expect(matchedMessagesAfterUpdate).To(Equal(matchedMessages + 1))
			})
		})

		Describe("UpdateSummaryInAllClinics", func() {
			It("updates the patient's updatedTime", func() {
				err := repo.UpdateSummaryInAllClinics(context.Background(), *randomPatient.UserId, nil)
				Expect(err).ToNot(HaveOccurred())

				updated, err := repo.Get(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated.UpdatedTime).To(BeTemporally(">", randomPatient.UpdatedTime))
			})
		})

		Describe("Remove", func() {
			It("removes the correct patient from the collection", func() {
				err := repo.Remove(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())

				res := collection.FindOne(context.Background(), bson.M{"$and": []bson.M{{"userId": randomPatient.UserId}, {"clinicId": randomPatient.ClinicId}}})
				Expect(res).ToNot(BeNil())
				Expect(res.Err()).ToNot(BeNil())
				Expect(res.Err()).To(MatchError(mongo.ErrNoDocuments))
				count -= 1
			})

			It("creates a deletion record", func() {
				err := repo.Remove(context.Background(), randomPatient.ClinicId.Hex(), *randomPatient.UserId, deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
				count -= 1

				count, err := deletionsCollection.CountDocuments(context.Background(), bson.M{"$and": []bson.M{{"patient.userId": randomPatient.UserId}, {"patient.clinicId": randomPatient.ClinicId}}})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeNumerically("==", 1))
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

				clinicIds, err := repo.DeleteFromAllClinics(context.Background(), *randomPatient.UserId, deletions.Metadata{})
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

			It("creates deletion records", func() {
				// Add the same user to  a different clinic
				patient := patientsTest.RandomPatient()
				patient.UserId = randomPatient.UserId
				_, err := collection.InsertOne(context.Background(), patient)
				Expect(err).ToNot(HaveOccurred())
				count += 1

				clinicIds, err := repo.DeleteFromAllClinics(context.Background(), *randomPatient.UserId, deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
				Expect(clinicIds).To(ConsistOf(randomPatient.ClinicId.Hex(), patient.ClinicId.Hex()))
				count -= 2

				count, err := deletionsCollection.CountDocuments(context.Background(), bson.M{"$and": []bson.M{{"patient.userId": randomPatient.UserId}}})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeNumerically("==", 2))
			})

			It("deletes no patients", func() {
				unusedUserId := *patientsTest.RandomPatient().UserId

				clinicIds, err := repo.DeleteFromAllClinics(context.Background(), unusedUserId, deletions.Metadata{})
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
				err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId.Hex(), deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
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

			It("deletes non-custodial patients", func() {
				err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId.Hex(), deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
				count -= len(nonCustodial)

				count, err := deletionsCollection.CountDocuments(context.Background(), bson.M{"$and": []bson.M{{"patient.clinicId": clinicId}}})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeNumerically("==", len(nonCustodial)))
			})

			It("does not delete custodial patients", func() {
				err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId.Hex(), deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
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

				err := repo.DeleteNonCustodialPatientsOfClinic(context.Background(), otherClinicId.Hex(), deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())

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

		Describe("Counts", func() {
			It("returns the expected count for the clinic id", func() {
				counts, err := repo.Counts(context.Background(), clinicIdString)
				Expect(err).ToNot(HaveOccurred())
				Expect(counts).ToNot(BeNil())
				Expect(counts.Total).To(Equal(5))
				Expect(counts.Demo).To(Equal(0))
				Expect(counts.Plan).To(Equal(3))
				Expect(counts.Providers).ToNot(BeNil())
				Expect(counts.Providers).To(HaveKeyWithValue("twiist", patients.ProviderCounts{States: map[string]int{"disconnected": 2}, Total: 2}))
			})

			It("returns the expected count for a clinic id with one patient", func() {
				counts, err := repo.Counts(context.Background(), allPatients[1].ClinicId.Hex())
				Expect(err).ToNot(HaveOccurred())
				Expect(counts).ToNot(BeNil())
				Expect(counts.Total).To(Equal(1))
				Expect(counts.Demo).To(Equal(0))
				Expect(counts.Plan).To(Equal(1))
				Expect(counts.Providers).ToNot(BeNil())
				Expect(counts.Providers).ToNot(HaveKey("twiist"))
			})

			It("returns no patients if the clinic id is unknown", func() {
				counts, err := repo.Counts(context.Background(), primitive.NewObjectID().Hex())
				Expect(err).ToNot(HaveOccurred())
				Expect(counts).ToNot(BeNil())
				Expect(counts.Total).To(Equal(0))
				Expect(counts.Demo).To(Equal(0))
				Expect(counts.Plan).To(Equal(0))
				Expect(counts.Providers).To(BeEmpty())
			})

			It("returns an error if the clinic id is invalid", func() {
				counts, err := repo.Counts(context.Background(), "invalid_clinic_id")
				Expect(err).To(HaveOccurred())
				Expect(counts).To(BeNil())
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
				Expect(*offsetResults.Patients[0]).To(patientsTest.PatientFieldsMatcher(*result.Patients[1]))
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

			It("filters multiple tags via AND", func() {
				var err error

				secondPatient := allPatients[len(allPatients)-1]
				secondTag := (*secondPatient.Tags)[0]
				newTags := append(*randomPatient.Tags, secondTag)
				updateFilter := bson.M{
					"clinicId": randomPatient.ClinicId,
					"userId":   randomPatient.UserId,
				}
				update := bson.M{
					"$set": bson.M{
						"tags":     newTags,
						"clinicId": randomPatient.ClinicId,
					},
				}
				_, err = collection.UpdateOne(context.Background(), updateFilter, update)
				Expect(err).ToNot(HaveOccurred())

				ctx := context.Background()
				tags := []string{
					newTags[0].Hex(),
					newTags[1].Hex(),
				}
				filter := patients.Filter{
					Tags: &tags,
				}
				pagination := store.Pagination{Offset: 0, Limit: 100}
				result, err := repo.List(ctx, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					patientTags := *patient.Tags
					Expect(patientTags).To(ConsistOf(newTags[0], newTags[1]))
				}
			})

			It("filters by patient tag correctly", func() {
				ctx := context.Background()
				randomPatientTags := *randomPatient.Tags
				tags := []string{randomPatientTags[0].Hex()}
				filter := patients.Filter{
					Tags: &tags,
				}
				pagination := store.Pagination{
					Offset: 0,
					Limit:  count,
				}
				result, err := repo.List(ctx, &filter, pagination, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Patients).ToNot(HaveLen(0))

				for _, patient := range result.Patients {
					patientTags := *patient.Tags
					Expect(patientTags).To(ContainElement(randomPatientTags[0]))
				}

				// an empty tag returns patients without tags
				noPatientTags := []primitive.ObjectID{}
				randomPatient.Tags = &noPatientTags
				update := patients.PatientUpdate{
					ClinicId: randomPatient.ClinicId.Hex(),
					UserId:   *randomPatient.UserId,
					Patient:  randomPatient,
				}
				got, err := repo.Update(ctx, update)
				Expect(err).To(Succeed())
				noTags := []string{""}
				filter.Tags = &noTags
				result2, err := repo.List(ctx, &filter, pagination, nil)
				Expect(err).To(Succeed())
				Expect(len(result2.Patients)).To(Equal(1))
				result2PatientUserIDs := []string{}
				for _, patient := range result2.Patients {
					result2PatientUserIDs = append(result2PatientUserIDs, *patient.UserId)
				}
				Expect(result2PatientUserIDs).To(ContainElement(*got.UserId))
			})

			It("filters by patient site correctly", func() {
				// non-existent sites match no patients
				ctx := context.Background()
				nonExistentSiteID := primitive.NewObjectID().Hex()
				nonSites := []string{nonExistentSiteID}
				filter := patients.Filter{
					Sites: &nonSites,
				}
				result, err := repo.List(ctx, &filter, store.DefaultPagination(), nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result.Patients)).To(Equal(0))

				// existing sites match no patients
				p := allPatients[0]
				existingSite := sitesTest.Random()
				p.Sites = &[]sites.Site{existingSite}
				update := patients.PatientUpdate{
					ClinicId: p.ClinicId.Hex(),
					UserId:   *p.UserId,
					Patient:  p,
				}
				got, err := repo.Update(ctx, update)
				Expect(err).To(Succeed())
				Expect(got.Sites).ToNot(BeNil())
				Expect(len(*got.Sites)).To(Equal(1))

				(*filter.Sites)[0] = existingSite.Id.Hex()
				result2, err := repo.List(ctx, &filter, store.DefaultPagination(), nil)
				Expect(err).To(Succeed())
				Expect(len(result2.Patients)).To(Equal(1))
				Expect(*result2.Patients[0].UserId).To(Equal(*got.UserId))

				// multiple sites are OR-ed
				newSites := []string{(*filter.Sites)[0], sitesTest.Random().Id.Hex()}
				filter.Sites = &newSites
				result3, err := repo.List(ctx, &filter, store.DefaultPagination(), nil)
				Expect(err).To(Succeed())
				Expect(len(result3.Patients)).To(Equal(1))
				Expect(*result3.Patients[0].UserId).To(Equal(*got.UserId))

				// an empty site returns patients without sites
				noSites := []string{""}
				filter.Sites = &noSites
				result4, err := repo.List(ctx, &filter, store.DefaultPagination(), nil)
				Expect(err).To(Succeed())
				Expect(len(result4.Patients)).To(Equal(len(allPatients) - 1))
				result4PatientUserIDs := []string{}
				for _, patient := range result4.Patients {
					result4PatientUserIDs = append(result4PatientUserIDs, *patient.UserId)
				}
				Expect(result4PatientUserIDs).ToNot(ContainElement(*got.UserId))
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
				matchPatientFields = patientsTest.PatientFieldsMatcher(randomPatient)

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

		Describe("DeleteSites", func() {
			var patientWithSites patients.Patient

			BeforeEach(func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				userId := *randomPatient.UserId
				patientWithSites = randomPatient
				sites := sitesTest.RandomSlice(1)
				patientWithSites.Sites = &sites
				update := patients.PatientUpdate{
					ClinicId: clinicId,
					UserId:   userId,
					Patient:  patientWithSites,
				}
				_, err := repo.Update(ctx, update)
				Expect(err).To(Succeed())
			})

			It("deletes patients' denormalized sites", func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				userId := *patientWithSites.UserId
				site := (*patientWithSites.Sites)[0]
				siteId := site.Id.Hex()

				Expect(repo.DeleteSites(ctx, clinicId, siteId)).To(Succeed())
				got, err := repo.Get(ctx, clinicId, userId)
				Expect(err).To(Succeed())
				Expect(got.Sites).ToNot(BeNil())
				Expect(len(*got.Sites)).To(Equal(0))
			})
		})

		Describe("MergeSites", func() {
			var patientWithSites patients.Patient

			BeforeEach(func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				userId := *randomPatient.UserId
				patientWithSites = randomPatient
				sites := sitesTest.RandomSlice(1)
				patientWithSites.Sites = &sites
				update := patients.PatientUpdate{
					ClinicId: clinicId,
					UserId:   userId,
					Patient:  patientWithSites,
				}
				_, err := repo.Update(ctx, update)
				Expect(err).To(Succeed())
			})

			It("updates patients' sites", func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				userId := *patientWithSites.UserId
				source := (*patientWithSites.Sites)[0]
				sourceId := source.Id.Hex()
				preMerge, err := repo.Get(ctx, clinicId, userId)
				Expect(err).To(Succeed())
				Expect(preMerge.Sites).ToNot(BeNil())
				Expect(len(*preMerge.Sites)).To(Equal(1))
				Expect((*preMerge.Sites)[0].Name).To(Equal(source.Name))
				target := sitesTest.Random()
				target.Name = source.Name + "-target"

				Expect(repo.MergeSites(ctx, clinicId, sourceId, &target)).To(Succeed())
				got, err := repo.Get(ctx, clinicId, userId)
				Expect(err).To(Succeed())
				Expect(got.Sites).ToNot(BeNil())
				Expect(len(*got.Sites)).To(Equal(1))
				Expect((*got.Sites)[0].Name).To(Equal(target.Name))
			})
		})

		Describe("UpdateSites", func() {
			var patientWithSites patients.Patient

			BeforeEach(func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				userId := *randomPatient.UserId
				patientWithSites = randomPatient
				sites := sitesTest.RandomSlice(1)
				patientWithSites.Sites = &sites
				update := patients.PatientUpdate{
					ClinicId: clinicId,
					UserId:   userId,
					Patient:  patientWithSites,
				}
				_, err := repo.Update(ctx, update)
				Expect(err).To(Succeed())
			})

			It("updates patients' denormalized sites", func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				userId := *patientWithSites.UserId
				site := (*patientWithSites.Sites)[0]
				siteId := site.Id.Hex()
				site.Name = site.Name + " test"

				Expect(repo.UpdateSites(ctx, clinicId, siteId, &site)).To(Succeed())
				got, err := repo.Get(ctx, clinicId, userId)
				Expect(err).To(Succeed())
				Expect(got.Sites).ToNot(BeNil())
				Expect(len(*got.Sites)).To(Equal(1))
				Expect((*got.Sites)[0].Name).To(Equal(site.Name))
			})
		})

		Describe("ConvertPatientTagToSite", func() {
			It("works", func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				Expect(randomPatient.Tags != nil).To(BeTrue())
				Expect(len(*randomPatient.Tags) > 0).To(BeTrue())
				tagID := (*randomPatient.Tags)[0]
				site := sitesTest.Random()

				err := repo.ConvertPatientTagToSite(ctx, clinicId, tagID.Hex(), &site)
				Expect(err).To(Succeed())
			})

			It("removes the tag", func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				Expect(randomPatient.Tags != nil).To(BeTrue())
				Expect(len(*randomPatient.Tags) > 0).To(BeTrue())
				tagID := (*randomPatient.Tags)[0]
				site := sitesTest.Random()
				err := repo.ConvertPatientTagToSite(ctx, clinicId, tagID.Hex(), &site)
				Expect(err).To(Succeed())

				patient, err := repo.Get(ctx, clinicId, *randomPatient.UserId)
				Expect(err).To(Succeed())
				Expect(slices.ContainsFunc(*patient.Tags, func(id primitive.ObjectID) bool {
					return tagID == id
				})).ToNot(BeTrue())
			})

			It("adds the site", func() {
				ctx := context.Background()
				clinicId := randomPatient.ClinicId.Hex()
				Expect(randomPatient.Tags != nil).To(BeTrue())
				Expect(len(*randomPatient.Tags) > 0).To(BeTrue())
				tagID := (*randomPatient.Tags)[0]
				site := sitesTest.Random()
				err := repo.ConvertPatientTagToSite(ctx, clinicId, tagID.Hex(), &site)
				Expect(err).To(Succeed())

				patient, err := repo.Get(ctx, clinicId, *randomPatient.UserId)
				Expect(err).To(Succeed())
				Expect(patient.Sites != nil).To(BeTrue())
				Expect(slices.ContainsFunc(*patient.Sites, func(s sites.Site) bool {
					return s.Name == site.Name
				})).To(BeTrue())
			})
		})
	})
})

var _ = Describe("TideReport", func() {
	Context("Metadata", func() {
		It("includes the number of candidate patients", func() {
			numWithoutData := 151
			ctx, th := newTestRepo(GinkgoT(), patientDataCounts{}, numWithoutData)
			params := th.params("7d", time.Now().Add(-7*24*time.Hour))

			tide, err := th.repo.TideReport(ctx, th.clinicId.Hex(), params)
			Expect(err).To(Succeed())
			Expect(tide.Metadata.CandidatePatients).To(Equal(numWithoutData))
		})

		It("includes the number of selected patients", func() {
			withDataCounts := patientDataCounts{
				withVeryLow:        33,
				withLow:            33,
				withMeetingTargets: 35,
			}
			numWithoutData := 51
			ctx, th := newTestRepo(GinkgoT(), withDataCounts, numWithoutData)
			params := th.params("7d", time.Now().Add(-7*24*time.Hour))

			tide, err := th.repo.TideReport(ctx, th.clinicId.Hex(), params)
			Expect(err).To(Succeed())
			exp := patientsRepository.TideReportNoDataPatientLimit + patientsRepository.TideReportPatientLimit
			Expect(tide.Metadata.SelectedPatients).To(Equal(exp))
		})

		AfterEach(func() {
			database := dbTest.GetTestDatabase()
			patients := database.Collection("patients")
			_, err := patients.DeleteMany(context.Background(), primitive.M{})
			Expect(err).To(Succeed())
		})
	})

	Describe("TideResults", func() {
		var cfg *config.Config
		var repo patients.Repository
		var database *mongo.Database
		var collection *mongo.Collection
		var clinicId *primitive.ObjectID
		BeforeEach(func() {
			var err error
			cfg = &config.Config{ClinicDemoPatientUserId: DemoPatientId}
			database = dbTest.GetTestDatabase()
			collection = database.Collection("patients")
			lifecycle := fxtest.NewLifecycle(GinkgoT())
			repo, err = patientsRepository.NewRepository(cfg, database, zap.NewNop().Sugar(), lifecycle)
			Expect(err).ToNot(HaveOccurred())
			Expect(repo).ToNot(BeNil())
			lifecycle.RequireStart()
			data, err := test.LoadFixture("test/fixtures/patient_summaries.json")
			Expect(err).ToNot(HaveOccurred())
			vr, err := bsonrw.NewExtJSONValueReader(bytes.NewReader(data), false)
			Expect(err).ToNot(HaveOccurred())

			clinicId = objectidp(primitive.NewObjectID())
			decoder, err := bson.NewDecoder(vr)
			Expect(err).ToNot(HaveOccurred())
			var patientRecords []patients.Patient
			err = decoder.Decode(&patientRecords)
			Expect(err).ToNot(HaveOccurred())
			var patientDocs []any
			for _, patient := range patientRecords {
				patient.ClinicId = clinicId
				patient.Id = objectidp(primitive.NewObjectID())
				patientDocs = append(patientDocs, patient)
			}
			_, err = collection.InsertMany(context.Background(), patientDocs)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_, err := collection.DeleteMany(context.Background(), primitive.M{"clinicId": clinicId})
			Expect(err).To(Succeed())
		})

		Describe("TideResultPatient", func() {

			Describe("Defaults", func() {
				var timeInVeryLowResults []patients.TideResultPatient
				var timeInAnyLowPercentResults []patients.TideResultPatient
				var timeCGMUsePercentResults []patients.TideResultPatient
				var noDataPatientResults []patients.TideResultPatient
				var meetingTargetsResults []patients.TideResultPatient
				var timeInVeryHighResults []patients.TideResultPatient
				var timeInAnyHighResults []patients.TideResultPatient
				BeforeEach(func() {
					timeInVeryLowResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("below+3+mmol+L@tidepool.org"),
								FullName: strp("Time below 3.0 mmol/L"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaaab3"),
								Tags:     []string{"efefefefefefefefefefefef", "aaaaaaaaaaaaaaaaaaaaaaaa"},
							},
							AverageGlucoseMmol:       floatp(2.994949494949495),
							TimeCGMUseMinutes:        intp(13860),
							TimeCGMUsePercent:        floatp(0.6875),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.9494949494949495),
							TimeInTargetPercent:      floatp(0),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0.050505050505050504),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(1),
							LastData:                 mustTime("2025-07-31T10:24:00.359Z"),
						},
					}
					timeInAnyLowPercentResults = []patients.TideResultPatient{
						{
							AverageGlucoseMmol: floatp(3.8945273631840798),
							Patient: patients.TidePatient{
								Email:       strp("time+in+low+4+pct@tidepool.org"),
								FullName:    strp("Time below 3.9 mmol/L > 4%"),
								Id:          strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaa4444"),
								Tags:        []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
								Reviews:     nil,
								DataSources: nil,
							},
							TimeCGMUseMinutes:        intp(14070),
							TimeCGMUsePercent:        floatp(0.6979166666666666),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.05472636815920398),
							TimeInTargetPercent:      floatp(0.945273631840796),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.05472636815920398),
							LastData:                 mustTime("2025-07-03T14:49:07.079Z"),
						},
					}
					timeInVeryHighResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("time+in+high+and+very+high@tidepool.org"),
								FullName: strp("High Glucose Person"),
								Id:       strp("7da9314c-8c73-487a-a7fb-ffbf8837f77d"),
								Tags:     []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
							},
							AverageGlucoseMmol:       floatp(9.590087955646196),
							TimeCGMUseMinutes:        intp(19615),
							TimeCGMUsePercent:        floatp(0.9729662698412699),
							TimeInHighPercent:        floatp(0.2523578893703798),
							TimeInLowPercent:         floatp(0.007137394850879429),
							TimeInTargetPercent:      floatp(0.7110119806270711),
							TimeInTargetPercentDelta: floatp(0.01276636659198338),
							TimeInVeryHighPercent:    floatp(0.12923782819270965),
							TimeInVeryLowPercent:     floatp(0.0002549069589599796),
							TimeInAnyHighPercent:     floatp(0.38159571756308946),
							TimeInAnyLowPercent:      floatp(0.007392301809839409),
							LastData:                 mustTime("2025-08-13T16:12:15Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("time+in+high+veryHigh+extremeHigh+glucose@tidepool.org"),
								FullName: strp("Extreme High"),
								Id:       strp("712994f9-79b3-4aaf-a82b-0c50aca5ed13"),
								Tags:     []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
							},
							AverageGlucoseMmol:       floatp(8.963836134474947),
							TimeCGMUseMinutes:        intp(19855),
							TimeCGMUsePercent:        floatp(0.9848710317460317),
							TimeInHighPercent:        floatp(0.20246789221858474),
							TimeInLowPercent:         floatp(0.02820448249811131),
							TimeInTargetPercent:      floatp(0.7139511458071015),
							TimeInTargetPercentDelta: floatp(-0.08842379102968467),
							TimeInVeryHighPercent:    floatp(0.14102241249055653),
							TimeInVeryLowPercent:     floatp(0.004354066985645933),
							TimeInAnyHighPercent:     floatp(0.34349030470914127),
							TimeInExtremeHighPercent: floatp(0.02064971040040292),
							TimeInAnyLowPercent:      floatp(0.03255854948375724),
							LastData:                 mustTime("2025-07-08T18:15:42.591Z"),
						},
					}
					timeInAnyHighResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("time+in+high@tidepool.org"),
								FullName: strp("High Glucose"),
								Id:       strp("54dceb3f-8591-45d1-b275-355e7f547aca"),
								Tags:     []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
							},
							AverageGlucoseMmol:       floatp(8.277430555555556),
							TimeCGMUseMinutes:        intp(20160),
							TimeCGMUsePercent:        floatp(1),
							TimeInHighPercent:        floatp(0.5972222222222222),
							TimeInLowPercent:         floatp(0),
							TimeInTargetPercent:      floatp(0.3819444444444444),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0.020833333333333332),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0.6180555555555556),
							TimeInAnyLowPercent:      floatp(0),
							TimeInExtremeHighPercent: floatp(0),
							LastData:                 mustTime("2025-07-05T23:55:00Z"),
						},
					}

					timeCGMUsePercentResults = []patients.TideResultPatient{
						{
							AverageGlucoseMmol: floatp(3.898989898989899),
							Patient: patients.TidePatient{
								Email:    strp("cgmweartime+lt+70+percent@tidepool.org"),
								FullName: strp("CGM Wear Time <70%"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaac70"),
								Tags:     []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
								Reviews:  nil,
								DataSources: &[]patients.DataSource{
									{
										DataSourceId:   nil,
										ModifiedTime:   nil,
										ExpirationTime: mustTime("2025-10-30T20:49:05.465Z"),
										ProviderName:   "dexcom",
										State:          "connected",
									},
								},
							},
							TimeCGMUseMinutes:        intp(13860),
							TimeCGMUsePercent:        floatp(0.6875),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.010101010101010102),
							TimeInTargetPercent:      floatp(0.97979797979799),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.010101010101010102),
							LastData:                 mustTime("2025-07-30T09:09:39.959Z"),
						},
					}
					noDataPatientResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("out+of+time+cutoff@tidepool.org"),
								FullName: strp("Part of category 'noData' in Tide Report because out of time cutoff."),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaadec"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`, `aaaaaaaaaaaaaaaaaaaaaaab`, `aaaaaaaaaaaaaaaaaaaaaaac`},
							},
							AverageGlucoseMmol:       floatp(3.8986111111111112),
							TimeCGMUseMinutes:        intp(15120),
							TimeCGMUsePercent:        floatp(0.75),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.013888888888888888),
							TimeInTargetPercent:      floatp(0.9561111111111112),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.013888888888888888),
							LastData:                 mustTime("2025-06-25T16:04:07.079Z"),
						},
						{
							AverageGlucoseMmol:         floatp(4.366742521825891),
							GlucoseManagementIndicator: floatp(5.2),
							Patient: patients.TidePatient{
								Email:    strp("disconnected+user@tidepool.org"),
								FullName: strp("Disconnected User"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaadcdc"),
								Tags:     []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
								Reviews:  nil,
								DataSources: &[]patients.DataSource{
									{
										DataSourceId:   mustObjectID("686c054cbea00653fd4fcf8b"),
										ModifiedTime:   mustTime("2025-07-07T17:41:13Z"),
										ExpirationTime: nil,
										ProviderName:   "dexcom",
										State:          "disconnected",
									},
								},
							},
							TimeCGMUseMinutes:        intp(20045),
							TimeCGMUsePercent:        floatp(0.9942956349206349),
							TimeInHighPercent:        floatp(0.001995510102269893),
							TimeInLowPercent:         floatp(0.014218009478672985),
							TimeInTargetPercent:      floatp(0.976303317535545),
							TimeInTargetPercentDelta: floatp(0.022936733994397884),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0.0074831628835120975),
							TimeInAnyHighPercent:     floatp(0.001995510102269893),
							TimeInAnyLowPercent:      floatp(0.021701172362185085),
							LastData:                 mustTime("2025-07-07T16:38:39.206Z"),
						},
					}

					meetingTargetsResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets@tidepool.org"),
								FullName: strp("Meeting Targets"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaaaaa"),
								Tags:     []string{`efefefefefefefefefefefef`, `aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(3.8986111111111112),
							TimeCGMUseMinutes:        intp(15120),
							TimeCGMUsePercent:        floatp(0.75),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.013888888888888888),
							TimeInTargetPercent:      floatp(0.9861111111111112),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.013888888888888888),
							LastData:                 mustTime("2025-07-31T11:54:00.359Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("disconnected+user@tidepool.org"),
								FullName: strp("Disconnected User"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaadcdc"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(4.366742521825891),
							TimeCGMUseMinutes:        intp(20045),
							TimeCGMUsePercent:        floatp(0.9942956349206349),
							TimeInHighPercent:        floatp(0.001995510102269893),
							TimeInLowPercent:         floatp(0.014218009478672985),
							TimeInTargetPercent:      floatp(0.976303317535545),
							TimeInTargetPercentDelta: floatp(0.022936733994397884),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0.0074831628835120975),
							TimeInAnyHighPercent:     floatp(0.001995510102269893),
							TimeInAnyLowPercent:      floatp(0.021701172362185085),
							LastData:                 mustTime("2025-07-07T16:38:39.206Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets+ii@tidepool.org"),
								FullName: strp("Meeting Targets II"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaa0000"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(5.300278257324843),
							TimeCGMUseMinutes:        intp(19625),
							TimeCGMUsePercent:        floatp(0.9734623015873016),
							TimeInHighPercent:        floatp(0.06929936305732484),
							TimeInLowPercent:         floatp(0.006878980891719746),
							TimeInTargetPercent:      floatp(0.8991082802547771),
							TimeInTargetPercentDelta: floatp(0.008514718886567851),
							TimeInVeryHighPercent:    floatp(0.019872611464968153),
							TimeInVeryLowPercent:     floatp(0.004840764331210191),
							TimeInAnyHighPercent:     floatp(0.08917197452229299),
							TimeInAnyLowPercent:      floatp(0.011719745222929937),
							LastData:                 mustTime("2025-07-07T16:28:39.66Z"),
						}}
				})

				Context("Config", func() {
					It("includes the correct config metadata", func(ctx SpecContext) {
						period := "14d"
						cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
						params := patients.TideReportParams{
							Period:         period,
							Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
							LastDataCutoff: cutoff,
						}
						report, err := repo.TideReport(ctx, clinicId.Hex(), params)
						Expect(err).ToNot(HaveOccurred())

						Expect(report.Config.Period).To(Equal("14d"))
						Expect(report.Config.LastDataCutoff).To(Equal(params.LastDataCutoff))
						Expect(report.Config.Tags).To(ConsistOf(params.Tags))
						Expect(report.Config.SchemaVersion).To(Equal(patients.TideSchemaVersion))
						Expect(report.Config.ClinicId).To(Equal(clinicId.Hex()))
						Expect(report.Config.VeryHighGlucoseThreshold).To(Equal(patients.VeryHighGlucoseThreshold))
						Expect(report.Config.VeryLowGlucoseThreshold).To(Equal(patients.VeryLowGlucoseThreshold))
						Expect(report.Config.ExtremeHighGlucoseThreshold).To(Equal(patients.ExtremeHighGlucoseThreshold))
						Expect(report.Config.LowGlucoseThreshold).To(Equal(patients.LowGlucoseThreshold))
						Expect(report.Config.HighGlucoseThreshold).To(Equal(patients.HighGlucoseThreshold))
					})
				})

				It("matches default categories given no categories in params", func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 8
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(11))
					Expect(report.Metadata.SelectedPatients).To(Equal(11))

					Expect(report.Results["timeInVeryLowPercent"]).To(tideResultsPatientMatcher(timeInVeryLowResults))

					Expect(report.Results["timeInAnyLowPercent"]).To(tideResultsPatientMatcher(timeInAnyLowPercentResults))

					Expect(report.Results["timeCGMUsePercent"]).To(tideResultsPatientMatcher(timeCGMUsePercentResults))

					Expect(report.Results["noData"]).To(tideResultsPatientMatcher(noDataPatientResults))

					Expect(report.Results["meetingTargets"]).To(tideResultsPatientMatcher(meetingTargetsResults))
					Expect(report.Results["timeInVeryHighPercent"]).To(tideResultsPatientMatcher(timeInVeryHighResults))
					Expect(report.Results["timeInAnyHighPercent"]).To(tideResultsPatientMatcher(timeInAnyHighResults))
				})

				It("matches default categories if Tide Report params use explicitly empty categories", func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						Categories:     []string{},
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 8
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(11))
					Expect(report.Metadata.SelectedPatients).To(Equal(11))

					Expect(report.Results["timeInVeryLowPercent"]).To(tideResultsPatientMatcher(timeInVeryLowResults))

					Expect(report.Results["timeInAnyLowPercent"]).To(tideResultsPatientMatcher(timeInAnyLowPercentResults))

					Expect(report.Results["timeCGMUsePercent"]).To(tideResultsPatientMatcher(timeCGMUsePercentResults))

					Expect(report.Results["noData"]).To(tideResultsPatientMatcher(noDataPatientResults))

					Expect(report.Results["meetingTargets"]).To(tideResultsPatientMatcher(meetingTargetsResults))
					Expect(report.Results["timeInVeryHighPercent"]).To(tideResultsPatientMatcher(timeInVeryHighResults))
					Expect(report.Results["timeInAnyHighPercent"]).To(tideResultsPatientMatcher(timeInAnyHighResults))
				})

				It(`excludes the "noData" category if params excludeNoDataPatient is explicitly set`, func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						ExcludeNoData:  true,
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 7
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(9))
					Expect(report.Metadata.SelectedPatients).To(Equal(9))

					Expect(report.Results["timeInVeryLowPercent"]).To(tideResultsPatientMatcher(timeInVeryLowResults))

					Expect(report.Results["timeInAnyLowPercent"]).To(tideResultsPatientMatcher(timeInAnyLowPercentResults))

					Expect(report.Results["timeCGMUsePercent"]).To(tideResultsPatientMatcher(timeCGMUsePercentResults))

					Expect(report.Results["meetingTargets"]).To(tideResultsPatientMatcher(meetingTargetsResults))
					Expect(report.Results["timeInVeryHighPercent"]).To(tideResultsPatientMatcher(timeInVeryHighResults))
					Expect(report.Results["timeInAnyHighPercent"]).To(tideResultsPatientMatcher(timeInAnyHighResults))

					Expect(report.Results["noData"]).To(BeEmpty())
				})

				It(`puts patients in next satisfied category if they would match default categories but specific ones selected that don't include that`, func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						ExcludeNoData:  true,
						Categories:     []string{"timeInVeryLowPercent", "timeCGMUsePercent"},
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 2
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(3))
					Expect(report.Metadata.SelectedPatients).To(Equal(3))

					timeCGMUsePercentResults = []patients.TideResultPatient{
						// This patient would normally be put in the "timeInAnyLowPercentResults" category if using default categories, but since there are non-empty Categories params that don't include timeInAnyLowPercentResults, that user is put in the next available
						{
							AverageGlucoseMmol: floatp(3.8945273631840798),
							Patient: patients.TidePatient{
								Email:       strp("time+in+low+4+pct@tidepool.org"),
								FullName:    strp("Time below 3.9 mmol/L > 4%"),
								Id:          strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaa4444"),
								Tags:        []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
								Reviews:     nil,
								DataSources: nil,
							},
							TimeCGMUseMinutes:        intp(14070),
							TimeCGMUsePercent:        floatp(0.6979166666666666),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.05472636815920398),
							TimeInTargetPercent:      floatp(0.945273631840796),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.05472636815920398),
							LastData:                 mustTime("2025-07-03T14:49:07.079Z"),
						},
						{
							AverageGlucoseMmol: floatp(3.898989898989899),
							Patient: patients.TidePatient{
								Email:    strp("cgmweartime+lt+70+percent@tidepool.org"),
								FullName: strp("CGM Wear Time <70%"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaac70"),
								Tags:     []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
								Reviews:  nil,
								DataSources: &[]patients.DataSource{
									{
										DataSourceId:   nil,
										ModifiedTime:   nil,
										ExpirationTime: mustTime("2025-10-30T20:49:05.465Z"),
										ProviderName:   "dexcom",
										State:          "connected",
									},
								},
							},
							TimeCGMUseMinutes:        intp(13860),
							TimeCGMUsePercent:        floatp(0.6875),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.010101010101010102),
							TimeInTargetPercent:      floatp(0.97979797979799),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.010101010101010102),
							LastData:                 mustTime("2025-07-30T09:09:39.959Z"),
						},
					}
					Expect(report.Results["timeInVeryLowPercent"]).To(tideResultsPatientMatcher(timeInVeryLowResults))

					Expect(report.Results["timeCGMUsePercent"]).To(tideResultsPatientMatcher(timeCGMUsePercentResults))
				})

				It(`meetingTargets correctly identifies patients meeting targets if no other categories are given in parameters`, func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						ExcludeNoData:  true,
						Categories:     []string{"meetingTargets"},
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 1
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(3))
					Expect(report.Metadata.SelectedPatients).To(Equal(3))

					meetingTargetsResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets@tidepool.org"),
								FullName: strp("Meeting Targets"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaaaaa"),
								Tags:     []string{`efefefefefefefefefefefef`, `aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(3.8986111111111112),
							TimeCGMUseMinutes:        intp(15120),
							TimeCGMUsePercent:        floatp(0.75),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.013888888888888888),
							TimeInTargetPercent:      floatp(0.9861111111111112),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.013888888888888888),
							LastData:                 mustTime("2025-07-31T11:54:00.359Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("disconnected+user@tidepool.org"),
								FullName: strp("Disconnected User"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaadcdc"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(4.366742521825891),
							TimeCGMUseMinutes:        intp(20045),
							TimeCGMUsePercent:        floatp(0.9942956349206349),
							TimeInHighPercent:        floatp(0.001995510102269893),
							TimeInLowPercent:         floatp(0.014218009478672985),
							TimeInTargetPercent:      floatp(0.976303317535545),
							TimeInTargetPercentDelta: floatp(0.022936733994397884),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0.0074831628835120975),
							TimeInAnyHighPercent:     floatp(0.001995510102269893),
							TimeInAnyLowPercent:      floatp(0.021701172362185085),
							LastData:                 mustTime("2025-07-07T16:38:39.206Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets+ii@tidepool.org"),
								FullName: strp("Meeting Targets II"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaa0000"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(5.300278257324843),
							TimeCGMUseMinutes:        intp(19625),
							TimeCGMUsePercent:        floatp(0.9734623015873016),
							TimeInHighPercent:        floatp(0.06929936305732484),
							TimeInLowPercent:         floatp(0.006878980891719746),
							TimeInTargetPercent:      floatp(0.8991082802547771),
							TimeInTargetPercentDelta: floatp(0.008514718886567851),
							TimeInVeryHighPercent:    floatp(0.019872611464968153),
							TimeInVeryLowPercent:     floatp(0.004840764331210191),
							TimeInAnyHighPercent:     floatp(0.08917197452229299),
							TimeInAnyLowPercent:      floatp(0.011719745222929937),
							LastData:                 mustTime("2025-07-07T16:28:39.66Z"),
						},
					}
					Expect(report.Results["meetingTargets"]).To(tideResultsPatientMatcher(meetingTargetsResults))
				})

				It(`meetingTargets and noData patients works`, func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						ExcludeNoData:  false,
						Categories:     []string{"meetingTargets"},
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 2
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(5))
					Expect(report.Metadata.SelectedPatients).To(Equal(5))

					meetingTargetsResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets@tidepool.org"),
								FullName: strp("Meeting Targets"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaaaaa"),
								Tags:     []string{`efefefefefefefefefefefef`, `aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(3.8986111111111112),
							TimeCGMUseMinutes:        intp(15120),
							TimeCGMUsePercent:        floatp(0.75),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.013888888888888888),
							TimeInTargetPercent:      floatp(0.9861111111111112),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.013888888888888888),
							LastData:                 mustTime("2025-07-31T11:54:00.359Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("disconnected+user@tidepool.org"),
								FullName: strp("Disconnected User"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaadcdc"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(4.366742521825891),
							TimeCGMUseMinutes:        intp(20045),
							TimeCGMUsePercent:        floatp(0.9942956349206349),
							TimeInHighPercent:        floatp(0.001995510102269893),
							TimeInLowPercent:         floatp(0.014218009478672985),
							TimeInTargetPercent:      floatp(0.976303317535545),
							TimeInTargetPercentDelta: floatp(0.022936733994397884),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0.0074831628835120975),
							TimeInAnyHighPercent:     floatp(0.001995510102269893),
							TimeInAnyLowPercent:      floatp(0.021701172362185085),
							LastData:                 mustTime("2025-07-07T16:38:39.206Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets+ii@tidepool.org"),
								FullName: strp("Meeting Targets II"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaa0000"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(5.300278257324843),
							TimeCGMUseMinutes:        intp(19625),
							TimeCGMUsePercent:        floatp(0.9734623015873016),
							TimeInHighPercent:        floatp(0.06929936305732484),
							TimeInLowPercent:         floatp(0.006878980891719746),
							TimeInTargetPercent:      floatp(0.8991082802547771),
							TimeInTargetPercentDelta: floatp(0.008514718886567851),
							TimeInVeryHighPercent:    floatp(0.019872611464968153),
							TimeInVeryLowPercent:     floatp(0.004840764331210191),
							TimeInAnyHighPercent:     floatp(0.08917197452229299),
							TimeInAnyLowPercent:      floatp(0.011719745222929937),
							LastData:                 mustTime("2025-07-07T16:28:39.66Z"),
						},
					}
					Expect(report.Results["meetingTargets"]).To(tideResultsPatientMatcher(meetingTargetsResults))
					Expect(report.Results["noData"]).To(tideResultsPatientMatcher(noDataPatientResults))
				})

				It(`meetingTargets and another chosen category, "timeInVeryLowPercent" works`, func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						ExcludeNoData:  true,
						Categories:     []string{"meetingTargets", "timeInVeryLowPercent"},
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 2
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(4))
					Expect(report.Metadata.SelectedPatients).To(Equal(4))

					meetingTargetsResults = []patients.TideResultPatient{
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets@tidepool.org"),
								FullName: strp("Meeting Targets"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaaaaaa"),
								Tags:     []string{`efefefefefefefefefefefef`, `aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(3.8986111111111112),
							TimeCGMUseMinutes:        intp(15120),
							TimeCGMUsePercent:        floatp(0.75),
							TimeInHighPercent:        floatp(0),
							TimeInLowPercent:         floatp(0.013888888888888888),
							TimeInTargetPercent:      floatp(0.9861111111111112),
							TimeInTargetPercentDelta: floatp(0),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0),
							TimeInAnyHighPercent:     floatp(0),
							TimeInAnyLowPercent:      floatp(0.013888888888888888),
							LastData:                 mustTime("2025-07-31T11:54:00.359Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("disconnected+user@tidepool.org"),
								FullName: strp("Disconnected User"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaadcdc"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(4.366742521825891),
							TimeCGMUseMinutes:        intp(20045),
							TimeCGMUsePercent:        floatp(0.9942956349206349),
							TimeInHighPercent:        floatp(0.001995510102269893),
							TimeInLowPercent:         floatp(0.014218009478672985),
							TimeInTargetPercent:      floatp(0.976303317535545),
							TimeInTargetPercentDelta: floatp(0.022936733994397884),
							TimeInVeryHighPercent:    floatp(0),
							TimeInVeryLowPercent:     floatp(0.0074831628835120975),
							TimeInAnyHighPercent:     floatp(0.001995510102269893),
							TimeInAnyLowPercent:      floatp(0.021701172362185085),
							LastData:                 mustTime("2025-07-07T16:38:39.206Z"),
						},
						{
							Patient: patients.TidePatient{
								Email:    strp("meeting+targets+ii@tidepool.org"),
								FullName: strp("Meeting Targets II"),
								Id:       strp("aaaaaaaa-bbbb-cccc-dddd-aaaaaaaa0000"),
								Tags:     []string{`aaaaaaaaaaaaaaaaaaaaaaaa`},
							},
							AverageGlucoseMmol:       floatp(5.300278257324843),
							TimeCGMUseMinutes:        intp(19625),
							TimeCGMUsePercent:        floatp(0.9734623015873016),
							TimeInHighPercent:        floatp(0.06929936305732484),
							TimeInLowPercent:         floatp(0.006878980891719746),
							TimeInTargetPercent:      floatp(0.8991082802547771),
							TimeInTargetPercentDelta: floatp(0.008514718886567851),
							TimeInVeryHighPercent:    floatp(0.019872611464968153),
							TimeInVeryLowPercent:     floatp(0.004840764331210191),
							TimeInAnyHighPercent:     floatp(0.08917197452229299),
							TimeInAnyLowPercent:      floatp(0.011719745222929937),
							LastData:                 mustTime("2025-07-07T16:28:39.66Z"),
						},
					}
					Expect(report.Results["meetingTargets"]).To(tideResultsPatientMatcher(meetingTargetsResults))
					Expect(report.Results["timeInVeryLowPercent"]).To(tideResultsPatientMatcher(timeInVeryLowResults))
				})

				It(`patient will be placed in "closest" category supplied in parameters when they "qualify" for a higher category. People in timeInVeryHighPercent will be placed in "lower" category timeInAnyHighPercent only if timeInAnyHighPercent category is selected before timeInVeryHighPercent or timeInVeryHighPercent is excluded in the category parameters`, func(ctx SpecContext) {
					period := "14d"
					cutoff := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
					params := patients.TideReportParams{
						Period:         period,
						Tags:           []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
						LastDataCutoff: cutoff,
						ExcludeNoData:  true,
						Categories:     []string{"timeInAnyHighPercent"},
					}
					report, err := repo.TideReport(ctx, clinicId.Hex(), params)
					Expect(err).ToNot(HaveOccurred())
					numResultCategories := 1
					Expect(len(report.Results)).To(Equal(numResultCategories))
					Expect(report.Metadata.CandidatePatients).To(Equal(3))
					Expect(report.Metadata.SelectedPatients).To(Equal(3))

					timeInAnyHighAndUp := append(slices.Clone(timeInAnyHighResults), timeInVeryHighResults...)
					Expect(report.Results["timeInAnyHighPercent"]).To(tideResultsPatientMatcher(timeInAnyHighAndUp))
				})
			})
		})
	})
})

func newTestRepo(t FullGinkgoTInterface, dataCounts patientDataCounts, withoutData int) (
	context.Context, *repoTestHelper) {

	t.Helper()
	cfg := &config.Config{ClinicDemoPatientUserId: DemoPatientId}
	database := dbTest.GetTestDatabase()
	collection := database.Collection("patients")
	lifecycle := fxtest.NewLifecycle(t)
	logger := testLogger()
	repo, err := patientsRepository.NewRepository(cfg, database, logger, lifecycle)
	Expect(err).ToNot(HaveOccurred())
	Expect(repo).ToNot(BeNil())
	lifecycle.RequireStart()
	ctx := context.Background()
	clinicId := primitive.NewObjectID()
	tagId := primitive.NewObjectID()

	yesterday := time.Now().Add(-24 * time.Hour)
	allPatients := []any{}
	modelPatient := patientsTest.RandomPatient() // re-use patient to save a little time
	withData := dataCounts.Counts()
	periods := genPeriods(dataCounts)
	for i := range withData {
		patient := modelPatient
		patientUUID := test.Faker.UUID().V4()
		patient.UserId = &patientUUID
		oid := primitive.NewObjectID()
		patient.Id = &oid
		patient.Summary = &patients.Summary{
			CGM: &patients.PatientCGMStats{
				Dates: patients.PatientSummaryDates{
					LastData: &yesterday,
				},
				Periods: patients.PatientCGMPeriods{
					"7d": periods[i],
				},
			},
		}
		patient.ClinicId = &clinicId
		patient.Tags = &[]primitive.ObjectID{tagId}
		allPatients = append(allPatients, patient)
	}
	for range withoutData {
		patient := patientsTest.RandomPatient()

		patient.ClinicId = &clinicId
		patient.Tags = &[]primitive.ObjectID{tagId}
		allPatients = append(allPatients, patient)
	}
	result, err := collection.InsertMany(ctx, allPatients)
	Expect(err).ToNot(HaveOccurred())
	Expect(len(result.InsertedIDs)).To(Equal(withData + withoutData))

	return ctx, &repoTestHelper{
		clinicId: clinicId,
		tagId:    tagId,
		repo:     repo,
	}
}

type patientDataCounts struct {
	withVeryLow        int
	withLow            int
	withVeryHigh       int
	withHigh           int
	withMeetingTargets int
}

func (p patientDataCounts) Counts() int {
	return p.withVeryLow + p.withLow + p.withVeryHigh + p.withHigh + p.withMeetingTargets
}

func genPeriods(counts patientDataCounts) []patients.PatientCGMPeriod {
	withData := counts.withVeryLow + counts.withLow + counts.withMeetingTargets + counts.withVeryHigh + counts.withHigh
	periods := make([]patients.PatientCGMPeriod, 0, withData)
	for range counts.withVeryLow {
		timeInVeryLow := 0.015
		periods = append(periods, patients.PatientCGMPeriod{
			TimeInVeryLowPercent: &timeInVeryLow,
			TimeInAnyLowPercent:  &timeInVeryLow,
		})
	}
	for range counts.withLow {
		timeInLow := .045
		periods = append(periods, patients.PatientCGMPeriod{
			TimeInLowPercent:    &timeInLow,
			TimeInAnyLowPercent: &timeInLow,
		})
	}
	for range counts.withMeetingTargets {
		timeInRange := 0.75
		timeCGMUse := 0.80
		periods = append(periods, patients.PatientCGMPeriod{
			TimeInTargetPercent: &timeInRange,
			TimeCGMUsePercent:   &timeCGMUse,
		})
	}
	for range counts.withVeryHigh {
		timeInRange := 0.75
		timeCGMUse := 0.80
		periods = append(periods, patients.PatientCGMPeriod{
			TimeInTargetPercent: &timeInRange,
			TimeCGMUsePercent:   &timeCGMUse,
		})
	}
	for range counts.withHigh {
		timeInHigh := 0.26
		periods = append(periods, patients.PatientCGMPeriod{
			TimeInHighPercent:    &timeInHigh,
			TimeInAnyHighPercent: &timeInHigh,
		})
	}
	for range counts.withVeryHigh {
		timeInVeryHigh := 0.055
		periods = append(periods, patients.PatientCGMPeriod{
			TimeInVeryHighPercent: &timeInVeryHigh,
			TimeInAnyHighPercent:  &timeInVeryHigh,
		})
	}
	rand.Shuffle(len(periods), func(i, j int) {
		periods[i], periods[j] = periods[j], periods[i]
	})
	return periods
}

type repoTestHelper struct {
	clinicId primitive.ObjectID
	tagId    primitive.ObjectID
	repo     patients.Repository
}

func (r repoTestHelper) params(period string, cutoff time.Time) patients.TideReportParams {
	return patients.TideReportParams{
		Period:         period,
		Tags:           []string{r.tagId.Hex()},
		LastDataCutoff: cutoff,
	}
}

func testLogger() *zap.SugaredLogger {
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(GinkgoWriter), zapcore.
		DebugLevel)
	return zap.New(core).Sugar()
}

func tideResultsPatientMatcher(results []patients.TideResultPatient) types.GomegaMatcher {
	matches := make([]types.GomegaMatcher, 0, len(results))
	for result := range slices.Values(results) {
		matches = append(matches, tideResultPatientMatcher(result))
	}
	return ConsistOf(matches)
}

func tideResultPatientMatcher(result patients.TideResultPatient) types.GomegaMatcher {
	fields := Fields{
		"AverageGlucoseMmol":         PointTo(BeNumerically(`~`, *result.AverageGlucoseMmol, math.SmallestNonzeroFloat64)),
		"Patient":                    tidePatientMatcher(result.Patient),
		"TimeCGMUseMinutes":          PointTo(BeNumerically(`~`, *result.TimeCGMUseMinutes, math.SmallestNonzeroFloat64)),
		"TimeCGMUsePercent":          PointTo(BeNumerically(`~`, *result.TimeCGMUsePercent, math.SmallestNonzeroFloat64)),
		"TimeInHighPercent":          PointTo(BeNumerically(`~`, *result.TimeInHighPercent, math.SmallestNonzeroFloat64)),
		"TimeInLowPercent":           PointTo(BeNumerically(`~`, *result.TimeInLowPercent, math.SmallestNonzeroFloat64)),
		"TimeInTargetPercent":        PointTo(BeNumerically(`~`, *result.TimeInTargetPercent, math.SmallestNonzeroFloat64)),
		"TimeInTargetPercentDelta":   PointTo(BeNumerically(`~`, *result.TimeInTargetPercentDelta, math.SmallestNonzeroFloat64)),
		"TimeInVeryHighPercent":      PointTo(BeNumerically(`~`, *result.TimeInVeryHighPercent, math.SmallestNonzeroFloat64)),
		"TimeInVeryLowPercent":       PointTo(BeNumerically(`~`, *result.TimeInVeryLowPercent, math.SmallestNonzeroFloat64)),
		"TimeInAnyHighPercent":       PointTo(BeNumerically(`~`, *result.TimeInAnyHighPercent, math.SmallestNonzeroFloat64)),
		"TimeInAnyLowPercent":        PointTo(BeNumerically(`~`, *result.TimeInAnyLowPercent, math.SmallestNonzeroFloat64)),
		"LastData":                   Ignore(),
		"GlucoseManagementIndicator": Ignore(),
		"TimeInExtremeHighPercent":   Ignore(),
	}
	// May be nil
	if result.GlucoseManagementIndicator != nil {
		fields["GlucoseManagementIndicator"] = PointTo(BeNumerically(`~`, *result.GlucoseManagementIndicator, math.SmallestNonzeroFloat64))
	}
	// May be nil if user has no data
	if result.LastData != nil {
		fields["LastData"] = PointTo(BeTemporally(`~`, *result.LastData, time.Second))
	}
	if result.TimeInExtremeHighPercent != nil {
		fields["TimeInExtremeHighPercent"] = PointTo(BeNumerically(`~`, *result.TimeInExtremeHighPercent, math.SmallestNonzeroFloat64))
	}

	return MatchAllFields(fields)
}

func tidePatientMatcher(patient patients.TidePatient) types.GomegaMatcher {
	return MatchAllFields(Fields{
		"Id":          PointTo(Not(BeEmpty())),
		"Email":       PointTo(Equal(*patient.Email)),
		"FullName":    PointTo(Equal(*patient.FullName)),
		"Tags":        ContainElements(patient.Tags),
		"DataSources": Ignore(),
		"Reviews":     Ignore(),
	})
}

func strp(s string) *string {
	return &s
}

func floatp(f float64) *float64 {
	return &f
}

func intp(i int) *int {
	return &i
}

func objectidp(o primitive.ObjectID) *primitive.ObjectID {
	return &o
}

func mustObjectID(hex string) *primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return &id
}

func mustTime(tim string) *time.Time {
	t, err := time.Parse(time.RFC3339, tim)
	if err != nil {
		panic(err)
	}
	return &t
}
