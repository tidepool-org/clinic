package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

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

// Marking an invitation as re-sent is a backend-only operation that makes the
// device-non-specific invitation the patient's primary issue.
var _ = Describe("Patient Invitation Re-sent Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	patientSelector := func() bson.M {
		GinkgoHelper()

		clinicId, err := primitive.ObjectIDFromHex(*clinic.Id)
		Expect(err).ToNot(HaveOccurred())
		return bson.M{"userId": *patient.Id, "clinicId": clinicId}
	}

	storedPatient := func() patients.Patient {
		GinkgoHelper()

		stored := patients.Patient{}
		db := test.GetTestDatabase()
		Expect(db.Collection("patients").FindOne(context.Background(), patientSelector()).
			Decode(&stored)).To(Succeed())
		return stored
	}

	getPatient := func() api.PatientV1 {
		GinkgoHelper()

		endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodGet, endpoint, "")
		asClinician(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

		fetched := api.PatientV1{}
		Expect(json.NewDecoder(rec.Result().Body).Decode(&fetched)).To(Succeed())
		return fetched
	}

	markResent := func(patientId string, authenticate func(*http.Request)) int {
		GinkgoHelper()

		endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s/invitation_resent",
			*clinic.Id, patientId)
		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodPost, endpoint, "")
		authenticate(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		return rec.Result().StatusCode
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

	Describe("Create a patient", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id)
			req := prepareRequest(http.MethodPost, endpoint,
				"./test/common_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			Expect(json.NewDecoder(rec.Result().Body).Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})

		It("Has a primary issue after it is set out-of-band", func() {
			db := test.GetTestDatabase()
			update := bson.M{"$set": bson.M{"primaryIssue": patients.PrimaryIssue{
				Source:        patients.AbbottDataSourceProviderName,
				EffectiveTime: time.Now(),
			}}}
			result, err := db.Collection("patients").UpdateOne(context.Background(),
				patientSelector(), update)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.MatchedCount).To(BeEquivalentTo(1))
			Expect(getPatient().PrimaryIssue).
				To(PointTo(HaveField("Source", api.PrimaryIssueSourceV1Abbott)))
		})
	})

	Describe("Mark the invitation re-sent as a clinician", func() {
		It("Is forbidden", func() {
			Expect(markResent(*patient.Id, asClinician)).To(Equal(http.StatusForbidden))
		})

		It("Leaves the primary issue in place", func() {
			Expect(getPatient().PrimaryIssue).
				To(PointTo(HaveField("Source", api.PrimaryIssueSourceV1Abbott)))
		})
	})

	Describe("Mark the invitation re-sent as a backend service", func() {
		It("Succeeds", func() {
			Expect(markResent(*patient.Id, asServer)).To(Equal(http.StatusNoContent))
		})

		It("Makes the invitation the primary issue", func() {
			Expect(getPatient().PrimaryIssue).To(PointTo(
				HaveField("Source", api.PrimaryIssueSourceV1DeviceNonSpecificInvite)))
			Expect(storedPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", patients.PrimaryIssueSourceDeviceNonSpecificInvite),
				HaveField("EffectiveTime", BeTemporally("~", time.Now(), time.Minute)),
			)))
		})

		It("Succeeds again when the invitation is already the primary issue", func() {
			Expect(markResent(*patient.Id, asServer)).To(Equal(http.StatusNoContent))
			Expect(getPatient().PrimaryIssue).To(PointTo(
				HaveField("Source", api.PrimaryIssueSourceV1DeviceNonSpecificInvite)))
		})

		It("Returns not found for an unknown patient", func() {
			Expect(markResent("0000000000", asServer)).To(Equal(http.StatusNotFound))
		})
	})
})
