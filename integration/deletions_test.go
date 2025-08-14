package integration_test

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	integrationTest "github.com/tidepool-org/clinic/integration/test"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

var _ = Describe("Deletions Test", Ordered, func() {
	var clinic client.ClinicV1
	var clinician client.ClinicianV1

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/deletions_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Create Clinician", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/clinicians", *clinic.Id), "./test/deletions_fixtures/02_create_clinician.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Returns the created clinician", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/clinicians/%s", *clinic.Id, "1111111111")
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinician)).To(Succeed())
			Expect(clinician.Id).ToNot(BeNil())
		})
	})

	Describe("Create Patient", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients", *clinic.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "./test/redox_fixtures/03_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Delete Patient", func() {
		var patient client.PatientV1

		It("Returns the patient", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients?search=%s", *clinic.Id, "0000000001")
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())

			response := client.PatientsResponseV1{}
			Expect(json.Unmarshal(body, &response)).To(Succeed())
			Expect(response.Data).ToNot(BeNil())
			Expect(response.Meta).ToNot(BeNil())
			Expect(response.Meta.Count).To(PointTo(Equal(1)))
			Expect(response.Data).To(PointTo(HaveLen(1)))

			patient = (*response.Data)[0]
			Expect(patient.Id).ToNot(BeNil())
			Expect(patient.Mrn).To(PointTo(Equal("0000000001")))
			Expect(patient.BirthDate.String()).To(Equal("2008-01-06"))
		})

		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Deleted the patient", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})

		It("Creates a deletion record for the deleted patient", func() {
			db := test.GetTestDatabase()
			collection := db.Collection("patient_deletions")

			var deletion struct {
				DeletedTime     time.Time        `bson:"deletedTime"`
				DeletedByUserId *string          `bson:"deletedByUserId"`
				Patient         patients.Patient `bson:"patient"`
			}

			err := collection.FindOne(testCtx(), bson.M{"patient.userId": *patient.Id}).Decode(&deletion)
			Expect(err).ToNot(HaveOccurred())
			Expect(deletion.DeletedTime).ToNot(BeZero())
			Expect(deletion.DeletedByUserId).To(PointTo(Equal(integrationTest.TestUserId)))
			Expect(deletion.Patient.UserId).To(PointTo(Equal(*patient.Id)))
		})
	})

	Describe("Delete Clinician", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/clinicians/%s", *clinic.Id, *clinician.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Deleted the clinician", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/clinicians/%s", *clinic.Id, "1111111111")
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})

		It("Creates a deletion record for the deleted clinician", func() {
			db := test.GetTestDatabase()
			collection := db.Collection("clinician_deletions")

			var deletion struct {
				DeletedTime     time.Time            `bson:"deletedTime"`
				DeletedByUserId *string              `bson:"deletedByUserId"`
				Clinician       clinicians.Clinician `bson:"clinician"`
			}

			err := collection.FindOne(testCtx(), bson.M{"clinician.userId": *clinician.Id}).Decode(&deletion)
			Expect(err).ToNot(HaveOccurred())
			Expect(deletion.DeletedTime).ToNot(BeZero())
			Expect(deletion.DeletedByUserId).To(PointTo(Equal(integrationTest.TestUserId)))
			Expect(deletion.Clinician.UserId).To(PointTo(Equal(*clinician.Id)))
		})
	})

	Describe("Delete Clinic", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v", *clinic.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Deleted the clinic", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v", *clinic.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNotFound))
		})

		It("Creates a deletion record for the deleted clinic", func() {
			db := test.GetTestDatabase()
			collection := db.Collection("clinic_deletions")

			var deletion struct {
				DeletedTime     time.Time      `bson:"deletedTime"`
				DeletedByUserId *string        `bson:"deletedByUserId"`
				Clinic          clinics.Clinic `bson:"clinic"`
			}

			objId, _ := primitive.ObjectIDFromHex(*clinic.Id)
			err := collection.FindOne(testCtx(), bson.M{"clinic._id": objId}).Decode(&deletion)
			Expect(err).ToNot(HaveOccurred())
			Expect(deletion.DeletedTime).ToNot(BeZero())
			Expect(deletion.DeletedByUserId).To(PointTo(Equal(integrationTest.TestUserId)))
			Expect(deletion.Clinic.Id.Hex()).To(Equal(*clinic.Id))
		})

		It("Creates a deletion record for the deleted clinic admin", func() {
			db := test.GetTestDatabase()
			collection := db.Collection("clinician_deletions")

			var deletion struct {
				DeletedTime     time.Time            `bson:"deletedTime"`
				DeletedByUserId *string              `bson:"deletedByUserId"`
				Clinician       clinicians.Clinician `bson:"clinician"`
			}

			err := collection.FindOne(testCtx(), bson.M{"clinician.userId": integrationTest.TestUserId}).Decode(&deletion)
			Expect(err).ToNot(HaveOccurred())
			Expect(deletion.DeletedTime).ToNot(BeZero())
			Expect(deletion.DeletedByUserId).To(PointTo(Equal(integrationTest.TestUserId)))
			Expect(deletion.Clinician.UserId).To(PointTo(Equal(integrationTest.TestUserId)))
		})
	})

})
