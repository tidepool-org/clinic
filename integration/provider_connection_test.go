package integration_test

import (
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

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/outbox"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Provider Connection Integration Test", Ordered, func() {
	var clinic client.ClinicV1
	var patient api.PatientV1

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/common_fixtures/01_create_clinic.json")
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
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", *clinic.Id), "./test/common_fixtures/02_create_patient.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})
	})

	Describe("Update Data Sources", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/patients/%s/data_sources", *patient.Id), "./test/common_fixtures/03_update_data_sources.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Send Dexcom Connection Request", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/dexcom", *clinic.Id, *patient.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Get updated patient succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})

		It("Adds the connection request", func() {
			Expect(patient.ConnectionRequests.Dexcom).To(HaveLen(1))
			Expect(patient.ConnectionRequests.Dexcom[0].ProviderName).To(Equal(api.Dexcom))
		})
	})

	Describe("Send Twiist Connection Request", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/twiist", *clinic.Id, *patient.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Get updated patient succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})

		It("Adds the connection request", func() {
			Expect(patient.ConnectionRequests.Twiist).To(HaveLen(1))
			Expect(patient.ConnectionRequests.Twiist[0].ProviderName).To(Equal(api.Twiist))
		})
	})

	Describe("Send a subsequent Dexom Connection Request", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/dexcom", *clinic.Id, *patient.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Get updated patient succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})

		It("Adds the connection request in correct order", func() {
			Expect(patient.ConnectionRequests.Dexcom).To(HaveLen(2))
			Expect(patient.ConnectionRequests.Dexcom[0].ProviderName).To(Equal(api.Dexcom))
			Expect(patient.ConnectionRequests.Dexcom[0].CreatedTime).To(Not(BeZero()))
			Expect(patient.ConnectionRequests.Dexcom[1].ProviderName).To(Equal(api.Dexcom))
			Expect(patient.ConnectionRequests.Dexcom[1].CreatedTime).To(Not(BeZero()))
			Expect(patient.ConnectionRequests.Dexcom[0].CreatedTime).To(BeTemporally(">", patient.ConnectionRequests.Dexcom[1].CreatedTime))
		})
	})

	Describe("Send Any Provider Connection Request", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients/%s/connect/any", *clinic.Id, *patient.Id), "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Get updated patient succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			dec := json.NewDecoder(rec.Result().Body)
			Expect(dec.Decode(&patient)).To(Succeed())
			Expect(patient.Id).To(PointTo(Not(BeEmpty())))
		})

		It("Adds the generic connection request", func() {
			Expect(patient.ConnectionRequests.Any).To(HaveLen(1))
			Expect(patient.ConnectionRequests.Any[0].ProviderName).To(Equal(api.Any))
		})

		It("Creates an outbox event with correct payload", func() {
			database := dbTest.GetTestDatabase()
			collection := database.Collection(outbox.CollectionName)

			var event outbox.Event
			err := collection.FindOne(context.Background(), bson.M{
				"eventType": string(outbox.EventTypeSendProviderConnectionEmail),
			}).Decode(&event)
			Expect(err).ToNot(HaveOccurred())

			Expect(event.EventType).To(Equal(outbox.EventTypeSendProviderConnectionEmail))
			Expect(event.CreatedTime).ToNot(BeZero())

			var payload outbox.SendProviderConnectionEmailPayload
			Expect(bson.Unmarshal(event.Payload, &payload)).To(Succeed())
			Expect(payload.ClinicId).To(Equal(*clinic.Id))
			Expect(payload.ClinicName).To(Equal(clinic.Name))
			Expect(payload.ProviderName).To(Equal("any"))
		})
	})
})
