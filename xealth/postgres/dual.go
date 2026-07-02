package postgres

import (
	"context"

	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/store/dualwrite"
	"github.com/tidepool-org/clinic/xealth"
)

// NewDualStore decorates the Mongo xealth store with best-effort mirroring
// to PostgreSQL. Mongo remains the source of truth; reads pass through.
func NewDualStore(store xealth.Store, writer *Writer, logger *zap.SugaredLogger) xealth.Store {
	if !writer.Enabled() {
		return store
	}
	return &dualStore{
		Store:  store,
		writer: writer,
		logger: logger,
	}
}

type dualStore struct {
	xealth.Store
	writer *Writer
	logger *zap.SugaredLogger
}

func (d *dualStore) CreatePreorderData(ctx context.Context, data xealth.PreorderFormData) error {
	if err := d.Store.CreatePreorderData(ctx, data); err != nil {
		return err
	}
	// The Mongo store doesn't return the created document, so re-read it by
	// its unique data tracking id inside the mirror operation.
	dataTrackingId := data.DataTrackingId
	dualwrite.Execute(ctx, d.logger, "xealth_preorder", "create", func(ctx context.Context) error {
		created, err := d.Store.GetPreorderData(ctx, dataTrackingId)
		if err != nil {
			return err
		}
		return d.writer.UpsertPreorder(ctx, created)
	})
	return nil
}

func (d *dualStore) CreateOrder(ctx context.Context, order xealth.OrderEvent) (*xealth.OrderEvent, error) {
	result, err := d.Store.CreateOrder(ctx, order)
	if err != nil {
		return result, err
	}
	mirrored := *result
	dualwrite.Execute(ctx, d.logger, "xealth_order", "create", func(ctx context.Context) error {
		return d.writer.UpsertOrder(ctx, &mirrored)
	})
	return result, nil
}

func (d *dualStore) CreateReportView(ctx context.Context, view xealth.ReportView) (*xealth.ReportView, error) {
	result, err := d.Store.CreateReportView(ctx, view)
	if err != nil {
		return result, err
	}
	mirrored := *result
	dualwrite.Execute(ctx, d.logger, "xealth_report_view", "create", func(ctx context.Context) error {
		return d.writer.UpsertReportView(ctx, &mirrored)
	})
	return result, nil
}
