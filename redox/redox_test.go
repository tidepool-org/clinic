package redox_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/redox"
	models "github.com/tidepool-org/clinic/redox_models"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

var _ = Describe("Redox", func() {
	var database *mongo.Database
	var collection *mongo.Collection
	var handler redox.Redox

	var clinicsService *clinicsTest.MockService
	var patientsService *patientsTest.MockService
	var patientsCtrl *gomock.Controller
	var clinicsCtrl *gomock.Controller

	BeforeEach(func() {
		database = dbTest.GetTestDatabase()
		collection = database.Collection("redox")
		config := redox.Config{
			VerificationToken: "super-secret-token",
		}
		lifecycle := fxtest.NewLifecycle(GinkgoT())

		patientsCtrl = gomock.NewController(GinkgoT())
		clinicsCtrl = gomock.NewController(GinkgoT())

		patientsService = patientsTest.NewMockService(patientsCtrl)
		clinicsService = clinicsTest.NewMockService(clinicsCtrl)

		var err error
		handler, err = redox.NewHandler(config, clinicsService, patientsService, database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(handler).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	AfterEach(func() {
		_, err := collection.DeleteMany(context.Background(), bson.M{})
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("VerifyEndpoint", func() {
		It("Returns the challenge when the token is correct", func() {
			challenge := "1234567890"
			result, err := handler.VerifyEndpoint(redox.VerificationRequest{
				VerificationToken: "super-secret-token",
				Challenge:         challenge,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
		})

		It("Returns unauthorized error when the token is incorrect", func() {
			challenge := "1234567890"
			_, err := handler.VerifyEndpoint(redox.VerificationRequest{
				VerificationToken: "incorrect-token",
				Challenge:         challenge,
			})
			Expect(err).To(MatchError(errors.Unauthorized))
		})
	})

	Describe("AuthorizeRequest", func() {
		It("Doesn't return an error when the token is correct", func() {
			req := http.Request{Header: make(http.Header)}
			req.Header.Set("verification-token", "super-secret-token")

			err := handler.AuthorizeRequest(&req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Returns unauthorized error when the token is incorrect", func() {
			req := http.Request{Header: make(http.Header)}
			req.Header.Set("verification-token", "incorrect-token")

			err := handler.AuthorizeRequest(&req)
			Expect(err).To(MatchError(errors.Unauthorized))
		})

		It("Returns unauthorized error when the token is missing", func() {
			req := http.Request{}

			err := handler.AuthorizeRequest(&req)
			Expect(err).To(MatchError(errors.Unauthorized))
		})
	})

	Describe("ProcessEHRMessage", func() {
		It("returns an error when metadata is invalid (missing)", func() {
			ctx := context.Background()
			payload := []byte(`{}`)

			err := handler.ProcessEHRMessage(ctx, payload)
			Expect(err).To(MatchError(errors.BadRequest))

			count, err := collection.CountDocuments(ctx, bson.M{})
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(BeEquivalentTo(0))
		})

		It("inserts the message if the data is valid", func() {
			ctx := context.Background()
			payload, err := test.LoadFixture("test/fixtures/enable_reports_order.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(payload).ToNot(HaveLen(0))

			var order models.NewOrder
			err = json.Unmarshal(payload, &order)
			Expect(err).ToNot(HaveOccurred())

			err = handler.ProcessEHRMessage(ctx, payload)
			Expect(err).ToNot(HaveOccurred())

			env := models.MessageEnvelope{}
			err = collection.FindOne(ctx, bson.M{
				"meta.Logs.ID": "d9f5d293-7110-461e-a875-3beb089e79f3",
			}).Decode(&env)

			Expect(err).ToNot(HaveOccurred())
			Expect(env.Meta).To(BeEquivalentTo(order.Meta))
		})
	})

	Describe("FindMatchingClinic", func() {
		var clinic *clinics.Clinic
		var criteria redox.ClinicMatchingCriteria

		BeforeEach(func() {
			clinicId := primitive.NewObjectID()
			clinic = clinicsTest.RandomClinic()
			clinic.Id = &clinicId

			criteria = redox.ClinicMatchingCriteria{
				SourceId:     clinic.EHRSettings.SourceId,
				FacilityName: &clinic.EHRSettings.Facility.Name,
			}
		})

		It("returns an error when the source id is empty", func() {
			criteria.SourceId = ""
			res, err := handler.FindMatchingClinic(context.Background(), criteria)
			Expect(err).To(MatchError(errors.BadRequest))
			Expect(res).To(BeNil())
		})

		It("returns the matching clinic when only one clinic matches", func() {
			ehrEnabled := true
			clinicsService.EXPECT().List(gomock.Any(), gomock.Eq(&clinics.Filter{
				EHRProvider:     &clinics.EHRProviderRedox,
				EHREnabled:      &ehrEnabled,
				EHRSourceId:     &criteria.SourceId,
				EHRFacilityName: criteria.FacilityName,
			}), gomock.Any()).Return([]*clinics.Clinic{clinic}, nil)

			res, err := handler.FindMatchingClinic(context.Background(), criteria)
			Expect(err).To(BeNil())
			Expect(res).ToNot(BeNil())
		})

		It("returns an error when multiple clinics match the criteria", func() {
			ehrEnabled := true
			clinicsService.EXPECT().List(gomock.Any(), gomock.Eq(&clinics.Filter{
				EHRProvider:     &clinics.EHRProviderRedox,
				EHREnabled:      &ehrEnabled,
				EHRSourceId:     &criteria.SourceId,
				EHRFacilityName: criteria.FacilityName,
			}), gomock.Any()).Return([]*clinics.Clinic{clinic, clinicsTest.RandomClinic()}, nil)

			res, err := handler.FindMatchingClinic(context.Background(), criteria)
			Expect(err).To(MatchError(errors.Duplicate))
			Expect(res).To(BeNil())
		})

		It("returns an error when no clinics match the criteria", func() {
			ehrEnabled := true
			clinicsService.EXPECT().List(gomock.Any(), gomock.Eq(&clinics.Filter{
				EHRProvider:     &clinics.EHRProviderRedox,
				EHREnabled:      &ehrEnabled,
				EHRSourceId:     &criteria.SourceId,
				EHRFacilityName: criteria.FacilityName,
			}), gomock.Any()).Return([]*clinics.Clinic{}, nil)

			res, err := handler.FindMatchingClinic(context.Background(), criteria)
			Expect(err).To(MatchError(errors.NotFound))
			Expect(res).To(BeNil())
		})
	})

	Describe("MatchNewOrderToPatient", func() {
		var clinic clinics.Clinic
		var patient patients.Patient
		var order models.NewOrder
		var matchOrder redox.MatchOrder
		var documentId primitive.ObjectID

		Context("with a subscription order", func() {
			BeforeEach(func() {
				clinic = *clinicsTest.RandomClinic()
				patient = patientsTest.RandomPatient()

				payload, err := test.LoadFixture("test/fixtures/enable_reports_order.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(payload).ToNot(HaveLen(0))

				err = json.Unmarshal(payload, &order)
				Expect(err).ToNot(HaveOccurred())

				documentId = primitive.NewObjectID()
				matchOrder = redox.MatchOrder{
					DocumentId:        documentId,
					Order:             order,
					PatientAttributes: []string{redox.MRNAndDOBPatientMatchingCriteria},
				}

				clinicsService.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*clinics.Clinic{&clinic}, nil)

				matchOrder.SubscriptionUpdate = &patients.SubscriptionUpdate{
					Name:     "summaryAndReports",
					Provider: clinics.EHRProviderRedox,
					Active:   true,
					MatchedMessage: patients.MatchedMessage{
						DocumentId: documentId,
						DataModel:  "Order",
						EventType:  "New",
					},
				}
			})

			It("does not return an error when mrn cannot be found", func() {
				matchOrder.Order.Patient.Identifiers = nil
				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)

				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.Patients).To(BeEmpty())
			})

			It("returns an error when demographics is empty", func() {
				matchOrder.Order.Patient.Demographics = nil
				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(MatchError(errors.BadRequest))
				Expect(res).To(BeNil())
			})

			It("returns an error when date of birth is empty", func() {
				matchOrder.Order.Patient.Demographics.DOB = nil
				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(MatchError(errors.BadRequest))
				Expect(res).To(BeNil())
			})

			It("successfully matches a patient", func() {
				fixtureMrn := "0000000001"
				fixtureDateOfBirth := "2008-01-06"
				clinicId := clinic.Id.Hex()
				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId:  &clinicId,
					Mrn:       &fixtureMrn,
					BirthDate: &fixtureDateOfBirth,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{&patient},
					TotalCount: 1,
				}, nil)

				patientsService.EXPECT().UpdateEHRSubscription(
					gomock.Any(),
					gomock.Eq(clinicId),
					gomock.Eq(*patient.UserId),
					gomock.Eq(*matchOrder.SubscriptionUpdate),
				).Return(nil)

				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
				Expect(res.Patients).To(HaveLen(1))
			})

			It("returns all patients when multiple matches are found", func() {
				fixtureMrn := "0000000001"
				fixtureDateOfBirth := "2008-01-06"
				second := patientsTest.RandomPatient()
				clinicId := clinic.Id.Hex()

				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId:  &clinicId,
					Mrn:       &fixtureMrn,
					BirthDate: &fixtureDateOfBirth,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{&patient, &second},
					TotalCount: 2,
				}, nil)

				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
				Expect(res.Patients).To(HaveLen(2))
			})

			It("does not return error when no patients are found", func() {
				fixtureMrn := "0000000001"
				fixtureDateOfBirth := "2008-01-06"
				clinicId := clinic.Id.Hex()

				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId:  &clinicId,
					Mrn:       &fixtureMrn,
					BirthDate: &fixtureDateOfBirth,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{},
					TotalCount: 0,
				}, nil)

				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
				Expect(res.Patients).To(HaveLen(0))
			})
		})

		Context("with an account creation order", func() {
			BeforeEach(func() {
				clinic = *clinicsTest.RandomClinic()
				patient = patientsTest.RandomPatient()

				payload, err := test.LoadFixture("test/fixtures/create_account_order.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(payload).ToNot(HaveLen(0))

				err = json.Unmarshal(payload, &order)
				Expect(err).ToNot(HaveOccurred())

				clinicsService.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*clinics.Clinic{&clinic}, nil)

				matchOrder.Order = order
				matchOrder.PatientAttributes = []string{redox.MRNPatientMatchingCriteria, redox.DOBAndFullNamePatientMatchingCriteria}
				matchOrder.SubscriptionUpdate = nil
			})

			It("returns unique patients when multiple matches are found", func() {
				fixtureMrn := "0000000001"
				fixtureDateOfBirth := "2008-01-06"
				fixtureFullName := "Timothy Bixby"
				clinicId := clinic.Id.Hex()

				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId: &clinicId,
					Mrn:      &fixtureMrn,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{&patient, &patient},
					TotalCount: 2,
				}, nil)

				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId:  &clinicId,
					BirthDate: &fixtureDateOfBirth,
					FullName:  &fixtureFullName,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{&patient},
					TotalCount: 1,
				}, nil)

				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
				Expect(res.Patients).To(HaveLen(1))
			})

		})

		Context("with create account and enable reports order", func() {
			BeforeEach(func() {
				clinic = *clinicsTest.RandomClinic()
				patient = patientsTest.RandomPatient()

				payload, err := test.LoadFixture("test/fixtures/create_account_enable_reports_order.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(payload).ToNot(HaveLen(0))

				err = json.Unmarshal(payload, &order)
				Expect(err).ToNot(HaveOccurred())

				clinicsService.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*clinics.Clinic{&clinic}, nil)

				matchOrder.Order = order
				matchOrder.PatientAttributes = []string{redox.MRNPatientMatchingCriteria, redox.DOBAndFullNamePatientMatchingCriteria}
				matchOrder.SubscriptionUpdate = nil
			})

			It("returns unique patients when multiple matches are found", func() {
				fixtureMrn := "0000000001"
				fixtureDateOfBirth := "2008-01-06"
				fixtureFullName := "Timothy Bixby"
				clinicId := clinic.Id.Hex()

				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId: &clinicId,
					Mrn:      &fixtureMrn,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{&patient, &patient},
					TotalCount: 2,
				}, nil)

				patientsService.EXPECT().List(gomock.Any(), gomock.Eq(&patients.Filter{
					ClinicId:  &clinicId,
					BirthDate: &fixtureDateOfBirth,
					FullName:  &fixtureFullName,
				}), gomock.Any(), gomock.Any()).Return(&patients.ListResult{
					Patients:   []*patients.Patient{&patient},
					TotalCount: 1,
				}, nil)

				res, err := handler.MatchNewOrderToPatient(context.Background(), matchOrder)
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
				Expect(res.Patients).To(HaveLen(1))
			})

		})

	})
})
