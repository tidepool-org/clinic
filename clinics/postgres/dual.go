package postgres

import (
	"context"

	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store/dualwrite"
)

// NewDualRepository decorates the Mongo clinics repository with best-effort
// mirroring to PostgreSQL. Most repository writes patch a single field of
// the clinic document, so the mirror re-reads the whole document from Mongo
// (the source of truth) and snapshot-upserts it, which converges regardless
// of which write triggered the mirror.
func NewDualRepository(repo clinics.Repository, writer *Writer, logger *zap.SugaredLogger) clinics.Repository {
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
	clinics.Repository
	writer *Writer
	logger *zap.SugaredLogger
}

// mirror re-reads the clinic and snapshots it to Postgres. It runs inside
// dualwrite.Execute, i.e. after the enclosing Mongo transaction commits.
func (d *dualRepository) mirror(ctx context.Context, clinicId string, operation string) {
	dualwrite.Execute(ctx, d.logger, "clinics", operation, func(ctx context.Context) error {
		clinic, err := d.Repository.Get(ctx, clinicId)
		if err != nil {
			return err
		}
		return d.writer.UpsertClinic(ctx, clinic)
	})
}

func (d *dualRepository) mirrorClinic(ctx context.Context, clinic *clinics.Clinic, operation string) {
	mirrored := *clinic
	dualwrite.Execute(ctx, d.logger, "clinics", operation, func(ctx context.Context) error {
		return d.writer.UpsertClinic(ctx, &mirrored)
	})
}

func (d *dualRepository) Create(ctx context.Context, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	result, err := d.Repository.Create(ctx, clinic)
	if err != nil {
		return result, err
	}
	d.mirrorClinic(ctx, result, "create")
	return result, nil
}

func (d *dualRepository) Update(ctx context.Context, id string, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	result, err := d.Repository.Update(ctx, id, clinic)
	if err != nil {
		return result, err
	}
	d.mirrorClinic(ctx, result, "update")
	return result, nil
}

func (d *dualRepository) Delete(ctx context.Context, id string, metadata deletions.Metadata) error {
	if err := d.Repository.Delete(ctx, id, metadata); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "clinics", "delete", func(ctx context.Context) error {
		return d.writer.DeleteClinic(ctx, id)
	})
	return nil
}

func (d *dualRepository) UpsertAdmin(ctx context.Context, clinicId, clinicianId string) error {
	if err := d.Repository.UpsertAdmin(ctx, clinicId, clinicianId); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "upsert_admin")
	return nil
}

func (d *dualRepository) RemoveAdmin(ctx context.Context, clinicId, clinicianId string, allowOrphaning bool) error {
	if err := d.Repository.RemoveAdmin(ctx, clinicId, clinicianId, allowOrphaning); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "remove_admin")
	return nil
}

func (d *dualRepository) UpdateTier(ctx context.Context, clinicId, tier string) error {
	if err := d.Repository.UpdateTier(ctx, clinicId, tier); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_tier")
	return nil
}

func (d *dualRepository) UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications clinics.SuppressedNotifications) error {
	if err := d.Repository.UpdateSuppressedNotifications(ctx, clinicId, suppressedNotifications); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_suppressed_notifications")
	return nil
}

func (d *dualRepository) CreatePatientTag(ctx context.Context, clinicId, tagName string) (*clinics.PatientTag, error) {
	result, err := d.Repository.CreatePatientTag(ctx, clinicId, tagName)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, "create_patient_tag")
	return result, nil
}

func (d *dualRepository) UpdatePatientTag(ctx context.Context, clinicId, tagId, tagName string) (*clinics.PatientTag, error) {
	result, err := d.Repository.UpdatePatientTag(ctx, clinicId, tagId, tagName)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, "update_patient_tag")
	return result, nil
}

func (d *dualRepository) DeletePatientTag(ctx context.Context, clinicId, tagId string) error {
	if err := d.Repository.DeletePatientTag(ctx, clinicId, tagId); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "delete_patient_tag")
	return nil
}

func (d *dualRepository) UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []clinics.MembershipRestrictions) error {
	if err := d.Repository.UpdateMembershipRestrictions(ctx, clinicId, restrictions); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_membership_restrictions")
	return nil
}

func (d *dualRepository) UpdateEHRSettings(ctx context.Context, clinicId string, settings *clinics.EHRSettings) error {
	if err := d.Repository.UpdateEHRSettings(ctx, clinicId, settings); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_ehr_settings")
	return nil
}

func (d *dualRepository) UpdateMRNSettings(ctx context.Context, clinicId string, settings *clinics.MRNSettings) error {
	if err := d.Repository.UpdateMRNSettings(ctx, clinicId, settings); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_mrn_settings")
	return nil
}

func (d *dualRepository) UpdatePatientCountSettings(ctx context.Context, clinicId string, settings *clinics.PatientCountSettings) error {
	if err := d.Repository.UpdatePatientCountSettings(ctx, clinicId, settings); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_patient_count_settings")
	return nil
}

func (d *dualRepository) UpdatePatientCount(ctx context.Context, clinicId string, patientCount *clinics.PatientCount) error {
	if err := d.Repository.UpdatePatientCount(ctx, clinicId, patientCount); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "update_patient_count")
	return nil
}

func (d *dualRepository) AppendShareCodes(ctx context.Context, clinicId string, shareCodes []string) error {
	if err := d.Repository.AppendShareCodes(ctx, clinicId, shareCodes); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "append_share_codes")
	return nil
}

func (d *dualRepository) CreateSite(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error) {
	result, err := d.Repository.CreateSite(ctx, clinicId, site)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, "create_site")
	return result, nil
}

func (d *dualRepository) CreateSiteIgnoringLimit(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error) {
	result, err := d.Repository.CreateSiteIgnoringLimit(ctx, clinicId, site)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, "create_site")
	return result, nil
}

func (d *dualRepository) DeleteSite(ctx context.Context, clinicId, siteId string) error {
	if err := d.Repository.DeleteSite(ctx, clinicId, siteId); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, "delete_site")
	return nil
}

func (d *dualRepository) UpdateSite(ctx context.Context, clinicId, siteId string, site *sites.Site) (*sites.Site, error) {
	result, err := d.Repository.UpdateSite(ctx, clinicId, siteId, site)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, "update_site")
	return result, nil
}
