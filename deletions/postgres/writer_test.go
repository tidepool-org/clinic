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

	deletionsPostgres "github.com/tidepool-org/clinic/deletions/postgres"
	"github.com/tidepool-org/clinic/patients"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Deletions Writer", func() {
	var writer *deletionsPostgres.Writer
	var conn *pgx.Conn
	var ctx context.Context

	newDeletionDoc := func(clinicId primitive.ObjectID, userId string) bson.M {
		fullName := "Deleted Patient"
		deletedBy := "9876543210"
		return bson.M{
			"_id":             primitive.NewObjectID(),
			"deletedTime":     time.Now().UTC().Truncate(time.Millisecond),
			"deletedByUserId": &deletedBy,
			"patient": patients.Patient{
				ClinicId: &clinicId,
				UserId:   &userId,
				FullName: &fullName,
			},
		}
	}

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

		writer = deletionsPostgres.NewWriter(client, zap.NewNop().Sugar())

		conn, err = pgx.Connect(ctx, config.ConnectionString())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() { _ = conn.Close(context.Background()) })
	})

	It("upserts deletion documents idempotently", func() {
		clinicId := primitive.NewObjectID()
		doc := newDeletionDoc(clinicId, "1111111111")

		Expect(writer.UpsertDocuments(ctx, "patient", []bson.M{doc})).To(Succeed())
		Expect(writer.UpsertDocuments(ctx, "patient", []bson.M{doc})).To(Succeed())

		id := doc["_id"].(primitive.ObjectID).Hex()
		Expect(count("SELECT count(*) FROM patient_deletions WHERE id = $1", id)).To(Equal(1))

		var clinicColumn, userColumn, deletedBy string
		var payload map[string]interface{}
		Expect(conn.QueryRow(ctx,
			"SELECT clinic_id, user_id, deleted_by_user_id, payload FROM patient_deletions WHERE id = $1", id,
		).Scan(&clinicColumn, &userColumn, &deletedBy, &payload)).To(Succeed())
		Expect(clinicColumn).To(Equal(clinicId.Hex()))
		Expect(userColumn).To(Equal("1111111111"))
		Expect(deletedBy).To(Equal("9876543210"))
		Expect(payload).To(HaveKey("patient"))
		Expect(payload).To(HaveKey("deletedTime"))
	})

	It("backfills documents from Mongo in resumable batches", func() {
		collection := dbTest.GetTestDatabase().Collection("patient_deletions")
		clinicId := primitive.NewObjectID()

		ids := make([]string, 0, 5)
		for i := 0; i < 5; i++ {
			doc := newDeletionDoc(clinicId, primitive.NewObjectID().Hex())
			_, err := collection.InsertOne(ctx, doc)
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, doc["_id"].(primitive.ObjectID).Hex())
		}

		backfiller := deletionsPostgres.NewBackfiller("patient", dbTest.GetTestDatabase(), writer)
		Expect(backfiller.Collection()).To(Equal("patient_deletions"))

		copied := 0
		cursor := primitive.NilObjectID
		for {
			last, n, err := backfiller.BackfillBatch(ctx, cursor, 2)
			Expect(err).ToNot(HaveOccurred())
			if n == 0 {
				break
			}
			Expect(last.Hex() > cursor.Hex()).To(BeTrue(), "cursor must advance")
			cursor = last
			copied += n
		}
		Expect(copied).To(Equal(5))

		for _, id := range ids {
			Expect(count("SELECT count(*) FROM patient_deletions WHERE id = $1 AND clinic_id = $2", id, clinicId.Hex())).To(Equal(1))
		}

		// Re-running from the beginning converges instead of duplicating
		_, n, err := backfiller.BackfillBatch(ctx, primitive.NilObjectID, 100)
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(BeNumerically(">=", 5))
		Expect(count("SELECT count(*) FROM patient_deletions WHERE clinic_id = $1", clinicId.Hex())).To(Equal(5))
	})
})
