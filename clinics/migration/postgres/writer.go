// Package postgres mirrors clinic migrations to PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
func NewBackfiller(db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		collection: db.Collection(collectionName),
		writer:     writer,
	}
}

type backfiller struct {
	collection *mongo.Collection
	writer     *Writer
}

func (b *backfiller) Collection() string {
	return collectionName
}

func (b *backfiller) BackfillBatch(ctx context.Context, after primitive.ObjectID, limit int) (primitive.ObjectID, int, error) {
	return storepg.BackfillDocuments(ctx, b.collection, after, limit, func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			raw, err := bson.Marshal(doc)
			if err != nil {
				return err
			}
			m := &migration.Migration{}
			if err := bson.Unmarshal(raw, m); err != nil {
				return err
			}
			if err := b.writer.UpsertMigration(ctx, m); err != nil {
				return err
			}
		}
		return nil
	})
}
