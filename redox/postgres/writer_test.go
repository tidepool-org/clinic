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

	"github.com/tidepool-org/clinic/redox"
	redoxPostgres "github.com/tidepool-org/clinic/redox/postgres"
	models "github.com/tidepool-org/clinic/redox_models"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

func newWriter() (*redoxPostgres.Writer, *pgx.Conn, context.Context) {
	GinkgoHelper()
	ctx := context.Background()
	config := *dbTest.GetTestPostgresConfig()
	config.Enabled = true
	config.RunMigrations = false

	lifecycle := fxtest.NewLifecycle(GinkgoT())
	client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
	Expect(err).ToNot(HaveOccurred())
	lifecycle.RequireStart()
	DeferCleanup(lifecycle.RequireStop)

	conn, err := pgx.Connect(ctx, config.ConnectionString())
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		Expect(conn.Close(context.Background())).To(Succeed())
	})

	return redoxPostgres.NewWriter(client, zap.NewNop().Sugar()), conn, ctx
}

func newEnvelope() models.MessageEnvelope {
	sourceId := "source-" + primitive.NewObjectID().Hex()
	sourceName := "Source Name"
	facilityCode := "facility-1"
	logId := "log-" + primitive.NewObjectID().Hex()
	raw, err := bson.Marshal(bson.M{"Meta": bson.M{"DataModel": "Order", "EventType": "New"}})
	Expect(err).ToNot(HaveOccurred())

	return models.MessageEnvelope{
		Id: primitive.NewObjectID(),
		Meta: models.Meta{
			DataModel: "Order",
			EventType: "New",
			Source: &struct {
				ID   *string `json:"ID"`
				Name *string `json:"Name"`
			}{ID: &sourceId, Name: &sourceName},
			FacilityCode: &facilityCode,
			Logs: &[]struct {
				AttemptID *string `json:"AttemptID"`
				ID        *string `json:"ID"`
			}{{ID: &logId}},
		},
		Message: raw,
	}
}

