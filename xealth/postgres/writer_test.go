package postgres_test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/xealth"
	xealthPostgres "github.com/tidepool-org/clinic/xealth/postgres"
)

var _ = Describe("Xealth Writer", func() {
	var writer *xealthPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	count := func(query string, args ...interface{}) int {
		GinkgoHelper()
		var result int
		Expect(conn.QueryRow(ctx, query, args...).Scan(&result)).To(Succeed())
		return result
	}

	BeforeEach(func() {
		ctx = context.Background()
		config := *dbTest.GetTestPostgresConfig()
		config.Enabled = true
		config.RunMigrations = false

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		lifecycle.RequireStart()
		DeferCleanup(lifecycle.RequireStop)

		writer = xealthPostgres.NewWriter(client, zap.NewNop().Sugar())

		conn, err = pgx.Connect(ctx, config.ConnectionString())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(conn.Close(context.Background())).To(Succeed())
		})
	})

	It("upserts preorders idempotently", func() {
		id := primitive.NewObjectID()
		data := &xealth.PreorderFormData{
			Id:             &id,
			DataTrackingId: primitive.NewObjectID().Hex(),
		}
		Expect(writer.UpsertPreorder(ctx, data)).To(Succeed())
		Expect(writer.UpsertPreorder(ctx, data)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM xealth_preorders WHERE id = $1", id.Hex())).To(Equal(1))

		var trackingId string
		Expect(conn.QueryRow(ctx, "SELECT data_tracking_id FROM xealth_preorders WHERE id = $1", id.Hex()).Scan(&trackingId)).To(Succeed())
		Expect(trackingId).To(Equal(data.DataTrackingId))
	})

	It("upserts orders idempotently", func() {
		id := primitive.NewObjectID()
		order := &xealth.OrderEvent{Id: &id}
		Expect(writer.UpsertOrder(ctx, order)).To(Succeed())
		Expect(writer.UpsertOrder(ctx, order)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM xealth_orders WHERE id = $1", id.Hex())).To(Equal(1))
	})

	It("upserts report views idempotently with all columns", func() {
		id := primitive.NewObjectID()
		systemLogin := "system-login"
		view := &xealth.ReportView{
			Id:            &id,
			UserId:        "user-1",
			DeploymentId:  "deployment-1",
			SystemLogin:   &systemLogin,
			PatientUserId: "patient-1",
			ProgramId:     "program-1",
			ClinicId:      primitive.NewObjectID(),
			CreatedTime:   time.Now().UTC().Truncate(time.Millisecond),
		}
		Expect(writer.UpsertReportView(ctx, view)).To(Succeed())
		Expect(writer.UpsertReportView(ctx, view)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM xealth_report_views WHERE id = $1", id.Hex())).To(Equal(1))

		var userId, deploymentId, patientUserId, programId, clinicId string
		var login *string
		var createdTime time.Time
		row := conn.QueryRow(ctx,
			"SELECT user_id, deployment_id, system_login, patient_user_id, program_id, clinic_id, created_time FROM xealth_report_views WHERE id = $1",
			id.Hex())
		Expect(row.Scan(&userId, &deploymentId, &login, &patientUserId, &programId, &clinicId, &createdTime)).To(Succeed())
		Expect(userId).To(Equal(view.UserId))
		Expect(deploymentId).To(Equal(view.DeploymentId))
		Expect(login).To(HaveValue(Equal(systemLogin)))
		Expect(patientUserId).To(Equal(view.PatientUserId))
		Expect(programId).To(Equal(view.ProgramId))
		Expect(clinicId).To(Equal(view.ClinicId.Hex()))
		Expect(createdTime.UTC()).To(BeTemporally("==", view.CreatedTime))
	})

	It("rejects documents without ids", func() {
		Expect(writer.UpsertPreorder(ctx, &xealth.PreorderFormData{})).ToNot(Succeed())
		Expect(writer.UpsertOrder(ctx, &xealth.OrderEvent{})).ToNot(Succeed())
		Expect(writer.UpsertReportView(ctx, &xealth.ReportView{})).ToNot(Succeed())
	})
})

var _ = Describe("Xealth Dual Store", func() {
	var store xealth.Store
	var conn *pgx.Conn
	var ctx context.Context

	count := func(query string, args ...interface{}) int {
		GinkgoHelper()
		var result int
		Expect(conn.QueryRow(ctx, query, args...).Scan(&result)).To(Succeed())
		return result
	}

	BeforeEach(func() {
		ctx = context.Background()
		config := *dbTest.GetTestPostgresConfig()
		config.Enabled = true
		config.RunMigrations = false

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())

		mongoStore, err := xealth.NewStore(dbTest.GetTestDatabase(), zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())

		lifecycle.RequireStart()
		DeferCleanup(lifecycle.RequireStop)

		writer := xealthPostgres.NewWriter(client, zap.NewNop().Sugar())
		store = xealthPostgres.NewDualStore(mongoStore, writer, zap.NewNop().Sugar())

		conn, err = pgx.Connect(ctx, config.ConnectionString())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(conn.Close(context.Background())).To(Succeed())
		})
	})

	It("mirrors preorder data with the id generated by Mongo", func() {
		data := xealth.PreorderFormData{
			DataTrackingId: primitive.NewObjectID().Hex(),
		}
		Expect(store.CreatePreorderData(ctx, data)).To(Succeed())

		created, err := store.GetPreorderData(ctx, data.DataTrackingId)
		Expect(err).ToNot(HaveOccurred())
		Expect(created.Id).ToNot(BeNil())
		Expect(count("SELECT COUNT(*) FROM xealth_preorders WHERE id = $1 AND data_tracking_id = $2",
			created.Id.Hex(), data.DataTrackingId)).To(Equal(1))
	})

	It("mirrors orders", func() {
		created, err := store.CreateOrder(ctx, xealth.OrderEvent{})
		Expect(err).ToNot(HaveOccurred())
		Expect(created.Id).ToNot(BeNil())
		Expect(count("SELECT COUNT(*) FROM xealth_orders WHERE id = $1", created.Id.Hex())).To(Equal(1))
	})

	It("mirrors report views", func() {
		created, err := store.CreateReportView(ctx, xealth.ReportView{
			UserId:        "user-2",
			DeploymentId:  "deployment-2",
			PatientUserId: "patient-2",
			ProgramId:     "program-2",
			ClinicId:      primitive.NewObjectID(),
			CreatedTime:   time.Now().UTC(),
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(created.Id).ToNot(BeNil())
		Expect(count("SELECT COUNT(*) FROM xealth_report_views WHERE id = $1", created.Id.Hex())).To(Equal(1))
	})
})

