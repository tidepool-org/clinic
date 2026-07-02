// Package postgres mirrors clinic migrations to PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics/migration"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
)

const collectionName = "migrations"

func NewWriter(client *storepg.Client, logger *zap.SugaredLogger) *Writer {
	w := &Writer{logger: logger}
	if client.Enabled() {
		w.queries = sqlcgen.New(client.Pool())
	}
	return w
}

type Writer struct {
	queries *sqlcgen.Queries
	logger  *zap.SugaredLogger
}

func (w *Writer) Enabled() bool {
	return w.queries != nil
}

func (w *Writer) UpsertMigration(ctx context.Context, m *migration.Migration) error {
	if m.UserId == "" {
		return fmt.Errorf("migration has no user id")
	}
	clinicId := ""
	if m.ClinicId != nil {
		clinicId = m.ClinicId.Hex()
	}
	return w.queries.UpsertMigration(ctx, sqlcgen.UpsertMigrationParams{
		UserID:      m.UserId,
		ClinicID:    clinicId,
		Status:      m.Status,
		CreatedTime: pgtype.Timestamptz{Time: m.CreatedTime.UTC(), Valid: !m.CreatedTime.IsZero()},
		UpdatedTime: pgtype.Timestamptz{Time: m.UpdatedTime.UTC(), Valid: !m.UpdatedTime.IsZero()},
	})
}

// NewBackfiller backfills the migrations collection.
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	collection := db.Collection(collectionName)
	upsert := func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			raw, err := bson.Marshal(doc)
			if err != nil {
				return err
			}
			m := &migration.Migration{}
			if err := bson.Unmarshal(raw, m); err != nil {
				return err
			}
			if err := writer.UpsertMigration(ctx, m); err != nil {
				return err
			}
		}
		return nil
	}
	return &verifier{
		CollectionSync: storepg.NewCollectionSync(collection, "migrations", upsert),
		collection:     collection,
		upsert:         upsert,
	}
}

// verifier overrides the identity methods of CollectionSync: the migrations
// table is keyed by the globally unique user id instead of the Mongo object
// id, which the Migration domain model does not expose.
type verifier struct {
	*storepg.CollectionSync
	collection *mongo.Collection
	upsert     func(ctx context.Context, docs []bson.M) error
}

func (v *verifier) IDColumn() string {
	return "user_id"
}

func (v *verifier) MongoIDs(ctx context.Context, after string, limit int) ([]string, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "userId", Value: 1}}).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"userId": 1})
	cursor, err := v.collection.Find(ctx, bson.M{"userId": bson.M{"$gt": after}}, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing migration user ids: %w", err)
	}

	var docs []struct {
		UserId string `bson:"userId"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("error decoding migration user ids: %w", err)
	}

	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.UserId)
	}
	return ids, nil
}

func (v *verifier) ResyncBatch(ctx context.Context, ids []string) error {
	cursor, err := v.collection.Find(ctx, bson.M{"userId": bson.M{"$in": ids}})
	if err != nil {
		return fmt.Errorf("error listing migrations: %w", err)
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return fmt.Errorf("error decoding migrations: %w", err)
	}
	if len(docs) == 0 {
		return nil
	}
	return v.upsert(ctx, docs)
}
