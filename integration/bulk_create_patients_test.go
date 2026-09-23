package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/tidepool-org/clinic/integration/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"

	"github.com/tidepool-org/clinic/clinicians"
	cliniciansRepository "github.com/tidepool-org/clinic/clinicians/repository"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("BulkCreatePatients Integration Test", Ordered, func() {
	var clinic *clinics.Clinic
	var ctx context.Context
	var clinician *clinicians.Clinician
	var existingPatient *patients.Patient
	var database *mongo.Database
	var clinicId primitive.ObjectID
	var clinicianId string
	BeforeEach(func() {
		ctx = context.Background()
		logger := testLogger()
		database = dbTest.GetTestDatabase()
		lifecycle := fxtest.NewLifecycle(GinkgoT())

		clinic = clinicsTest.RandomClinic()
		clinic.MRNSettings = &clinics.MRNSettings{
			Required: true,
			Unique:   true,
		}
		clinicsRepo, err := clinicsRepository.NewRepository(database, logger, lifecycle)
		Expect(err).To(Succeed())
		clinic, err = clinicsRepo.Create(ctx, clinic)
		Expect(err).To(Succeed())
		clinicId = *clinic.Id

		clinician = cliniciansTest.RandomClinicianWithClinicId(*clinic.Id)
		clinicianId = test.TestUserId
		clinician.UserId = &clinicianId
		clinician.Roles = []string{"CLINIC_ADMIN"}
		cliniciansRepo, err := cliniciansRepository.NewRepository(database, logger, lifecycle)
		Expect(err).To(Succeed())
		clinician, err = cliniciansRepo.Create(ctx, clinician)
		Expect(err).To(Succeed())

		patientData := patientsTest.RandomPatient()
		patientData.ClinicId = &clinicId
		patientEmail := "existing+patient+email@tidepool.org"
		patientData.Email = &patientEmail
		patientMRN := "EXISTINGMRN"
		patientData.Mrn = &patientMRN
		patientsRepo, err := patientsRepository.NewRepository(&config.Config{}, database, logger, lifecycle)
		Expect(err).To(Succeed())
		patient, err := patientsRepo.Create(ctx, patientData)
		Expect(err).To(Succeed())
		existingPatient = patient
	})

	Describe("Invalid CSV", func() {
		It("columns with missing or invalid fields", func() {
			rec := httptest.NewRecorder()
			req := prepareRequestMIMEType(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/bulk/patients", clinicId.Hex()), "./test/bulk_patients_fixtures/02_missing_fields.csv", "text/csv")
			asClinician(req)
			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("List only", func() {
		DescribeTable("Access",
			func(mutator func(*http.Request)) {
				rec := httptest.NewRecorder()
				req := prepareRequestMIMEType(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/bulk/patients", clinicId.Hex()), "./test/bulk_patients_fixtures/01_valid_patients.csv", "text/csv")
				if mutator != nil {
					mutator(req)
				}
				server.ServeHTTP(rec, req)
				Expect(rec.Result()).ToNot(BeNil())
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
				body, err := io.ReadAll(rec.Result().Body)
				Expect(err).ToNot(HaveOccurred())
				expectedResBody := `Name,Birthdate,MRN,Email,Diabetes Type,Glycemic Target,Reason,Emailed?
Thomas Jefferson,1952-01-09,112233,working+test+custodial+thomas@tidepool.org,,adaStandard,,
Ben Franklin,1952-01-10,112234,,,adaStandard,,
George Washington,1950-01-02,123456789,working+test+custodial+george@tidepool.org,,adaStandard,,
Duplicate MRN,1951-01-02,DUPLICATEMRN,duplicate+mrn+1@tidepool.org,,adaStandard,duplicate MRN,
Duplicate MRN II,1949-03-04,DUPLICATEMRN,duplicate+mrn+2@tidepool.org,,adaStandard,duplicate MRN,
Existing Email In System,1990-06-21,113322445577,existing+patient+email@tidepool.org,,adaStandard,duplicate email,
Duplicate Email,1999-05-02,98765432,duplicate+email@tidepool.org,,adaStandard,duplicate email,
Duplicate Email II,1999-06-01,98765431,duplicate+email@tidepool.org,,adaStandard,duplicate email,
Duplicate Email And MRN,2000-01-02,22222222,duplicate+email+again@tidepool.org,,adaStandard,"duplicate MRN, duplicate email",
Duplicate Email And MRN II,2000-01-03,22222222,duplicate+email+again@tidepool.org,,adaStandard,"duplicate MRN, duplicate email",
`
				Expect(string(body)).To(Equal(expectedResBody))
				// Expect no actual patients created.
				count, err := database.Collection("patients").CountDocuments(ctx, bson.M{"clinicId": clinicId, "userId": bson.M{"$ne": existingPatient.UserId}})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeNumerically("==", 0))
			},
			Entry("as server", asServer),
			Entry("as clinician", asClinician),
		)
	})

	Describe("List and Create", func() {
		Describe("as clinician", func() {
			It("minimum required fields", func() {
				rec := httptest.NewRecorder()
				req := prepareRequestMIMEType(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/bulk/patients", clinicId.Hex()), "./test/bulk_patients_fixtures/01_valid_patients.csv", "text/csv")
				params := req.URL.Query()
				params.Set("dryRun", "false")
				req.URL.RawQuery = params.Encode()
				asClinician(req)
				server.ServeHTTP(rec, req)
				Expect(rec.Result()).ToNot(BeNil())
				body, err := io.ReadAll(rec.Result().Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
				expectedResBody := `Name,Birthdate,MRN,Email,Diabetes Type,Glycemic Target,Reason,Emailed?
Thomas Jefferson,1952-01-09,112233,working+test+custodial+thomas@tidepool.org,,adaStandard,,Y
Ben Franklin,1952-01-10,112234,,,adaStandard,,N
George Washington,1950-01-02,123456789,working+test+custodial+george@tidepool.org,,adaStandard,,Y
Duplicate MRN,1951-01-02,DUPLICATEMRN,duplicate+mrn+1@tidepool.org,,adaStandard,duplicate MRN,N
Duplicate MRN II,1949-03-04,DUPLICATEMRN,duplicate+mrn+2@tidepool.org,,adaStandard,duplicate MRN,N
Existing Email In System,1990-06-21,113322445577,existing+patient+email@tidepool.org,,adaStandard,duplicate email,N
Duplicate Email,1999-05-02,98765432,duplicate+email@tidepool.org,,adaStandard,duplicate email,N
Duplicate Email II,1999-06-01,98765431,duplicate+email@tidepool.org,,adaStandard,duplicate email,N
Duplicate Email And MRN,2000-01-02,22222222,duplicate+email+again@tidepool.org,,adaStandard,"duplicate MRN, duplicate email",N
Duplicate Email And MRN II,2000-01-03,22222222,duplicate+email+again@tidepool.org,,adaStandard,"duplicate MRN, duplicate email",N
Patients Processed,10
Patients Created,3
Patients Skipped,7
Patients Emailed,2
Duplicate MRNs count,4
Duplicate emails count,5
`
				Expect(string(body)).To(Equal(expectedResBody))
				// Count new patients
				count, err := database.Collection("patients").CountDocuments(ctx, bson.M{"clinicId": clinicId, "userId": bson.M{"$ne": existingPatient.UserId}})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeNumerically("==", 3))
			})

			It("with optional fields", func() {
				rec := httptest.NewRecorder()
				req := prepareRequestMIMEType(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/bulk/patients", clinicId.Hex()), "./test/bulk_patients_fixtures/03_valid_patients_optional_columns.csv", "text/csv")
				params := req.URL.Query()
				params.Set("dryRun", "false")
				req.URL.RawQuery = params.Encode()
				asClinician(req)
				server.ServeHTTP(rec, req)
				Expect(rec.Result()).ToNot(BeNil())
				body, err := io.ReadAll(rec.Result().Body)
				Expect(err).ToNot(HaveOccurred())
				expectedResBody := `Name,Birthdate,MRN,Email,Diabetes Type,Glycemic Target,Reason,Emailed?
John Doe,1970-01-02,1111111,working+test+custodial+john+doe@tidepool.org,type1,adaStandard,,Y
Jane Doe,1971-02-03,1111112,working+test+custodial+jane+doe@tidepool.org,type2,adaStandard,,Y
Max Mustermann,1975-09-09,1111113,,type1,adaStandard,,N
Duplicate Existing Email,1995-01-23,1111114,existing+patient+email@tidepool.org,,adaStandard,duplicate email,N
Glycemic Target,1960-10-15,1111115,,,adaHighRisk,,N
Duplicate Existing MRN,1972-03-04,EXISTINGMRN,,,adaStandard,duplicate MRN,N
Patients Processed,6
Patients Created,4
Patients Skipped,2
Patients Emailed,2
Duplicate MRNs count,1
Duplicate emails count,1
`
				Expect(string(body)).To(Equal(expectedResBody))
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
				// Count new patients
				count, err := database.Collection("patients").CountDocuments(ctx, bson.M{"clinicId": clinicId, "userId": bson.M{"$ne": existingPatient.UserId}})
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(BeNumerically("==", 4))
			})
		})
	})
})
