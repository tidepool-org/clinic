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

var _ = Describe("Device Issues Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	now := time.Now().UTC().Truncate(time.Millisecond)
	staleRequestCreated := now.Add(-patients.PendingDataSourceStaleDuration - time.Hour)
	staleAt := staleRequestCreated.Add(patients.PendingDataSourceStaleDuration)
	expiredRequestCreated := now.
		Add(-patients.PendingDataSourceExpirationDuration - time.Hour)
	expiration := expiredRequestCreated.Add(patients.PendingDataSourceExpirationDuration)

	triggerCheck := func(authenticate func(*http.Request)) int {
		GinkgoHelper()

		rec := httptest.NewRecorder()
		req := prepareRequest(http.MethodPost, "/v1/device_issues", "")
		authenticate(req)

		server.ServeHTTP(rec, req)
		Expect(rec.Result()).ToNot(BeNil())
		return rec.Result().StatusCode
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

	// ageRequest moves the patient's dexcom connection request to have been created at the
	// given time, keeping its expiration PendingDataSourceExpirationDuration later.
	ageRequest := func(createdTime time.Time) {
		GinkgoHelper()

		clinicId, err := primitive.ObjectIDFromHex(*clinic.Id)
		Expect(err).ToNot(HaveOccurred())

		db := test.GetTestDatabase()
		selector := bson.M{"userId": *patient.Id, "clinicId": clinicId}
		update := bson.M{"$set": bson.M{
			"providerConnectionRequests.dexcom.0.createdTime": createdTime,
			"providerConnectionRequests.dexcom.0.expirationTime": createdTime.
				Add(patients.PendingDataSourceExpirationDuration),
		}}
		result, err := db.Collection("patients").UpdateOne(context.Background(),
			selector, update)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.ModifiedCount).To(BeEquivalentTo(1))
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
	})

	Describe("Request a dexcom connection", func() {
		It("Makes dexcom the primary issue", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/dexcom",
				*clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Dexcom),
				HaveField("Kind", BeNil()),
			)))
		})
	})

	Describe("Trigger the check while the invitation is current", func() {
		It("Is forbidden for a clinician", func() {
			Expect(triggerCheck(asClinician)).To(Equal(http.StatusForbidden))
		})

		It("Succeeds for a backend service", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))
		})

		It("Leaves the primary issue unclassified", func() {
			Expect(getPatient().PrimaryIssue).To(PointTo(HaveField("Kind", BeNil())))
		})
	})

	Describe("Trigger the check after the invitation goes stale", func() {
		var updatedTime time.Time

		It("Ages the request out-of-band", func() {
			ageRequest(staleRequestCreated)
		})

		It("Classifies the primary issue as a stale invitation", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))

			fetched := getPatient()
			Expect(fetched.PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Dexcom),
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1StaleInvite))),
				HaveField("EffectiveTime", PointTo(BeTemporally("==", staleAt))),
			)))
			updatedTime = *fetched.UpdatedTime
		})

		It("Leaves the patient alone when triggered again", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))

			fetched := getPatient()
			Expect(fetched.PrimaryIssue).To(PointTo(
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1StaleInvite)))))
			Expect(fetched.UpdatedTime).To(PointTo(BeTemporally("==", updatedTime)))
		})
	})

	Describe("Trigger the check after the invitation expires", func() {
		var updatedTime time.Time

		It("Ages the request out-of-band", func() {
			ageRequest(expiredRequestCreated)
		})

		It("Classifies the primary issue as an expired invitation", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))

			fetched := getPatient()
			Expect(fetched.PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Dexcom),
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1ExpiredInvite))),
				HaveField("EffectiveTime", PointTo(BeTemporally("==", expiration))),
			)))
			Expect(fetched.UpdatedTime).
				To(PointTo(BeTemporally("~", time.Now(), time.Minute)))
			updatedTime = *fetched.UpdatedTime
		})

		It("Leaves the patient alone when triggered again", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))

			fetched := getPatient()
			Expect(fetched.PrimaryIssue).To(PointTo(
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1ExpiredInvite)))))
			Expect(fetched.UpdatedTime).To(PointTo(BeTemporally("==", updatedTime)))
		})
	})

	Describe("Trigger the check after connected data goes stale", func() {
		var updatedTime time.Time

		// oldData is the latest data time of the data source connected below: it went
		// stale an hour ago.
		oldData := now.Add(-patients.DataSourceStaleDataDuration - time.Hour)
		staleAt := oldData.Add(patients.DataSourceStaleDataDuration)

		// putDataSource reports a connected dexcom data source created now, as the data
		// service does, with the given latest data time.
		putDataSource := func(latestDataTime time.Time) {
			GinkgoHelper()

			body, err := json.Marshal([]map[string]interface{}{{
				"state":          patients.DataSourceStateConnected,
				"providerName":   patients.DexcomDataSourceProviderName,
				"dataSourceId":   "507f1f77bcf86cd799439011",
				"createdTime":    now.Format(time.RFC3339Nano),
				"modifiedTime":   now.Format(time.RFC3339Nano),
				"latestDataTime": latestDataTime.Format(time.RFC3339Nano),
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

		It("Connects a dexcom data source whose data is stale", func() {
			putDataSource(oldData)
			// Connecting replaces the expired invitation as the primary issue.
			Expect(getPatient().PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Dexcom),
				HaveField("Kind", BeNil()),
			)))
		})

		It("Classifies the primary issue as stale data", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))

			fetched := getPatient()
			Expect(fetched.PrimaryIssue).To(PointTo(And(
				HaveField("Source", api.PrimaryIssueSourceV1Dexcom),
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1StaleData))),
				HaveField("EffectiveTime", PointTo(BeTemporally("==", staleAt))),
			)))
			updatedTime = *fetched.UpdatedTime
		})

		It("Leaves the patient alone when triggered again", func() {
			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))

			fetched := getPatient()
			Expect(fetched.PrimaryIssue).To(PointTo(
				HaveField("Kind", PointTo(Equal(api.PrimaryIssueKindV1StaleData)))))
			Expect(fetched.UpdatedTime).To(PointTo(BeTemporally("==", updatedTime)))
		})

		It("Does not classify stale data once a newer connection request exists", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/dexcom",
				*clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
			// The new request replaces the primary issue, clearing the kind.
			Expect(getPatient().PrimaryIssue).To(PointTo(HaveField("Kind", BeNil())))

			Expect(triggerCheck(asServer)).To(Equal(http.StatusNoContent))
			Expect(getPatient().PrimaryIssue).To(PointTo(HaveField("Kind", BeNil())))
		})
	})
})
