package patients_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"time"
)


var _ = Describe("Patient Deletions Repository", func() {
	var repo patients.PatientDeletionsRepository
	var database *mongo.Database
	var collection *mongo.Collection

	BeforeEach(func() {
		var err error
		database = dbTest.GetTestDatabase()
		collection = database.Collection("patient_deletions")
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		repo, err = patients.NewDeletionsRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	Describe("Create", func() {
		var clinician shoreline.UserData
		var deletion patients.PatientDeletion
		var matchPatientFields types.GomegaMatcher

		BeforeEach(func() {
			clinician = patientsTest.RandomUser()
			patientId := primitive.NewObjectID()
			deletion = patients.PatientDeletion{
				Patient:       patientsTest.RandomPatient(),
				DeletedTime:   time.Now(),
				DeletedByUserId: &clinician.UserID,
			}
			deletion.Patient.Id = &patientId
			matchPatientFields = patientFieldsMatcher(deletion.Patient)
		})

		It("creates the patient deletion record", func() {
			err := repo.Create(context.Background(), deletion)
			Expect(err).ToNot(HaveOccurred())

			var result patients.PatientDeletion
			err = collection.FindOne(context.Background(), bson.M{
				"patient._id": deletion.Patient.Id,
			}).Decode(&result)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Patient).To(matchPatientFields)
			Expect(result.DeletedTime).To(BeTemporally("~", time.Now(), time.Second))
			Expect(result.DeletedByUserId).To(gstruct.PointTo(Equal(clinician.UserID)))
		})
	})
})
