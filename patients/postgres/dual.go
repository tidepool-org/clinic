package postgres

import (
	"context"

	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store/dualwrite"
)

// NewDualRepository decorates the Mongo patients repository with best-effort
// mirroring to PostgreSQL. Mirror inputs are always complete Mongo documents
// (either the entity returned by the write or a re-read of the affected
// documents), so snapshot upserts converge regardless of which write fired.
// Bulk tag and site operations are mirrored with targeted SQL against rows
// that are already mirrored, matching the Mongo update semantics.
func NewDualRepository(repo patients.Repository, writer *Writer, logger *zap.SugaredLogger) patients.Repository {
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
	patients.Repository
	writer *Writer
	logger *zap.SugaredLogger
}

// mirrorPatient snapshots an entity returned by a repository write.
func (d *dualRepository) mirrorPatient(ctx context.Context, patient *patients.Patient, operation string) {
	mirrored := *patient
	dualwrite.Execute(ctx, d.logger, "patients", operation, func(ctx context.Context) error {
		return d.writer.UpsertPatient(ctx, &mirrored)
	})
}

// mirror re-reads the patient from Mongo and snapshots it, for writes that
// don't return the updated entity.
func (d *dualRepository) mirror(ctx context.Context, clinicId, userId string, operation string) {
	dualwrite.Execute(ctx, d.logger, "patients", operation, func(ctx context.Context) error {
		patient, err := d.Repository.Get(ctx, clinicId, userId)
		if err != nil {
			return err
		}
		return d.writer.UpsertPatient(ctx, patient)
	})
}

// mirrorAllClinics re-reads every membership of the user and snapshots each,
// for bounded fan-out writes keyed by user id.
func (d *dualRepository) mirrorAllClinics(ctx context.Context, userId string, operation string) {
	dualwrite.Execute(ctx, d.logger, "patients", operation, func(ctx context.Context) error {
		clinicIds, err := d.Repository.ClinicIds(ctx, userId)
		if err != nil {
			return err
		}
		for _, clinicId := range clinicIds {
			patient, err := d.Repository.Get(ctx, clinicId, userId)
			if err != nil {
				return err
			}
			if err := d.writer.UpsertPatient(ctx, patient); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *dualRepository) Create(ctx context.Context, patient patients.Patient) (*patients.Patient, error) {
	result, err := d.Repository.Create(ctx, patient)
	if err != nil {
		return result, err
	}
	d.mirrorPatient(ctx, result, "create")
	return result, nil
}

func (d *dualRepository) Update(ctx context.Context, update patients.PatientUpdate) (*patients.Patient, error) {
	result, err := d.Repository.Update(ctx, update)
	if err != nil {
		return result, err
	}
	d.mirrorPatient(ctx, result, "update")
	return result, nil
}

func (d *dualRepository) AddReview(ctx context.Context, clinicId, userId string, review patients.Review) ([]patients.Review, error) {
	result, err := d.Repository.AddReview(ctx, clinicId, userId, review)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, userId, "add_review")
	return result, nil
}

func (d *dualRepository) DeleteReview(ctx context.Context, clinicId, clinicianId, userId string) ([]patients.Review, error) {
	result, err := d.Repository.DeleteReview(ctx, clinicId, clinicianId, userId)
	if err != nil {
		return result, err
	}
	d.mirror(ctx, clinicId, userId, "delete_review")
	return result, nil
}

func (d *dualRepository) UpdateEmail(ctx context.Context, userId string, email *string) error {
	if err := d.Repository.UpdateEmail(ctx, userId, email); err != nil {
		return err
	}
	d.mirrorAllClinics(ctx, userId, "update_email")
	return nil
}

func (d *dualRepository) Remove(ctx context.Context, clinicId string, userId string, metadata deletions.Metadata) error {
	if err := d.Repository.Remove(ctx, clinicId, userId, metadata); err != nil {
		return err
	}
	// The deletion audit record is mirrored by the deletions mirror
	dualwrite.Execute(ctx, d.logger, "patients", "remove", func(ctx context.Context) error {
		return d.writer.DeletePatient(ctx, clinicId, userId)
	})
	return nil
}

func (d *dualRepository) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *patients.Permissions) (*patients.Patient, error) {
	result, err := d.Repository.UpdatePermissions(ctx, clinicId, userId, permissions)
	if err != nil {
		return result, err
	}
	d.mirrorPatient(ctx, result, "update_permissions")
	return result, nil
}

func (d *dualRepository) DeletePermission(ctx context.Context, clinicId, userId, permission string) (*patients.Patient, error) {
	result, err := d.Repository.DeletePermission(ctx, clinicId, userId, permission)
	if err != nil {
		return result, err
	}
	d.mirrorPatient(ctx, result, "delete_permission")
	return result, nil
}

func (d *dualRepository) DeleteFromAllClinics(ctx context.Context, userId string, metadata deletions.Metadata) ([]string, error) {
	result, err := d.Repository.DeleteFromAllClinics(ctx, userId, metadata)
	if err != nil {
		return result, err
	}
	dualwrite.Execute(ctx, d.logger, "patients", "delete_from_all_clinics", func(ctx context.Context) error {
		return d.writer.DeletePatientsByUserId(ctx, userId)
	})
	return result, nil
}

func (d *dualRepository) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	if err := d.Repository.DeleteNonCustodialPatientsOfClinic(ctx, clinicId, metadata); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "patients", "delete_non_custodial", func(ctx context.Context) error {
		return d.writer.DeleteNonCustodialPatientsOfClinic(ctx, clinicId)
	})
	return nil
}

