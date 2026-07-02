package postgres

import (
	"context"

	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/store/dualwrite"
)

// NewDualRepository decorates the Mongo migrations repository with
// best-effort mirroring to PostgreSQL. Mongo remains the source of truth;
// reads pass through.
func NewDualRepository(repo migration.Repository, writer *Writer, logger *zap.SugaredLogger) migration.Repository {
	if !writer.Enabled() {
		return repo
	}
	return &dualRepository{
		Repository: repo,
		writer:     writer,
		logger:     logger,
	}
}

type dualRepository struct {
	migration.Repository
	writer *Writer
	logger *zap.SugaredLogger
}

func (d *dualRepository) Create(ctx context.Context, m *migration.Migration) (*migration.Migration, error) {
	result, err := d.Repository.Create(ctx, m)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, "create", result)
	return result, nil
}

func (d *dualRepository) UpdateStatus(ctx context.Context, clinicId, userId, status string) (*migration.Migration, error) {
	result, err := d.Repository.UpdateStatus(ctx, clinicId, userId, status)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, "update_status", result)
	return result, nil
}

func (d *dualRepository) mirror(ctx context.Context, operation string, m *migration.Migration) {
	mirrored := *m
	dualwrite.Execute(ctx, d.logger, "migrations", operation, func(ctx context.Context) error {
		return d.writer.UpsertMigration(ctx, &mirrored)
	})
}
