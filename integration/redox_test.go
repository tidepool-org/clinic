package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

var _ = Describe("Redox Integration Test", Ordered, func() {
	var clinic client.Clinic
	var patient client.Patient
	var documentId string

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/redox_fixtures/01_create_clinic.json")
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

	Describe("Enable Redox integration", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/settings/ehr", *clinic.Id), "./test/redox_fixtures/02_enable_redox.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
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

	Describe("Get Patient by MRN", func() {
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

			response := client.PatientsResponse{}
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
	})

	Describe("Send subscription order", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/redox", "./test/redox_fixtures/04_enable_reports_order.json")
			asRedox(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Persists the order", func() {
			db := test.GetTestDatabase()
			Expect(db).ToNot(BeNil())

			document := redox_models.MessageEnvelope{}
			err := db.Collection("redox").FindOne(context.Background(), bson.M{
				"meta.Logs.ID": "d9f5d293-7110-461e-a875-3beb089e79f3",
			}).Decode(&document)
			Expect(err).ToNot(HaveOccurred())
			Expect(document.Id).ToNot(Equal(primitive.NilObjectID))

			documentId = document.Id.Hex()
		})

		It("Matches the order by DOB and Full Name", func() {
			body := fmt.Sprintf(`{
				"messageRef": {
					"dataModel": "Order", 
					"eventType": "New", 
					"documentId": "%s"
				},
                "patients": {
					"criteria": ["DOB_FULLNAME"]
                }
			}`, documentId)
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost, "/v1/redox/match", bytes.NewBufferString(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var response client.EHRMatchResponse
			Expect(json.NewDecoder(rec.Result().Body).Decode(&response)).To(Succeed())
			Expect(response.Patients).ToNot(BeNil())
			Expect(response.Patients).To(PointTo(HaveLen(1)))

			expectedMRN := "0000000001"
			expectedFullName := "Timothy Bixby"
			expectedDateOfBirth := "2008-01-06"
			Expect((*response.Patients)[0].Mrn).To(PointTo(Equal(expectedMRN)))
			Expect((*response.Patients)[0].FullName).To(Equal(expectedFullName))
			Expect((*response.Patients)[0].BirthDate.String()).To(Equal(expectedDateOfBirth))
		})

		It("Doesn't enable the Redox subscription for the patient when sending only match request without action", func() {
			db := test.GetTestDatabase()
			Expect(db).ToNot(BeNil())

			clinicId, _ := primitive.ObjectIDFromHex(*clinic.Id)
			selector := bson.M{
				"userId":   *patient.Id,
				"clinicId": clinicId,
			}

			p := patients.Patient{}
			err := db.Collection("patients").FindOne(context.Background(), selector).Decode(&p)
			Expect(err).ToNot(HaveOccurred())
			Expect(p.EHRSubscriptions).To(HaveLen(0))
		})

		It("Matches the order by MRN and DOB successfully", func() {
			body := fmt.Sprintf(`{
				"messageRef": {
					"dataModel": "Order", 
					"eventType": "New", 
					"documentId": "%s"
				},
                "patients": {
					"criteria": ["MRN_DOB"],
	            	"onUniqueMatch": "ENABLE_REPORTS"
                }
			}`, documentId)
			rec := httptest.NewRecorder()
			req := prepareRequestWithBody(http.MethodPost, "/v1/redox/match", bytes.NewBufferString(body))
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			var response client.EHRMatchResponse
			Expect(json.NewDecoder(rec.Result().Body).Decode(&response)).To(Succeed())
			Expect(response.Patients).ToNot(BeNil())
			Expect(response.Patients).To(PointTo(HaveLen(1)))

			expectedMRN := "0000000001"
			expectedFullName := "Timothy Bixby"
			expectedDateOfBirth := "2008-01-06"
			Expect((*response.Patients)[0].Mrn).To(PointTo(Equal(expectedMRN)))
			Expect((*response.Patients)[0].FullName).To(Equal(expectedFullName))
			Expect((*response.Patients)[0].BirthDate.String()).To(Equal(expectedDateOfBirth))
		})

		It("Enables the Redox subscription for the patient", func() {
			db := test.GetTestDatabase()
			Expect(db).ToNot(BeNil())

			clinicId, _ := primitive.ObjectIDFromHex(*clinic.Id)
			selector := bson.M{
				"userId":   *patient.Id,
				"clinicId": clinicId,
			}

			p := patients.Patient{}
			err := db.Collection("patients").FindOne(context.Background(), selector).Decode(&p)
			Expect(err).ToNot(HaveOccurred())
			Expect(p.EHRSubscriptions).To(HaveLen(1))

			subscription, ok := p.EHRSubscriptions["summaryAndReports"]
			Expect(ok).To(BeTrue())
			Expect(subscription.Active).To(BeTrue())
			Expect(subscription.Provider).To(Equal("redox"))
		})
	})

	Describe("Sync EHR data of all patients", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/ehr/sync", *clinic.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusAccepted))
		})

		It("Schedules the order for re-execution", func() {
			db := test.GetTestDatabase()
			Expect(db).ToNot(BeNil())

			res, err := db.Collection("scheduledSummaryAndReportsOrders").CountDocuments(context.Background(), bson.M{
				"userId": *patient.Id,
				"precedingDocument._id": bson.M{
					"$exists": false,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(int64(1)))
		})
	})

	Describe("Sync EHR data of a single patient", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/patients/%s/ehr/sync", *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusAccepted))
		})

		It("Schedules the order for re-execution", func() {
			db := test.GetTestDatabase()
			Expect(db).ToNot(BeNil())

			type rescheduled struct {
				Id                primitive.ObjectID            `bson:"_id"`
				UserId            string                        `bson:"userId"`
				ClinicId          primitive.ObjectID            `bson:"clinicId"`
				CreatedTime       time.Time                     `bson:"createdTime"`
				LastMatchedOrder  *redox_models.MessageEnvelope `bson:"lastMatchedOrder"`
				PrecedingDocument *rescheduled                  `bson:"precedingDocument"`
			}

			var r rescheduled
			cur, err := db.Collection("scheduledSummaryAndReportsOrders").Find(context.Background(), bson.M{
				"userId": *patient.Id,
				"precedingDocument._id": bson.M{
					"$exists": true,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(cur.Next(context.Background())).To(BeTrue())
			Expect(cur.Decode(&r)).To(Succeed())
			Expect(r.UserId).To(Equal(*patient.Id))
			Expect(r.ClinicId.Hex()).To(Equal(*clinic.Id))
			Expect(r.CreatedTime).To(BeTemporally("~", time.Now(), 5*time.Second))
			Expect(r.LastMatchedOrder).ToNot(BeNil())
			Expect(r.LastMatchedOrder.Id.IsZero()).To(BeFalse())
			Expect(r.PrecedingDocument).ToNot(BeNil())
			Expect(r.PrecedingDocument.Id.IsZero()).To(BeFalse())

			order := redox_models.NewOrder{}
			Expect(bson.Unmarshal(r.LastMatchedOrder.Message, &order)).To(Succeed())
			Expect(order.Order.ID).To(Equal("157968300"))

			Expect(cur.Next(context.Background())).To(BeFalse())
		})
	})

	Describe("Update Summary", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/patients/%s/summary", *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "./test/redox_fixtures/05_update_summary.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Delete patient", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})
	})
})