func (d *dualRepository) UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *patients.Summary) error {
	if err := d.Repository.UpdateSummaryInAllClinics(ctx, userId, summary); err != nil {
		return err
	}
	d.mirrorAllClinics(ctx, userId, "update_summary")
	return nil
}

func (d *dualRepository) DeleteSummaryInAllClinics(ctx context.Context, summaryId string) error {
	if err := d.Repository.DeleteSummaryInAllClinics(ctx, summaryId); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "patients", "delete_summary", func(ctx context.Context) error {
		return d.writer.DeleteSummariesBySummaryId(ctx, summaryId)
	})
	return nil
}

func (d *dualRepository) UpdateLastUploadReminderTime(ctx context.Context, update *patients.UploadReminderUpdate) (*patients.Patient, error) {
	result, err := d.Repository.UpdateLastUploadReminderTime(ctx, update)
	if err != nil {
		return result, err
	}
	d.mirrorPatient(ctx, result, "update_upload_reminder")
	return result, nil
}

func (d *dualRepository) AddProviderConnectionRequest(ctx context.Context, clinicId, userId string, request patients.ConnectionRequest) error {
	if err := d.Repository.AddProviderConnectionRequest(ctx, clinicId, userId, request); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, userId, "add_connection_request")
	return nil
}

func (d *dualRepository) AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	if err := d.Repository.AssignPatientTagToClinicPatients(ctx, clinicId, tagId, patientIds); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "patients", "assign_tag", func(ctx context.Context) error {
		return d.writer.AssignTag(ctx, clinicId, tagId, patientIds)
	})
	return nil
}

func (d *dualRepository) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	if err := d.Repository.DeletePatientTagFromClinicPatients(ctx, clinicId, tagId, patientIds); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "patients", "delete_tag", func(ctx context.Context) error {
		return d.writer.DeleteTag(ctx, clinicId, tagId, patientIds)
	})
	return nil
}

func (d *dualRepository) ConvertPatientTagToSite(ctx context.Context, clinicId, patientTagId string, site *sites.Site) error {
	if err := d.Repository.ConvertPatientTagToSite(ctx, clinicId, patientTagId, site); err != nil {
		return err
	}
	siteId := site.Id.Hex()
	siteName := site.Name
	dualwrite.Execute(ctx, d.logger, "patients", "convert_tag_to_site", func(ctx context.Context) error {
		return d.writer.ConvertTagToSite(ctx, clinicId, patientTagId, siteId, siteName)
	})
	return nil
}

func (d *dualRepository) UpdatePatientDataSources(ctx context.Context, userId string, dataSources *patients.DataSources) error {
	if err := d.Repository.UpdatePatientDataSources(ctx, userId, dataSources); err != nil {
		return err
	}
	d.mirrorAllClinics(ctx, userId, "update_data_sources")
	return nil
}

func (d *dualRepository) UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update patients.SubscriptionUpdate) error {
	if err := d.Repository.UpdateEHRSubscription(ctx, clinicId, userId, update); err != nil {
		return err
	}
	d.mirror(ctx, clinicId, userId, "update_ehr_subscription")
	return nil
}

func (d *dualRepository) DeleteSites(ctx context.Context, clinicId string, siteId string) error {
	if err := d.Repository.DeleteSites(ctx, clinicId, siteId); err != nil {
		return err
	}
	dualwrite.Execute(ctx, d.logger, "patients", "delete_sites", func(ctx context.Context) error {
		return d.writer.DeleteSite(ctx, clinicId, siteId)
	})
	return nil
}

func (d *dualRepository) UpdateSites(ctx context.Context, clinicId string, siteId string, site *sites.Site) error {
	if err := d.Repository.UpdateSites(ctx, clinicId, siteId, site); err != nil {
		return err
	}
	name := site.Name
	dualwrite.Execute(ctx, d.logger, "patients", "update_sites", func(ctx context.Context) error {
		return d.writer.RenameSite(ctx, clinicId, siteId, name)
	})
	return nil
}

func (d *dualRepository) MergeSites(ctx context.Context, clinicId, sourceSiteId string, targetSite *sites.Site) error {
	if err := d.Repository.MergeSites(ctx, clinicId, sourceSiteId, targetSite); err != nil {
		return err
	}
	targetSiteId := targetSite.Id.Hex()
	targetSiteName := targetSite.Name
	dualwrite.Execute(ctx, d.logger, "patients", "merge_sites", func(ctx context.Context) error {
		return d.writer.MergeSites(ctx, clinicId, sourceSiteId, targetSiteId, targetSiteName)
	})
	return nil
}
