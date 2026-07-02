// Package postgres mirrors xealth documents to PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
	"github.com/tidepool-org/clinic/xealth"
)

const (
	preorderCollection   = "xealth_preorder"
	orderCollection      = "xealth_order"
	reportViewCollection = "xealth_report_view"
)

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

func (w *Writer) UpsertPreorder(ctx context.Context, data *xealth.PreorderFormData) error {
	if data.Id == nil {
		return fmt.Errorf("preorder data has no id")
	}
	payload, err := marshalPayload(data)
	if err != nil {
		return err
	}
	return w.queries.UpsertXealthPreorder(ctx, sqlcgen.UpsertXealthPreorderParams{
		ID:             data.Id.Hex(),
		DataTrackingID: data.DataTrackingId,
		Payload:        payload,
	})
}

func (w *Writer) UpsertOrder(ctx context.Context, order *xealth.OrderEvent) error {
	if order.Id == nil {
		return fmt.Errorf("order event has no id")
	}
	payload, err := marshalPayload(order)
	if err != nil {
		return err
	}
	return w.queries.UpsertXealthOrder(ctx, sqlcgen.UpsertXealthOrderParams{
		ID:      order.Id.Hex(),
		Payload: payload,
	})
}

func (w *Writer) UpsertReportView(ctx context.Context, view *xealth.ReportView) error {
	if view.Id == nil {
		return fmt.Errorf("report view has no id")
	}
	systemLogin := pgtype.Text{}
	if view.SystemLogin != nil {
		systemLogin = pgtype.Text{String: *view.SystemLogin, Valid: true}
	}
	return w.queries.UpsertXealthReportView(ctx, sqlcgen.UpsertXealthReportViewParams{
		ID:            view.Id.Hex(),
		UserID:        view.UserId,
		DeploymentID:  view.DeploymentId,
		SystemLogin:   systemLogin,
		PatientUserID: view.PatientUserId,
		ProgramID:     view.ProgramId,
		ClinicID:      view.ClinicId.Hex(),
		CreatedTime:   pgtype.Timestamptz{Time: view.CreatedTime.UTC(), Valid: !view.CreatedTime.IsZero()},
	})
}

func marshalPayload(v interface{}) ([]byte, error) {
	normalized, err := storepg.NormalizeDocument(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// Backfillers copy the three xealth collections. They unmarshal raw documents
// into the domain models and reuse the writer upserts, so live dual writes
// and backfilled rows converge.

func NewPreorderBackfiller(db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		collection: db.Collection(preorderCollection),
		upsert: func(ctx context.Context, w *Writer, doc bson.M) error {
			return unmarshalAndUpsert(ctx, doc, func(ctx context.Context, data *xealth.PreorderFormData) error {
				return w.UpsertPreorder(ctx, data)
			})
		},
		writer: writer,
	}
}

func NewOrderBackfiller(db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		collection: db.Collection(orderCollection),
		upsert: func(ctx context.Context, w *Writer, doc bson.M) error {
			return unmarshalAndUpsert(ctx, doc, func(ctx context.Context, order *xealth.OrderEvent) error {
				return w.UpsertOrder(ctx, order)
			})
		},
		writer: writer,
	}
}

func NewReportViewBackfiller(db *mongo.Database, writer *Writer) storepg.Backfiller {
	return &backfiller{
		collection: db.Collection(reportViewCollection),
		upsert: func(ctx context.Context, w *Writer, doc bson.M) error {
			return unmarshalAndUpsert(ctx, doc, func(ctx context.Context, view *xealth.ReportView) error {
				return w.UpsertReportView(ctx, view)
			})
		},
		writer: writer,
	}
}

func unmarshalAndUpsert[T any](ctx context.Context, doc bson.M, upsert func(context.Context, *T) error) error {
	raw, err := bson.Marshal(doc)
	if err != nil {
		return err
	}
	model := new(T)
	if err := bson.Unmarshal(raw, model); err != nil {
		return err
	}
	return upsert(ctx, model)
}

type backfiller struct {
	collection *mongo.Collection
	upsert     func(ctx context.Context, w *Writer, doc bson.M) error
	writer     *Writer
}

func (b *backfiller) Collection() string {
	return b.collection.Name()
}

func (b *backfiller) BackfillBatch(ctx context.Context, after primitive.ObjectID, limit int) (primitive.ObjectID, int, error) {
	return storepg.BackfillDocuments(ctx, b.collection, after, limit, func(ctx context.Context, docs []bson.M) error {
		for _, doc := range docs {
			if err := b.upsert(ctx, b.writer, doc); err != nil {
				return err
			}
		}
		return nil
	})
}
