package integration_test

import (
	"bytes"
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

// The primaryIssue object is read-only. The request validator accepts read-only fields in
// request bodies, so the API must silently ignore them rather than reject them.
var _ = Describe("Primary Issue Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	dexcom := patients.DexcomDataSourceProviderName
	twiist := patients.TwiistDataSourceProviderName
	connected := patients.DataSourceStateConnected

	// seededTime is the effective time set out-of-band below. Mongo stores dates at
	// millisecond precision, so it is truncated to compare equal after a round trip.
	seededTime := time.Now().UTC().Truncate(time.Millisecond)

	patientEndpoint := func() string {
		return fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
	}

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

	// setStoredIssue bypasses the API, standing in for the backend services that own the
	// primary issue.
	setStoredIssue := func(update bson.M) {
		GinkgoHelper()

		db := test.GetTestDatabase()
		result, err := db.Collection("patients").UpdateOne(context.Background(),
			patientSelector(), bson.M{"$set": update})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.MatchedCount).To(BeEquivalentTo(1))
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

	expectStoredSource := func(source string) {
		GinkgoHelper()
		Expect(storedPatient().PrimaryIssue).To(PointTo(HaveField("Source", source)))
	}

	expectStoredKind := func(kind string) {
		GinkgoHelper()
		Expect(storedPatient().PrimaryIssue).To(PointTo(HaveField("Kind", kind)))
	}

	// expectSeededIssue matches the issue set out-of-band in "Set the primary issue
	// out-of-band", which the client-driven updates below must leave in place.
	expectSeededIssue := func(issue *api.PrimaryIssueV1) {
		GinkgoHelper()
		Expect(issue).To(PointTo(And(
			HaveField("Source", api.PrimaryIssueSourceV1Abbott),
			HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1StaleData))),
			HaveField("EffectiveTime", PointTo(BeTemporally("==", seededTime))),
		)))
	}

	// putDataSource reports a single data source for the patient, as the data service does,
	// replacing whatever data sources were stored before.
	putDataSource := func(provider, state string, modifiedTime time.Time) {
		GinkgoHelper()

		body, err := json.Marshal([]map[string]interface{}{{
			"state":        state,
			"providerName": provider,
			"dataSourceId": "507f1f77bcf86cd799439011",
			// Created when reported, so the data source is newer than any request made
			// earlier in the flow. Keep sub-second precision, so comparisons against
			// seeded issues are decided by time rather than by provider precedence.
			"createdTime":  modifiedTime.UTC().Format(time.RFC3339Nano),
			"modifiedTime": modifiedTime.UTC().Format(time.RFC3339Nano),
		}})
		Expect(err).ToNot(HaveOccurred())

		rec := httptest.NewRecorder()
		endpoint := fmt.Sprintf("/v1/patients/%s/data_sources", *patient.Id)
		req := prepareRequestWithBody(http.MethodPut, endpoint, bytes.NewReader(body))
		asServer(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
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

	Describe("Create a patient with a primary issue", func() {
		It("Succeeds but ignores the field", func() {
			body, err := json.Marshal(map[string]interface{}{
				"fullName":  "Timothy Bixby",
				"birthDate": "2008-01-06",
				"mrn":       "0000000001",
				"email":     "test@tidepool.org",
				"primaryIssue": map[string]interface{}{
					"source":        "dexcom",
					"kind":          "erroring",
					"effectiveTime": "2020-01-01T00:00:00Z",
				},
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
			// The custodial invitation sent at creation is the primary issue, not the
			// value supplied in the body.
			Expect(patient.PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1DeviceNonSpecificInvite),
				HaveField("Kind", BeNil()),
				HaveField("EffectiveTime",
					PointTo(BeTemporally("~", time.Now(), time.Minute))),
			)))
		})

		It("Records the invitation rather than the supplied value", func() {
			expectStoredSource(patients.PrimaryIssueSourceDeviceNonSpecificInvite)
			expectStoredKind("")
		})
	})

	Describe("Set the primary issue out-of-band", func() {
		It("Succeeds", func() {
			setStoredIssue(bson.M{"primaryIssue": patients.PrimaryIssue{
				Source:        patients.AbbottDataSourceProviderName,
				Kind:          patients.PrimaryIssueKindStaleData,
				EffectiveTime: seededTime,
			}})
		})

		It("Is returned when the patient is fetched", func() {
			expectSeededIssue(getPatient().PrimaryIssue)
		})
	})

	Describe("Update the patient with a different primary issue", func() {
		It("Succeeds but ignores the field", func() {
			updated := updatePatient(func(body map[string]interface{}) {
				body["primaryIssue"] = map[string]interface{}{
					"source":        "twiist",
					"kind":          "disconnected",
					"effectiveTime": "2030-01-01T00:00:00Z",
				}
			})
			expectSeededIssue(updated.PrimaryIssue)
		})

		It("Keeps the previous value when the patient is fetched", func() {
			expectSeededIssue(getPatient().PrimaryIssue)
		})

		It("Keeps the previous value in the database", func() {
			expectStoredSource(patients.AbbottDataSourceProviderName)
			expectStoredKind(patients.PrimaryIssueKindStaleData)
		})
	})

	Describe("Update the patient without a primary issue", func() {
		It("Succeeds", func() {
			updated := updatePatient(func(body map[string]interface{}) {
				delete(body, "primaryIssue")
				body["fullName"] = "Timothy A. Bixby"
			})
			Expect(updated.FullName).To(Equal("Timothy A. Bixby"))
			expectSeededIssue(updated.PrimaryIssue)
		})

		It("Keeps the previous value in the database", func() {
			expectStoredSource(patients.AbbottDataSourceProviderName)
			expectStoredKind(patients.PrimaryIssueKindStaleData)
		})
	})

	Describe("A primary issue with unusual stored values", func() {
		It("Is returned without a kind when it hasn't been classified", func() {
			setStoredIssue(bson.M{"primaryIssue": patients.PrimaryIssue{
				Source:        patients.AbbottDataSourceProviderName,
				EffectiveTime: seededTime,
			}})
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Abbott),
				HaveField("Kind", BeNil()),
			)))
		})

		It("Is returned without a kind when the stored kind is unknown", func() {
			setStoredIssue(bson.M{"primaryIssue.kind": "notAKnownKind"})
			Expect(getPatient().PrimaryIssue).To(PointTo(HaveField("Kind", BeNil())))
		})

		It("Is returned without an effective time when the stored time is zero", func() {
			setStoredIssue(bson.M{"primaryIssue": patients.PrimaryIssue{
				Source: patients.AbbottDataSourceProviderName,
				Kind:   patients.PrimaryIssueKindStaleData,
			}})
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1StaleData))),
				HaveField("EffectiveTime", BeNil()),
			)))
		})
	})

	Describe("A data source becomes connected", func() {
		It("Makes its provider the primary issue source and clears the kind", func() {
			putDataSource(dexcom, connected, time.Now())
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Dexcom),
				HaveField("Kind", BeNil()),
			)))
			expectStoredSource(patients.DexcomDataSourceProviderName)
			expectStoredKind("")
		})

		It("Can be classified out-of-band", func() {
			setStoredIssue(bson.M{"primaryIssue.kind": patients.PrimaryIssueKindErroring})
			Expect(getPatient().PrimaryIssue).To(PointTo(
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1Erroring)))))
		})

		It("Yields to a later connection request, which clears the kind", func() {
			rec := httptest.NewRecorder()
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/twiist",
				*clinic.Id, *patient.Id)
			req := prepareRequest(http.MethodPost, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Twiist),
				HaveField("Kind", BeNil()),
			)))
			expectStoredKind("")
		})

		It("Does not reclaim the primary issue while it stays connected", func() {
			putDataSource(dexcom, connected, time.Now().Add(time.Hour))
			Expect(getPatient().PrimaryIssue).
				To(PointTo(HaveField("Source", api.PrimaryIssueSourceV1Twiist)))
		})
	})

	Describe("A connected data source fails", func() {
		var classifiedTime time.Time

		It("Is ignored when the primary issue is about another provider", func() {
			// dexcom is connected, but twiist holds the primary issue.
			putDataSource(dexcom, patients.DataSourceStateDisconnected, time.Now())
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Twiist),
				HaveField("Kind", BeNil()),
			)))
		})

		It("Classifies the primary issue when its provider errors", func() {
			putDataSource(twiist, connected, time.Now())
			Expect(getPatient().PrimaryIssue).To(PointTo(HaveField("Kind", BeNil())))

			putDataSource(twiist, patients.DataSourceStateError, time.Now())
			issue := getPatient().PrimaryIssue
			Expect(issue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Twiist),
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1Erroring))),
				HaveField("EffectiveTime",
					PointTo(BeTemporally("~", time.Now(), time.Minute))),
			)))
			classifiedTime = *issue.EffectiveTime
			expectStoredKind(patients.PrimaryIssueKindErroring)
		})

		It("Is not reclassified by a repeated failure report", func() {
			putDataSource(twiist, patients.DataSourceStateError, time.Now())
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1Erroring))),
				HaveField("EffectiveTime", PointTo(BeTemporally("==", classifiedTime))),
			)))
		})
	})
})
