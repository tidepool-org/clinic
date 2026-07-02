package postgres_test

import (
	"context"
	"fmt"
	"net"

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

// Exercises the three verification tiers over the preorders pair: counts,
// the sorted identity diff, and repair through the idempotent upserts.
var _ = Describe("Verify", func() {
	var ctx context.Context
	var writer *xealthPostgres.Writer
	var verifier storepg.Verifier
	var client *storepg.Client
	var conn *pgx.Conn

	var ids []primitive.ObjectID
	var missingId, phantomId, driftedId string
	var driftedTrackingId string

	BeforeEach(func() {
		ctx = context.Background()
		config := *dbTest.GetTestPostgresConfig()
		config.Enabled = true
		config.RunMigrations = false

		lifecycle := fxtest.NewLifecycle(GinkgoT())
		var err error
		client, err = storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		lifecycle.RequireStart()
		DeferCleanup(lifecycle.RequireStop)

		conn, err = pgx.Connect(ctx, config.ConnectionString())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(conn.Close(context.Background())).To(Succeed())
		})

		writer = xealthPostgres.NewWriter(client, zap.NewNop().Sugar())
		db := dbTest.GetTestDatabase()
		verifier = xealthPostgres.NewPreorderBackfiller(db, writer)

		// The verification tiers compare complete identity sets, so this
		// spec owns both stores' preorders exclusively.
		collection := db.Collection("xealth_preorder")
		_, err = collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())
		_, err = conn.Exec(ctx, "DELETE FROM xealth_preorders")
		Expect(err).ToNot(HaveOccurred())

		// Three mirrored documents...
		ids = ids[:0]
		for i := 0; i < 3; i++ {
			id := primitive.NewObjectID()
			data := &xealth.PreorderFormData{Id: &id, DataTrackingId: primitive.NewObjectID().Hex()}
			_, err := collection.InsertOne(ctx, data)
			Expect(err).ToNot(HaveOccurred())
			Expect(writer.UpsertPreorder(ctx, data)).To(Succeed())
			ids = append(ids, id)
		}

		// ...one of which is missing in Postgres...
		missingId = ids[0].Hex()
		_, err = conn.Exec(ctx, "DELETE FROM xealth_preorders WHERE id = $1", missingId)
		Expect(err).ToNot(HaveOccurred())

		// ...one Postgres row with no Mongo document...
		phantomId = primitive.NewObjectID().Hex()
		_, err = conn.Exec(ctx,
			"INSERT INTO xealth_preorders (id, data_tracking_id, payload) VALUES ($1, $2, '{}')",
			phantomId, primitive.NewObjectID().Hex())
		Expect(err).ToNot(HaveOccurred())

		// ...and one matched row whose content drifted.
		driftedId = ids[1].Hex()
		var record struct {
			DataTrackingId string `bson:"dataTrackingId"`
		}
		Expect(collection.FindOne(ctx, bson.M{"_id": ids[1]}).Decode(&record)).To(Succeed())
		driftedTrackingId = record.DataTrackingId
		_, err = conn.Exec(ctx, "UPDATE xealth_preorders SET data_tracking_id = $1 WHERE id = $2",
			"drifted-"+primitive.NewObjectID().Hex(), driftedId)
		Expect(err).ToNot(HaveOccurred())
	})

	It("reports counts, missing ids and phantom ids without repairing", func() {
		report, err := storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(report.Collection).To(Equal("xealth_preorder"))
		Expect(report.MongoCount).To(Equal(int64(3)))
		Expect(report.PGCount).To(Equal(int64(3))) // 2 mirrored + 1 phantom
		Expect(report.Missing).To(ConsistOf(missingId))
		Expect(report.Phantom).To(ConsistOf(phantomId))
		Expect(report.Matched).To(Equal(int64(2)))
		Expect(report.HasDrift()).To(BeTrue())
		Expect(report.Repaired).To(BeFalse())

		// Without repair nothing changes
		var count int
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM xealth_preorders WHERE id = $1", missingId).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(0))
	})

	It("converges both stores with repair and sampling", func() {
		report, err := storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{
			Repair: true,
			Sample: 10,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Repaired).To(BeTrue())
		// The missing document plus both sampled matched documents
		Expect(report.Resynced).To(Equal(3))
		Expect(report.Deleted).To(Equal(1))

		var count int
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM xealth_preorders WHERE id = $1", missingId).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(1))
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM xealth_preorders WHERE id = $1", phantomId).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(0))

		// The sampled re-upsert converged the drifted row
		var trackingId string
		Expect(conn.QueryRow(ctx, "SELECT data_tracking_id FROM xealth_preorders WHERE id = $1", driftedId).Scan(&trackingId)).To(Succeed())
		Expect(trackingId).To(Equal(driftedTrackingId))

		// A second verification is clean
		report, err = storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(report.HasDrift()).To(BeFalse())
		Expect(report.Matched).To(Equal(int64(3)))
	})

	It("streams identities in small batches", func() {
		report, err := storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{
			BatchSize: 1,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Missing).To(ConsistOf(missingId))
		Expect(report.Phantom).To(ConsistOf(phantomId))
		Expect(report.Matched).To(Equal(int64(2)))
	})
})

// Proves the dual-write guarantee under a Postgres outage: Mongo writes
// succeed and the API of the store is unaffected when Postgres is
// unreachable.
var _ = Describe("Dual writes during a Postgres outage", func() {
	It("never fails the Mongo write", func() {
		ctx := context.Background()

		// A port that just refused a listener is almost certainly closed
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).ToNot(HaveOccurred())
		port := listener.Addr().(*net.TCPAddr).Port
		Expect(listener.Close()).To(Succeed())

		config := *dbTest.GetTestPostgresConfig()
		config.Enabled = true
		config.RunMigrations = false
		config.Host = "127.0.0.1"
		config.Port = port
		config.ConnectTimeoutSeconds = 1

		// The client is constructed without starting the lifecycle: the
		// OnStart ping would fail, which is the deployment-time signal, but
		// an outage at runtime must not fail requests.
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		client, err := storepg.NewClient(&config, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			client.Pool().Close()
		})

		writer := xealthPostgres.NewWriter(client, zap.NewNop().Sugar())
		mongoStore, err := xealth.NewStore(dbTest.GetTestDatabase(), zap.NewNop().Sugar(), fxtest.NewLifecycle(GinkgoT()))
		Expect(err).ToNot(HaveOccurred())
		store := xealthPostgres.NewDualStore(mongoStore, writer, zap.NewNop().Sugar())

		created, err := store.CreateOrder(ctx, xealth.OrderEvent{})
		Expect(err).ToNot(HaveOccurred(), "a postgres outage must never fail the write")
		Expect(created).ToNot(BeNil())
		Expect(created.Id).ToNot(BeNil())

		// The write reached Mongo
		count, err := dbTest.GetTestDatabase().Collection("xealth_order").
			CountDocuments(ctx, bson.M{"_id": *created.Id})
		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(int64(1)))
	})
})

var _ = Describe("Config", func() {
	It("includes the connect timeout in the connection string", func() {
		config := storepg.Config{Host: "localhost", Port: 5432, User: "postgres", DatabaseName: "clinic", SslMode: "disable", ConnectTimeoutSeconds: 7}
		Expect(config.ConnectionString()).To(ContainSubstring(fmt.Sprintf("connect_timeout=%d", 7)))
	})
})
