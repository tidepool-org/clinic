package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/store/test"
)

// The primaryIssueProvider field is read-only. The request validator accepts read-only
// fields in request bodies, so the API must silently ignore them rather than reject them.
var _ = Describe("Primary Issue Provider Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	patientEndpoint := func() string {
		return fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
	}

	patientDocument := func() bson.M {
		GinkgoHelper()

		clinicId, err := primitive.ObjectIDFromHex(*clinic.Id)
		Expect(err).ToNot(HaveOccurred())
		selector := bson.M{"userId": *patient.Id, "clinicId": clinicId}

		doc := bson.M{}
		db := test.GetTestDatabase()
		Expect(db.Collection("patients").FindOne(context.Background(), selector).
			Decode(&doc)).To(Succeed())
		return doc
	}

	getPatient := func() api.PatientV1 {
		GinkgoHelper()

		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, patientEndpoint(), "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		fetched := api.PatientV1{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&fetched)).To(Succeed())
		return fetched
	}

	// updatePatient sends the fetched patient back like a client would, with the
	// read-only fields a real client strips removed, and mutate applied to the body.
	updatePatient := func(mutate func(body map[string]interface{})) api.PatientV1 {
		GinkgoHelper()

		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, patientEndpoint(), "")
		asClinician(req)
		server.ServeHTTP(rec, req)
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		update := map[string]interface{}{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&update)).To(Succeed())
		for _, field := range updateClinicPatientOmittedFields {
			delete(update, field)
		}
		mutate(update)

		body, err := json.Marshal(update)
		Expect(err).ToNot(HaveOccurred())

		rec = httptest.NewRecorder()
		req = prepareRequestWithBody(http.MethodPut, patientEndpoint(),
			bytes.NewReader(body))
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		updated := api.PatientV1{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&updated)).To(Succeed())
		return updated
	}

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics",
				"./test/common_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			Expect(json.NewDecoder(rec.Result().Body).Decode(&clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Create a patient with a primary issue provider", func() {
		It("Succeeds but ignores the field", func() {
			body, err := json.Marshal(map[string]interface{}{
				"fullName":             "Timothy Bixby",
				"birthDate":            "2008-01-06",
				"mrn":                  "0000000001",
				"email":                "test@tidepool.org",
				"primaryIssueProvider": "dexcom",
			})
			Expect(err).ToNot(HaveOccurred())

			rec := httptest.NewRecorder()
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id)
			req := prepareRequestWithBody(http.MethodPost, endpoint, bytes.NewReader(body))
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			Expect(json.NewDecoder(rec.Result().Body).Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
			Expect(patient.PrimaryIssueProvider).To(BeNil())
		})

		It("Does not persist the field", func() {
			Expect(patientDocument()).ToNot(HaveKey("primaryIssueProvider"))
		})
	})

	Describe("Set the primary issue provider out-of-band", func() {
		It("Succeeds", func() {
			clinicId, err := primitive.ObjectIDFromHex(*clinic.Id)
			Expect(err).ToNot(HaveOccurred())

			db := test.GetTestDatabase()
			selector := bson.M{"userId": *patient.Id, "clinicId": clinicId}
			update := bson.M{"$set": bson.M{"primaryIssueProvider": "abbott"}}
			result, err := db.Collection("patients").UpdateOne(context.Background(),
				selector, update)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.MatchedCount).To(BeEquivalentTo(1))
		})

		It("Is returned when the patient is fetched", func() {
			Expect(getPatient().PrimaryIssueProvider).To(PointTo(Equal(api.Abbott)))
		})
	})

	Describe("Update the patient with a different primary issue provider", func() {
		It("Succeeds but ignores the field", func() {
			updated := updatePatient(func(body map[string]interface{}) {
				body["primaryIssueProvider"] = "twiist"
			})
			Expect(updated.PrimaryIssueProvider).To(PointTo(Equal(api.Abbott)))
		})

		It("Keeps the previous value when the patient is fetched", func() {
			Expect(getPatient().PrimaryIssueProvider).To(PointTo(Equal(api.Abbott)))
		})

		It("Keeps the previous value in the database", func() {
			Expect(patientDocument()).To(HaveKeyWithValue("primaryIssueProvider", "abbott"))
		})
	})

	Describe("Update the patient without a primary issue provider", func() {
		It("Succeeds", func() {
			updated := updatePatient(func(body map[string]interface{}) {
				delete(body, "primaryIssueProvider")
				body["fullName"] = "Timothy A. Bixby"
			})
			Expect(updated.FullName).To(Equal("Timothy A. Bixby"))
			Expect(updated.PrimaryIssueProvider).To(PointTo(Equal(api.Abbott)))
		})

		It("Keeps the previous value in the database", func() {
			Expect(patientDocument()).To(HaveKeyWithValue("primaryIssueProvider", "abbott"))
		})
	})
})
