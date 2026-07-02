package postgres_test

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics/migration"
	migrationPostgres "github.com/tidepool-org/clinic/clinics/migration/postgres"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

// The migrations table is keyed by user id instead of the Mongo object id;
// this covers the identity override of the verifier.
var _ = Describe("Verify migrations", func() {
	var ctx context.Context
	var client *storepg.Client
	var conn *pgx.Conn
	var verifier storepg.Verifier

	var mirrored, missing *migration.Migration
	var phantomUserId string

	newMigration := func(n int) *migration.Migration {
		m := migration.NewMigration(primitive.NewObjectID().Hex(), fmt.Sprintf("%d%s", n, primitive.NewObjectID().Hex()[:8]))
		return m
	}

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

		writer := migrationPostgres.NewWriter(client, zap.NewNop().Sugar())
		db := dbTest.GetTestDatabase()
		verifier = migrationPostgres.NewBackfiller(db, writer)

		collection := db.Collection("migrations")
		_, err = collection.DeleteMany(ctx, bson.M{})
		Expect(err).ToNot(HaveOccurred())
		_, err = conn.Exec(ctx, "DELETE FROM migrations")
		Expect(err).ToNot(HaveOccurred())

		mirrored = newMigration(1)
		_, err = collection.InsertOne(ctx, mirrored)
		Expect(err).ToNot(HaveOccurred())
		Expect(writer.UpsertMigration(ctx, mirrored)).To(Succeed())

		missing = newMigration(2)
		_, err = collection.InsertOne(ctx, missing)
		Expect(err).ToNot(HaveOccurred())

		phantomUserId = "9" + primitive.NewObjectID().Hex()[:8]
		_, err = conn.Exec(ctx, "INSERT INTO migrations (user_id, clinic_id, status) VALUES ($1, $2, 'PENDING')",
			phantomUserId, primitive.NewObjectID().Hex())
		Expect(err).ToNot(HaveOccurred())
	})

	It("diffs and repairs by user id", func() {
		report, err := storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Missing).To(ConsistOf(missing.UserId))
		Expect(report.Phantom).To(ConsistOf(phantomUserId))
		Expect(report.Matched).To(Equal(int64(1)))
		Expect(report.HasDrift()).To(BeTrue())

		report, err = storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{Repair: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Resynced).To(Equal(1))
		Expect(report.Deleted).To(Equal(1))

		report, err = storepg.VerifyCollection(ctx, client.Pool(), dbTest.GetTestDatabase(), verifier, storepg.VerifyOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(report.HasDrift()).To(BeFalse())
		Expect(report.Matched).To(Equal(int64(2)))
	})
})
