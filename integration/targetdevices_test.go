package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store/test"
)

// updateClinicPatientOmittedFields are the read-only fields the uploader's platform client
// strips from a fetched patient before sending an update (tidepool-platform-client
// clinics.js, updateClinicPatient).
var updateClinicPatientOmittedFields = []string{
	"clinicId",
	"createdTime",
	"ehrSubscriptions",
	"id",
	"isMigrated",
	"legacyClinicianIds",
	"invitedBy",
	"lastUploadReminderTime",
	"permissions",
	"reviews",
	"summary",
	"updatedTime",
	"userId",
}

var _ = Describe("Target Devices Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	fetchPatientLikeUploader := func() map[string]interface{} {
		GinkgoHelper()

		endpoint := fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id)
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, endpoint, "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		body, err := io.ReadAll(rec.Result().Body)
		Expect(err).ToNot(HaveOccurred())

		response := struct {
			Data []map[string]interface{} `json:"data"`
		}{}
		Expect(json.Unmarshal(body, &response)).To(Succeed())

		for _, p := range response.Data {
			if p["id"] == *patient.Id {
				return p
			}
		}

		Fail(fmt.Sprintf("patient %s not found in clinic patient list", *patient.Id))
		return nil
	}

	updatePatientLikeUploader := func(targetDevices []string) api.PatientV1 {
		GinkgoHelper()

		update := fetchPatientLikeUploader()
		for _, field := range updateClinicPatientOmittedFields {
			delete(update, field)
		}
		update["targetDevices"] = targetDevices

		body, err := json.Marshal(update)
		Expect(err).ToNot(HaveOccurred())

		endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
		rec := httptest.NewRecorder()
		req := prepareRequestWithBody(http.MethodPut, endpoint, bytes.NewReader(body))
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		updated := api.PatientV1{}
		dec := json.NewDecoder(rec.Result().Body)
		Expect(dec.Decode(&updated)).To(Succeed())
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

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Create Patient", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients",
				*clinic.Id), "./test/common_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})

		It("Has no target devices", func() {
			Expect(patient.TargetDevices).To(BeNil())
		})
	})

	Describe("Set target devices", func() {
		It("Succeeds", func() {
			updated := updatePatientLikeUploader([]string{"medtronic", "omnipod"})
			Expect(updated.TargetDevices).
				To(PointTo(Equal([]string{"medtronic", "omnipod"})))
		})

		It("Returns the devices when the patient list is fetched", func() {
			fetched := fetchPatientLikeUploader()
			Expect(fetched["targetDevices"]).
				To(Equal([]interface{}{"medtronic", "omnipod"}))
		})
	})

	Describe("Change the device selection", func() {
		It("Succeeds", func() {
			updated := updatePatientLikeUploader([]string{"tandem"})
			Expect(updated.TargetDevices).To(PointTo(Equal([]string{"tandem"})))
		})

		It("Replaces the previous selection when the patient list is fetched", func() {
			fetched := fetchPatientLikeUploader()
			Expect(fetched["targetDevices"]).To(Equal([]interface{}{"tandem"}))
		})

		It("Persists the devices in the database", func() {
			db := test.GetTestDatabase()
			clinicId, err := primitive.ObjectIDFromHex(*clinic.Id)
			Expect(err).ToNot(HaveOccurred())

			selector := bson.M{"userId": *patient.Id, "clinicId": clinicId}
			p := patients.Patient{}
			Expect(db.Collection("patients").FindOne(context.Background(), selector).
				Decode(&p)).To(Succeed())
			Expect(p.TargetDevices).To(PointTo(Equal([]string{"tandem"})))
		})
	})
})