var _ = Describe("Redox Writer", func() {
	var writer *redoxPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	count := func(query string, args ...interface{}) int {
		GinkgoHelper()
		var result int
		Expect(conn.QueryRow(ctx, query, args...).Scan(&result)).To(Succeed())
		return result
	}

	BeforeEach(func() {
		writer, conn, ctx = newWriter()
	})

	It("upserts messages idempotently with extracted meta columns", func() {
		envelope := newEnvelope()
		Expect(writer.UpsertMessage(ctx, &envelope)).To(Succeed())
		Expect(writer.UpsertMessage(ctx, &envelope)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM redox_messages WHERE id = $1", envelope.Id.Hex())).To(Equal(1))

		var dataModel, eventType, sourceId, sourceName, facilityCode string
		var logIds []string
		row := conn.QueryRow(ctx,
			"SELECT meta_data_model, meta_event_type, meta_source_id, meta_source_name, meta_facility_code, meta_log_ids FROM redox_messages WHERE id = $1",
			envelope.Id.Hex())
		Expect(row.Scan(&dataModel, &eventType, &sourceId, &sourceName, &facilityCode, &logIds)).To(Succeed())
		Expect(dataModel).To(Equal("Order"))
		Expect(eventType).To(Equal("New"))
		Expect(sourceId).To(Equal(*envelope.Meta.Source.ID))
		Expect(sourceName).To(Equal("Source Name"))
		Expect(facilityCode).To(Equal("facility-1"))
		Expect(logIds).To(ConsistOf(*(*envelope.Meta.Logs)[0].ID))
	})

	It("rejects messages without ids", func() {
		Expect(writer.UpsertMessage(ctx, &models.MessageEnvelope{})).ToNot(Succeed())
	})

	It("upserts scheduled orders idempotently with extracted columns", func() {
		id := primitive.NewObjectID()
		clinicId := primitive.NewObjectID()
		orderId := primitive.NewObjectID()
		createdTime := time.Now().UTC().Truncate(time.Millisecond)
		doc := bson.M{
			"_id":              id,
			"clinicId":         clinicId,
			"userId":           "1234567890",
			"createdTime":      primitive.NewDateTimeFromTime(createdTime),
			"lastMatchedOrder": bson.M{"_id": orderId},
		}
		Expect(writer.UpsertScheduledOrder(ctx, doc)).To(Succeed())
		Expect(writer.UpsertScheduledOrder(ctx, doc)).To(Succeed())
		Expect(count("SELECT COUNT(*) FROM scheduled_summary_reports_orders WHERE id = $1", id.Hex())).To(Equal(1))

		var gotClinicId, gotUserId, gotOrderId string
		var gotCreated time.Time
		row := conn.QueryRow(ctx,
			"SELECT clinic_id, user_id, last_matched_order_id, created_time FROM scheduled_summary_reports_orders WHERE id = $1",
			id.Hex())
		Expect(row.Scan(&gotClinicId, &gotUserId, &gotOrderId, &gotCreated)).To(Succeed())
		Expect(gotClinicId).To(Equal(clinicId.Hex()))
		Expect(gotUserId).To(Equal("1234567890"))
		Expect(gotOrderId).To(Equal(orderId.Hex()))
		Expect(gotCreated.UTC()).To(BeTemporally("==", createdTime))
	})

	It("prunes scheduled orders past the retention period", func() {
		fresh := primitive.NewObjectID()
		expired := primitive.NewObjectID()
		Expect(writer.UpsertScheduledOrder(ctx, bson.M{
			"_id":         fresh,
			"createdTime": primitive.NewDateTimeFromTime(time.Now().UTC().Add(-time.Hour)),
		})).To(Succeed())
		Expect(writer.UpsertScheduledOrder(ctx, bson.M{
			"_id":         expired,
			"createdTime": primitive.NewDateTimeFromTime(time.Now().UTC().Add(-redox.RescheduledMessagesExpiration - time.Hour)),
		})).To(Succeed())

		pruned, err := writer.PruneScheduledOrders(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(pruned).To(BeNumerically(">=", 1))

		Expect(count("SELECT COUNT(*) FROM scheduled_summary_reports_orders WHERE id = $1", fresh.Hex())).To(Equal(1))
		Expect(count("SELECT COUNT(*) FROM scheduled_summary_reports_orders WHERE id = $1", expired.Hex())).To(Equal(0))
	})
})

var _ = Describe("Redox Backfillers", func() {
	var writer *redoxPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	BeforeEach(func() {
		writer, conn, ctx = newWriter()
	})

	It("copies messages in resumable batches", func() {
		db := dbTest.GetTestDatabase()
		collection := db.Collection("redox")

		// Other specs in this suite may create messages in the shared
		// database; batch counts below require exclusive ownership.
		_, err := collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())

		ids := make([]primitive.ObjectID, 0, 3)
		for i := 0; i < 3; i++ {
			envelope := newEnvelope()
			_, err := collection.InsertOne(ctx, envelope)
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, envelope.Id)
		}

		backfiller := redoxPostgres.NewMessageBackfiller(db, writer)
		Expect(backfiller.Collection()).To(Equal("redox"))

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
			Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM redox_messages WHERE id = $1", id.Hex()).Scan(&result)).To(Succeed())
			Expect(result).To(Equal(1))
		}
	})

	It("copies scheduled orders produced by the reschedule pipeline", func() {
		db := dbTest.GetTestDatabase()
		collection := db.Collection("scheduledSummaryAndReportsOrders")

		_, err := collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())

		id := primitive.NewObjectID()
		_, err = collection.InsertOne(ctx, bson.M{
			"_id":              id,
			"clinicId":         primitive.NewObjectID(),
			"userId":           "1234567890",
			"createdTime":      primitive.NewDateTimeFromTime(time.Now().UTC()),
			"lastMatchedOrder": bson.M{"_id": primitive.NewObjectID()},
		})
		Expect(err).ToNot(HaveOccurred())

		backfiller := redoxPostgres.NewScheduledOrderBackfiller(db, writer)
		Expect(backfiller.Collection()).To(Equal("scheduledSummaryAndReportsOrders"))

		_, n, err := backfiller.BackfillBatch(ctx, primitive.NilObjectID, 10)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))

		var result int
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM scheduled_summary_reports_orders WHERE id = $1", id.Hex()).Scan(&result)).To(Succeed())
		Expect(result).To(Equal(1))
	})
})

var _ = Describe("Redox Mirror", func() {
	It("mirrors messages best-effort through the handler seam", func() {
		ctx := context.Background()
		config := *dbTest.GetTestPostgresConfig()
		config.Enabled = true
		config.RunMigrations = false

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		lifecycle.RequireStart()
		DeferCleanup(lifecycle.RequireStop)

		conn, err := pgx.Connect(ctx, config.ConnectionString())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(conn.Close(context.Background())).To(Succeed())
		})

		mirror := redoxPostgres.NewMirror(client, zap.NewNop().Sugar())
		envelope := newEnvelope()
		mirror.CreateMessage(ctx, envelope)

		var result int
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM redox_messages WHERE id = $1", envelope.Id.Hex()).Scan(&result)).To(Succeed())
		Expect(result).To(Equal(1))
	})
})