var _ = Describe("Xealth Backfillers", func() {
	var writer *xealthPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		config := *dbTest.GetTestPostgresConfig()
		config.Enabled = true
		config.RunMigrations = false

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		lifecycle.RequireStart()
		DeferCleanup(lifecycle.RequireStop)

		writer = xealthPostgres.NewWriter(client, zap.NewNop().Sugar())

		conn, err = pgx.Connect(ctx, config.ConnectionString())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(conn.Close(context.Background())).To(Succeed())
		})
	})

	It("copies orders in resumable batches", func() {
		db := dbTest.GetTestDatabase()
		collection := db.Collection("xealth_order")

		// Other specs in this suite create orders in the shared database;
		// batch counts below require exclusive ownership of the collection.
		_, err := collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())

		ids := make([]primitive.ObjectID, 0, 3)
		for i := 0; i < 3; i++ {
			id := primitive.NewObjectID()
			_, err := collection.InsertOne(ctx, xealth.OrderEvent{Id: &id})
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, id)
		}

		backfiller := xealthPostgres.NewOrderBackfiller(db, writer)
		Expect(backfiller.Collection()).To(Equal("xealth_order"))

		last, n, err := backfiller.BackfillBatch(ctx, primitive.NilObjectID, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(2))

		last, n, err = backfiller.BackfillBatch(ctx, last, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))

		_, n, err = backfiller.BackfillBatch(ctx, last, 2)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(0))

		for _, id := range ids {
			var result int
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM xealth_orders WHERE id = $1", id.Hex()).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(1))
		}
	})
})
