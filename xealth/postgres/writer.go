// Package postgres mirrors xealth documents to PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/xealth"
	"github.com/tidepool-org/clinic/xealth/postgres/sqlcgen"
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
	payload, err := storepg.MarshalPayload(data)
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
	payload, err := storepg.MarshalPayload(order)
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

// Backfillers copy the three xealth collections. They unmarshal raw documents
// into the domain models and reuse the writer upserts, so live dual writes
// and backfilled rows converge.

func NewPreorderBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(preorderCollection), "xealth_preorders",
		storepg.UnmarshalAndUpsert(func(ctx context.Context, data *xealth.PreorderFormData) error {
			return writer.UpsertPreorder(ctx, data)
		}))
}

func NewOrderBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(orderCollection), "xealth_orders",
		storepg.UnmarshalAndUpsert(func(ctx context.Context, order *xealth.OrderEvent) error {
			return writer.UpsertOrder(ctx, order)
		}))
}

func NewReportViewBackfiller(db *mongo.Database, writer *Writer) storepg.Verifier {
	return storepg.NewCollectionSync(db.Collection(reportViewCollection), "xealth_report_views",
		storepg.UnmarshalAndUpsert(func(ctx context.Context, view *xealth.ReportView) error {
			return writer.UpsertReportView(ctx, view)
		}))
}
