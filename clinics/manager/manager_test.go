package manager_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/deletions"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/patients"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
)

func Ptr[T any](value T) *T {
	return &value
}

var _ = Describe("Clinics Manager", func() {
	var patientsService patients.Service

	var cfg *config.Config
	var database *mongo.Database
	var patientsCollection *mongo.Collection
	var cliniciansCollection *mongo.Collection
	var clinicsCollection *mongo.Collection
	var mngr manager.Manager

	var DemoPatientId = "demo"

	BeforeEach(func() {
		var err error
		cfg = &config.Config{ClinicDemoPatientUserId: DemoPatientId}
		database = dbTest.GetTestDatabase()
		patientsCollection = database.Collection("patients")
		cliniciansCollection = database.Collection("clinicians")
		clinicsCollection = database.Collection("clinics")

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		lgr := zap.NewNop().Sugar()

		cliniciansRepo, err := clinicians.NewRepository(database, lgr, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(cliniciansRepo).ToNot(BeNil())

		clinicsRepo, err := clinics.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(clinicsRepo).ToNot(BeNil())

		clinicsService, err := clinics.NewService(clinicsRepo)
		Expect(err).ToNot(HaveOccurred())
		Expect(clinicsService).ToNot(BeNil())

		patientsRepo, err := patients.NewRepository(cfg, database, lgr, lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(patientsRepo).ToNot(BeNil())

		patientsService, err = patients.NewService(patientsRepo, clinicsService, nil, lgr, database.Client())
		Expect(err).ToNot(HaveOccurred())
		Expect(patientsService).ToNot(BeNil())

		mngr, err = manager.NewManager(manager.Params{
			ClinicsService:       clinicsService,
			CliniciansRepository: cliniciansRepo,
			Config:               cfg,
			DbClient:             database.Client(),
			PatientsRepository:   patientsRepo,
			PatientsService:      patientsService,
			ShareCodeGenerator:   nil,
			UserService:          nil,
		})
		Expect(err).ToNot(HaveOccurred())

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
				err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
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
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).ToNot(HaveOccurred())

					selector := bson.M{
						"_id": clinic.Id,
					}

					count, err := clinicsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(0)))
				})

				It("deletes the patient object", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
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
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
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
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"_id": clinic.Id,
					}

					count, err := clinicsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(1)))
				})

				It("returns an error and doesn't delete patient objects", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
					Expect(err).To(HaveOccurred())

					selector := bson.M{
						"clinicId": clinic.Id,
					}

					count, err := patientsCollection.CountDocuments(context.Background(), selector)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(int64(2)))
				})

				It("returns an error and doesn't delete clinician objects", func() {
					err := mngr.DeleteClinic(context.Background(), clinic.Id.Hex(), deletions.Metadata{})
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

	Describe("GetClinicPatientCount", func() {
		var clinic *clinics.Clinic
		var clinicIdString string

		BeforeEach(func() {
			clinic = test.RandomClinic()
			res, err := clinicsCollection.InsertOne(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())

			clinic.Id = Ptr(res.InsertedID.(primitive.ObjectID))
			clinicIdString = clinic.Id.Hex()
		})

		When("the clinic has no patients", func() {
			It("returns the correct patient count", func() {
				patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
				Expect(err).ToNot(HaveOccurred())
				Expect(patientCount).ToNot(BeNil())
				Expect(patientCount.PatientCount).To(Equal(0))
			})
		})

		When("a patient is added to the clinic", func() {
			BeforeEach(func() {
				randomPatient := patientsTest.RandomPatient()
				randomPatient.ClinicId = clinic.Id
				randomPatient.Permissions = &patients.Permissions{View: &patients.Permission{}}

				createdPatient, err := patientsService.Create(context.Background(), randomPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdPatient).ToNot(BeNil())
			})

			It("returns the correct patient count", func() {
				patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
				Expect(err).ToNot(HaveOccurred())
				Expect(patientCount).ToNot(BeNil())
				Expect(patientCount.PatientCount).To(Equal(1))
			})

			When("a demo patient is added to the clinic", func() {
				BeforeEach(func() {
					randomPatient := patientsTest.RandomPatient()
					randomPatient.UserId = &DemoPatientId
					randomPatient.ClinicId = clinic.Id
					randomPatient.Permissions = &patients.Permissions{View: &patients.Permission{}}

					createdPatient, err := patientsService.Create(context.Background(), randomPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdPatient).ToNot(BeNil())
				})

				It("returns the correct patient count", func() {
					patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
					Expect(err).ToNot(HaveOccurred())
					Expect(patientCount).ToNot(BeNil())
					Expect(patientCount.PatientCount).To(Equal(1))
				})

				When("aanother patient is added to the clinic", func() {
					BeforeEach(func() {
						randomPatient := patientsTest.RandomPatient()
						randomPatient.ClinicId = clinic.Id
						randomPatient.Permissions = &patients.Permissions{View: &patients.Permission{}}

						createdPatient, err := patientsService.Create(context.Background(), randomPatient)
						Expect(err).ToNot(HaveOccurred())
						Expect(createdPatient).ToNot(BeNil())
					})

					It("returns the correct patient count", func() {
						patientCount, err := mngr.GetClinicPatientCount(context.Background(), clinicIdString)
						Expect(err).ToNot(HaveOccurred())
						Expect(patientCount).ToNot(BeNil())
						Expect(patientCount.PatientCount).To(Equal(2))
					})
				})
			})
		})
	})
})
