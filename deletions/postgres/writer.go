// Package postgres mirrors deletion audit records to PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/store/dualwrite"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
)

const (
	TypePatient   = "patient"
	TypeClinician = "clinician"
	TypeClinic    = "clinic"
)

// NewMirror adapts the writer to the deletions.Mirror interface consumed by
// the Mongo deletions repositories.
func NewMirror(client *storepg.Client, logger *zap.SugaredLogger) deletions.Mirror {
	return NewWriter(client, logger)
}

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

var _ deletions.Mirror = &Writer{}

// CreateDeletions mirrors deletion audit documents best-effort; it never
// fails the caller.
func (w *Writer) CreateDeletions(ctx context.Context, documentType string, docs []bson.M) {
	if w.queries == nil {
		return
	}
	dualwrite.Execute(ctx, w.logger, fmt.Sprintf("%s_deletions", documentType), "create", func(ctx context.Context) error {
		return w.UpsertDocuments(ctx, documentType, docs)
	})
}

// UpsertDocuments idempotently upserts deletion audit documents. Documents
// are the exact shape persisted in Mongo: either the bson.M built by the
// deletions repository (typed entity inside) or a raw document read back
// during backfill.
func (w *Writer) UpsertDocuments(ctx context.Context, documentType string, docs []bson.M) error {
	for _, doc := range docs {
		normalized, err := storepg.NormalizeDocument(doc)
		if err != nil {
			return fmt.Errorf("error normalizing %s deletion: %w", documentType, err)
		}

		id, ok := normalized["_id"].(primitive.ObjectID)
		if !ok {
			return fmt.Errorf("%s deletion has a non object id key", documentType)
		}
		deletedTime, ok := normalized["deletedTime"].(time.Time)
		if !ok {
			return fmt.Errorf("%s deletion %s has no deleted time", documentType, id.Hex())
		}

		payload, err := json.Marshal(normalized)
		if err != nil {
			return fmt.Errorf("error marshaling %s deletion %s: %w", documentType, id.Hex(), err)
		}

		entity, _ := normalized[documentType].(bson.M)
		params := deletionParams{
			id:              id.Hex(),
			deletedTime:     deletedTime,
			deletedByUserId: stringValue(normalized["deletedByUserId"]),
			payload:         payload,
		}

		switch documentType {
		case TypePatient, TypeClinician:
			params.clinicId = objectIdHex(entity["clinicId"])
			params.userId = stringValue(entity["userId"])
		case TypeClinic:
			params.clinicId = objectIdHex(entity["_id"])
		default:
			return fmt.Errorf("unknown deletion document type %s", documentType)
		}

		if err := w.upsert(ctx, documentType, params); err != nil {
			return fmt.Errorf("error upserting %s deletion %s: %w", documentType, id.Hex(), err)
		}
	}
	return nil
}

type deletionParams struct {
	id              string
	deletedTime     time.Time
	deletedByUserId *string
	clinicId        *string
	userId          *string
	payload         []byte
}

func (w *Writer) upsert(ctx context.Context, documentType string, params deletionParams) error {
	switch documentType {
	case TypePatient:
		return w.queries.UpsertPatientDeletion(ctx, sqlcgen.UpsertPatientDeletionParams{
			ID:              params.id,
			DeletedTime:     pgtype.Timestamptz{Time: params.deletedTime, Valid: true},
			DeletedByUserID: textValue(params.deletedByUserId),
			ClinicID:        textValue(params.clinicId),
			UserID:          textValue(params.userId),
			Payload:         params.payload,
		})
	case TypeClinician:
		return w.queries.UpsertClinicianDeletion(ctx, sqlcgen.UpsertClinicianDeletionParams{
			ID:              params.id,
			DeletedTime:     pgtype.Timestamptz{Time: params.deletedTime, Valid: true},
			DeletedByUserID: textValue(params.deletedByUserId),
			ClinicID:        textValue(params.clinicId),
			UserID:          textValue(params.userId),
			Payload:         params.payload,
		})
	case TypeClinic:
		return w.queries.UpsertClinicDeletion(ctx, sqlcgen.UpsertClinicDeletionParams{
			ID:              params.id,
			DeletedTime:     pgtype.Timestamptz{Time: params.deletedTime, Valid: true},
			DeletedByUserID: textValue(params.deletedByUserId),
			ClinicID:        textValue(params.clinicId),
			Payload:         params.payload,
		})
	default:
		return fmt.Errorf("unknown deletion document type %s", documentType)
	}
}

func stringValue(v interface{}) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func objectIdHex(v interface{}) *string {
	if id, ok := v.(primitive.ObjectID); ok {
		hex := id.Hex()
		return &hex
	}
	return stringValue(v)
}

func textValue(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

// NewBackfiller backfills one of the deletions collections.
func NewBackfiller(documentType string, db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		documentType: documentType,
		collection:   db.Collection(fmt.Sprintf("%s_deletions", documentType)),
		writer:       writer,
	}
}

type backfiller struct {
	documentType string
	collection   *mongo.Collection
	writer       *Writer
}

func (b *backfiller) Collection() string {
	return b.collection.Name()
}

func (b *backfiller) BackfillBatch(ctx context.Context, after primitive.ObjectID, limit int) (primitive.ObjectID, int, error) {
	return storepg.BackfillDocuments(ctx, b.collection, after, limit, func(ctx context.Context, docs []bson.M) error {
		return b.writer.UpsertDocuments(ctx, b.documentType, docs)
	})
}
