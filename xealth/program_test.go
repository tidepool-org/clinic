package xealth_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/test"
	"github.com/tidepool-org/clinic/xealth"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

var _ = Describe("Program", func() {
	Describe("Title", func() {
		It("is set to Tidepool", func() {
			Expect(xealth.GetProgramTitle()).To(PointTo(Equal("Tidepool")))
		})
	})

	Describe("Program Id", func() {
		var orderEvent xealth.OrderEvent

		BeforeEach(func() {
			objId := primitive.NewObjectID()
			orderEvent.Id = &objId

			body, err := test.LoadFixture("test/fixtures/order.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &orderEvent.OrderData)).To(Succeed())

			body, err = test.LoadFixture("test/fixtures/order_event_notification.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &orderEvent.EventNotification)).To(Succeed())
		})

		It("is correct", func() {
			Expect(xealth.GetProgramIdFromOrder(&orderEvent)).To(PointTo(Equal("awesome_program_601")))
		})
	})

	Describe("Program Enrollment Date", func() {
		var orderEvent xealth.OrderEvent

		BeforeEach(func() {
			objId := primitive.NewObjectID()
			orderEvent.Id = &objId

			body, err := test.LoadFixture("test/fixtures/order.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &orderEvent.OrderData)).To(Succeed())

			body, err = test.LoadFixture("test/fixtures/order_event_notification.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &orderEvent.EventNotification)).To(Succeed())
		})

		It("is correct", func() {
			Expect(xealth.GetProgramEnrollmentDateFromOrder(&orderEvent)).To(PointTo(Equal("2021-01-14")))
		})
	})

	Describe("Last Upload Date", func() {
		var patient patients.Patient

		BeforeEach(func() {
			patient = patientsTest.RandomPatient()
		})

		It("is zero when patient is nil", func() {
			Expect(xealth.GetLastUploadDate(nil)).To(BeZero())
		})

		It("is zero when patient summary is nil", func() {
			patient.Summary = nil
			Expect(xealth.GetLastUploadDate(nil)).To(BeZero())
		})

		It("is uses cgm upload date if it is more recent the bgm upload date", func() {
			older := time.Date(2019, 02, 28, 0, 0, 0, 0, time.Local)
			newer := time.Date(2020, 02, 28, 0, 0, 0, 0, time.Local)

			patient.Summary = &patients.Summary{
				CGM: &patients.PatientCGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &newer,
					},
				},
				BGM: &patients.PatientBGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &older,
					},
				},
			}

			Expect(xealth.GetLastUploadDate(&patient)).To(Equal(newer))
		})

		It("is uses bgm upload date if it is more recent the cgm upload date", func() {
			older := time.Date(2019, 02, 28, 0, 0, 0, 0, time.Local)
			newer := time.Date(2020, 02, 28, 0, 0, 0, 0, time.Local)

			patient.Summary = &patients.Summary{
				CGM: &patients.PatientCGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &older,
					},
				},
				BGM: &patients.PatientBGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &newer,
					},
				},
			}

			Expect(xealth.GetLastUploadDate(&patient)).To(Equal(newer))
		})
	})

	Describe("HasStatusView", func() {
		var patient patients.Patient

		BeforeEach(func() {
			patient = patientsTest.RandomPatient()
		})

		It("is false when patient hasn't uploaded", func() {
			patient.Summary = nil
			Expect(xealth.HasStatusView(&patient, nil)).To(PointTo(BeFalse()))
		})

		It("is false if the patient has uploaded data but doesn't have a subscription", func() {
			lastUpload := time.Date(2019, 02, 28, 0, 0, 0, 0, time.Local)

			patient.Summary = &patients.Summary{
				CGM: &patients.PatientCGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &lastUpload,
					},
				},
			}

			Expect(xealth.HasStatusView(&patient, nil)).To(PointTo(BeFalse()))
		})

		It("is false if the patient has uploaded data but doesn't have an active subscription", func() {
			lastUpload := time.Date(2019, 02, 28, 0, 0, 0, 0, time.Local)

			patient.Summary = &patients.Summary{
				CGM: &patients.PatientCGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &lastUpload,
					},
				},
			}

			subscriptions := patientsTest.RandomSubscriptions()
			subscription := subscriptions[patients.SubscriptionXealthReports]
			subscription.Active = false

			Expect(xealth.HasStatusView(&patient, &subscription)).To(PointTo(BeFalse()))
		})

		It("is trie if the patient has uploaded data and has an active subscription", func() {
			lastUpload := time.Date(2019, 02, 28, 0, 0, 0, 0, time.Local)

			patient.Summary = &patients.Summary{
				CGM: &patients.PatientCGMStats{
					Dates: patients.PatientSummaryDates{
						LastUploadDate: &lastUpload,
					},
				},
			}

			subscriptions := patientsTest.RandomSubscriptions()
			subscription := subscriptions[patients.SubscriptionXealthReports]
			subscription.Active = true

			Expect(xealth.HasStatusView(&patient, &subscription)).To(PointTo(BeTrue()))
		})
	})

	Describe("Description", func() {
		var lastUpload time.Time
		var lastViewed time.Time

		BeforeEach(func() {
			lastUpload = time.Time{}
			lastViewed = time.Time{}
		})

		It("is correct when last upload and last viewed are not set", func() {
			expected := "Last Upload: N/A | Last Viewed by You: N/A"
			Expect(xealth.GetProgramDescription(lastUpload, lastViewed)).To(PointTo(Equal(expected)))
		})

		It("is correct when last upload is set", func() {
			lastUpload = time.Date(2020, 02, 28, 0, 0, 0, 0, time.Local)
			expected := "Last Upload: 2020-02-28 | Last Viewed by You: N/A"
			Expect(xealth.GetProgramDescription(lastUpload, lastViewed)).To(PointTo(Equal(expected)))
		})

		It("is correct when last viewed is set", func() {
			lastViewed = time.Date(2019, 01, 15, 0, 0, 0, 0, time.Local)
			expected := "Last Upload: N/A | Last Viewed by You: 2019-01-15"
			Expect(xealth.GetProgramDescription(lastUpload, lastViewed)).To(PointTo(Equal(expected)))
		})

		It("is correct when last viewed and last upload are set", func() {
			lastUpload = time.Date(2020, 02, 28, 0, 0, 0, 0, time.Local)
			lastViewed = time.Date(2019, 01, 15, 0, 0, 0, 0, time.Local)
			expected := "Last Upload: 2020-02-28 | Last Viewed by You: 2019-01-15"
			Expect(xealth.GetProgramDescription(lastUpload, lastViewed)).To(PointTo(Equal(expected)))
		})
	})

	Describe("IsProgramAlertActive", func() {
		var lastUpload time.Time
		var lastViewed time.Time

		BeforeEach(func() {
			lastUpload = time.Time{}
			lastViewed = time.Time{}
		})

		It("is false when last upload and last viewed are not set", func() {
			Expect(xealth.IsProgramAlertActive(lastUpload, lastViewed)).To(PointTo(Equal(false)))
		})

		It("is true when last viewed is before last upload", func() {
			lastUpload = time.Now()
			Expect(xealth.IsProgramAlertActive(lastUpload, lastViewed)).To(PointTo(Equal(true)))
		})
	})

})
