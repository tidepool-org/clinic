package manager_test

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/clinics/test"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/patients"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
)

var _ = Describe("Clinics Manager", func() {
	var patientsRepo patients.Repository
	var clinicsRepo clinics.Service
	var cliniciansRepo *clinicians.Repository

	var database *mongo.Database
	var patientsCollection *mongo.Collection
	var cliniciansCollection *mongo.Collection
	var clinicsCollection *mongo.Collection
	var mngr manager.Manager

	var DemoPatientId = "demo"

	BeforeEach(func() {
		var err error
		database = dbTest.GetTestDatabase()
		patientsCollection = database.Collection("patients")
		cliniciansCollection = database.Collection("clinicians")
		clinicsCollection = database.Collection("clinics")

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		patientsRepo, err = patients.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(patientsRepo).ToNot(BeNil())

		cliniciansRepo, err = clinicians.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(cliniciansRepo).ToNot(BeNil())

		clinicsRepo, err = clinics.NewRepository(database, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(clinicsRepo).ToNot(BeNil())

		mngr, err = manager.NewManager(manager.Params{
			Clinics:              clinicsRepo,
			CliniciansRepository: cliniciansRepo,
			Config:               &manager.Config{ClinicDemoPatientUserId: DemoPatientId},
			DbClient:             database.Client(),
			PatientsService:      patientsRepo,
			ShareCodeGenerator:   nil,
			UserService:          nil,
		})

		lifecycle.RequireStart()
	})

	Describe("Delete", func() {
		var clinic *clinics.Clinic

		Context("With existing clinic", func() {
			BeforeEach(func() {
				clinic = test.RandomClinic()

				res, err := clinicsCollection.InsertOne(context.Background(), clinic)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				clinicId := res.InsertedID.(primitive.ObjectID)
				clinic.Id = &clinicId
			})

			It("deletes the clinic object", func() {
				err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
				Expect(err).ToNot(HaveOccurred())

				selector := bson.M{
					"_id": clinic.Id,
				}

				count, err := clinicsCollection.CountDocuments(context.Background(), selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(0)))
			})

			Context("With demo patient and clinician", func() {
				var patient patients.Patient
				var clinician *clinicians.Clinician

				BeforeEach(func() {
					patient = patientsTest.RandomPatient()
					patient.UserId = &DemoPatientId
					patient.ClinicId = clinic.Id

					res, err := patientsCollection.InsertOne(context.Background(), patient)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())

					clinician = cliniciansTest.RandomClinician()
					clinician.ClinicId = clinic.Id

					res, err = cliniciansCollection.InsertOne(context.Background(), clinician)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
				})

				It("deletes the clinic object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
					Expect(err).ToNot(HaveOccurred())

					selector := bson.M{
						"_id": clinic.Id,
					}

					count, err := clinicsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})

				It("deletes the patient object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
					Expect(err).ToNot(HaveOccurred())

					patientSelector := bson.M{
						"userId":   DemoPatientId,
						"clinicId": patient.ClinicId,
					}
					count, err := patientsCollection.CountDocuments(context.Background(), patientSelector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})

				It("deletes the clinician object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
					Expect(err).ToNot(HaveOccurred())

					clinicianSelector := bson.M{
						"userId":   clinician.UserId,
						"clinicId": clinician.ClinicId,
					}
					count, err := clinicsCollection.CountDocuments(context.Background(), clinicianSelector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})
			})

			Context("With multiple patients and clinician", func() {
				var patient patients.Patient
				var clinician *clinicians.Clinician

				BeforeEach(func() {
					patient = patientsTest.RandomPatient()
					patient.UserId = &DemoPatientId
					patient.ClinicId = clinic.Id

					res, err := patientsCollection.InsertOne(context.Background(), patient)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())

					secondPatient := patientsTest.RandomPatient()
					secondPatient.ClinicId = clinic.Id
					res, err = patientsCollection.InsertOne(context.Background(), secondPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())

					clinician = cliniciansTest.RandomClinician()
					clinician.ClinicId = clinic.Id

					res, err = cliniciansCollection.InsertOne(context.Background(), clinician)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
				})

				It("returns an error and doesn't delete the clinic object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"_id": clinic.Id,
					}

					count, err := clinicsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(1)))
				})

				It("returns an error and doesn't delete patient objects", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"clinicId": clinic.Id,
					}

					count, err := patientsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(2)))
				})

				It("returns an error and doesn't delete clinician objects", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex())
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"clinicId": clinic.Id,
					}

					count, err := cliniciansCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(1)))
				})

			})
		})
	})
})
