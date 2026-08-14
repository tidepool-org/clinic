package xealth_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	errs "github.com/tidepool-org/clinic/errors"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/xealth"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

var _ = Describe("Store", func() {
	var collection *mongo.Collection
	var store xealth.Store

	BeforeEach(func() {
		database := dbTest.GetTestDatabase()
		collection = database.Collection("xealth_report_view")
		lifecycle := fxtest.NewLifecycle(GinkgoT())

		var err error
		store, err = xealth.NewStore(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(store).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	AfterEach(func() {
		_, err := collection.DeleteMany(context.Background(), bson.M{})
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("GetMostRecentReportView", func() {
		const (
			deploymentId       = "test-deployment"
			programId          = "test-program"
			patientUserId      = "patient-user-id"
			otherPatientUserId = "other-patient-user-id"
		)

		var clinicId primitive.ObjectID
		var now time.Time

		createView := func(userId, patientId string, createdTime time.Time) {
			_, err := store.CreateReportView(context.Background(), xealth.ReportView{
				UserId:        userId,
				DeploymentId:  deploymentId,
				PatientUserId: patientId,
				ProgramId:     programId,
				ClinicId:      clinicId,
				CreatedTime:   createdTime,
			})
			Expect(err).ToNot(HaveOccurred())
		}

		BeforeEach(func() {
			clinicId = primitive.NewObjectID()
			now = time.Now().UTC().Truncate(time.Millisecond)

			createView("user-a", patientUserId, now.Add(-3*time.Hour))
			createView("user-a", patientUserId, now.Add(-2*time.Hour))
			createView("user-b", patientUserId, now.Add(-1*time.Hour))
			createView("user-c", otherPatientUserId, now)
		})

		It("returns the most recent view of the given user", func() {
			view, err := store.GetMostRecentReportView(context.Background(), xealth.ReportViewFilter{
				ClinicId:      clinicId,
				DeploymentId:  deploymentId,
				PatientUserId: patientUserId,
				ProgramId:     programId,
				UserId:        "user-a",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(view).ToNot(BeNil())
			Expect(view.UserId).To(Equal("user-a"))
			Expect(view.CreatedTime).To(BeTemporally("==", now.Add(-2*time.Hour)))
		})

		It("returns the most recent view of the patient by any user when the user id is empty", func() {
			view, err := store.GetMostRecentReportView(context.Background(), xealth.ReportViewFilter{
				ClinicId:      clinicId,
				DeploymentId:  deploymentId,
				PatientUserId: patientUserId,
				ProgramId:     programId,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(view).ToNot(BeNil())
			Expect(view.UserId).To(Equal("user-b"))
			Expect(view.CreatedTime).To(BeTemporally("==", now.Add(-1*time.Hour)))
		})

		It("returns not found when the user has no views", func() {
			_, err := store.GetMostRecentReportView(context.Background(), xealth.ReportViewFilter{
				ClinicId:      clinicId,
				DeploymentId:  deploymentId,
				PatientUserId: patientUserId,
				ProgramId:     programId,
				UserId:        "user-d",
			})
			Expect(err).To(MatchError(errs.NotFound))
		})

		It("round-trips the last updated dates of the stats", func() {
			cgmLastUpdated := now.Add(-30 * time.Minute)
			bgmLastUpdated := now.Add(-45 * time.Minute)
			_, err := store.CreateReportView(context.Background(), xealth.ReportView{
				UserId:        "user-d",
				DeploymentId:  deploymentId,
				PatientUserId: patientUserId,
				ProgramId:     programId,
				ClinicId:      clinicId,
				CreatedTime:   now,
				ReportViewStats: xealth.ReportViewStats{
					CgmLastUpdated: &cgmLastUpdated,
					BgmLastUpdated: &bgmLastUpdated,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			view, err := store.GetMostRecentReportView(context.Background(), xealth.ReportViewFilter{
				ClinicId:      clinicId,
				DeploymentId:  deploymentId,
				PatientUserId: patientUserId,
				ProgramId:     programId,
				UserId:        "user-d",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(view).ToNot(BeNil())
			Expect(view.CgmLastUpdated).To(PointTo(BeTemporally("==", cgmLastUpdated)))
			Expect(view.BgmLastUpdated).To(PointTo(BeTemporally("==", bgmLastUpdated)))

			// The stats dates are inlined in the document, not nested
			raw := bson.M{}
			Expect(collection.FindOne(context.Background(), bson.M{"userId": "user-d"}).Decode(&raw)).To(Succeed())
			Expect(raw).To(HaveKey("cgmLastUpdated"))
			Expect(raw).To(HaveKey("bgmLastUpdated"))
		})

		It("stores views without stats last updated dates", func() {
			view, err := store.GetMostRecentReportView(context.Background(), xealth.ReportViewFilter{
				ClinicId:      clinicId,
				DeploymentId:  deploymentId,
				PatientUserId: patientUserId,
				ProgramId:     programId,
				UserId:        "user-a",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(view).ToNot(BeNil())
			Expect(view.CgmLastUpdated).To(BeNil())
			Expect(view.BgmLastUpdated).To(BeNil())
		})
	})
})
