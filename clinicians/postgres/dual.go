package postgres

import (
	"context"

	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/store/dualwrite"
)

// NewDualRepository decorates the Mongo clinicians repository with
// best-effort mirroring to PostgreSQL.
func NewDualRepository(repo clinicians.Repository, writer *Writer, logger *zap.SugaredLogger) clinicians.Repository {
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
	clinicians.Repository
	writer *Writer
	logger *zap.SugaredLogger
}

func (d *dualRepository) mirrorClinician(ctx context.Context, clinician *clinicians.Clinician, operation string) {
	mirrored := *clinician
	dualwrite.Execute(ctx, d.logger, "clinicians", operation, func(ctx context.Context) error {
		return d.writer.UpsertClinician(ctx, &mirrored)
	})
}

func (d *dualRepository) Create(ctx context.Context, clinician *clinicians.Clinician) (*clinicians.Clinician, error) {
	result, err := d.Repository.Create(ctx, clinician)
	if err != nil {
		return result, err
	}
	d.mirrorClinician(ctx, result, "create")
	return result, nil
}

func (d *dualRepository) Update(ctx context.Context, update *clinicians.ClinicianUpdate) (*clinicians.Clinician, error) {
	result, err := d.Repository.Update(ctx, update)
	if err != nil {
		return result, err
	}
	d.mirrorClinician(ctx, result, "update")
	return result, nil
}

func (d *dualRepository) UpdateAll(ctx context.Context, update *clinicians.CliniciansUpdate) error {
	if err := d.Repository.UpdateAll(ctx, update); err != nil {
		return err
	}
	// The bulk update doesn't return the updated records; re-read every
	// clinician record of the user (bounded by the number of clinic
	// memberships) page by page and snapshot each one.
	userId := update.UserId
	dualwrite.Execute(ctx, d.logger, "clinicians", "update_all", func(ctx context.Context) error {
		const pageSize = 1000
		page := store.Pagination{Limit: pageSize}
		for {
			list, err := d.Repository.List(ctx, &clinicians.Filter{UserId: &userId}, page)
			if err != nil {
				return err
			}
			for _, clinician := range list {
				if err := d.writer.UpsertClinician(ctx, clinician); err != nil {
					return err
				}
			}
			if len(list) < pageSize {
				return nil
			}
			page.Offset += pageSize
		}
	})
	return nil
}

func (d *dualRepository) Delete(ctx context.Context, clinicId string, userId string, metadata deletions.Metadata) error {
	if err := d.Repository.Delete(ctx, clinicId, userId, metadata); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "clinicians", "delete", func(ctx context.Context) error {
		return d.writer.DeleteClinician(ctx, clinicId, userId)
	})
	return nil
}

func (d *dualRepository) DeleteAll(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	if err := d.Repository.DeleteAll(ctx, clinicId, metadata); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "clinicians", "delete_all", func(ctx context.Context) error {
		return d.writer.DeleteAllClinicians(ctx, clinicId)
	})
	return nil
}

func (d *dualRepository) DeleteInvite(ctx context.Context, clinicId, inviteId string) error {
	if err := d.Repository.DeleteInvite(ctx, clinicId, inviteId); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "clinicians", "delete_invite", func(ctx context.Context) error {
		return d.writer.DeleteClinicianInvite(ctx, clinicId, inviteId)
	})
	return nil
}

func (d *dualRepository) AssociateInvite(ctx context.Context, associate clinicians.AssociateInvite) (*clinicians.Clinician, error) {
	result, err := d.Repository.AssociateInvite(ctx, associate)
	if err != nil {
		return result, err
	}
	d.mirrorClinician(ctx, result, "associate_invite")
	return result, nil
}
