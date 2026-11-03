package api

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tealeg/xlsx/v3"

	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/clinics/merge"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/pointer"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

func (h *Handler) ListClinics(ec echo.Context, params ListClinicsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)

	filter := clinics.Filter{}
	if params.ShareCode != nil {
		filter.ShareCodes = []string{*params.ShareCode}
	}
	if params.CreatedTimeStart != nil {
		start := *params.CreatedTimeStart
		if !start.IsZero() {
			filter.CreatedTimeStart = &start
		}
	}
	if params.CreatedTimeEnd != nil {
		end := *params.CreatedTimeEnd
		if !end.IsZero() {
			filter.CreatedTimeEnd = &end
		}
	}
	if params.EhrEnabled != nil {
		filter.EHREnabled = params.EhrEnabled
	}

	list, err := h.Clinics.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicsDto(list))
}

func (h *Handler) CreateClinic(ec echo.Context) error {
	ctx := ec.Request().Context()
	dto := ClinicV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	authData := auth.GetAuthData(ctx)
	if authData == nil || authData.SubjectId == "" {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "expected authenticated user id",
		}
	}

	clinic := NewClinicWithDefaults(dto)

	// Set new clinic migration status to true.
	// Only clinics created via `EnableNewClinicExperience` handler should be subject to initial clinician patient migration
	clinic.IsMigrated = true

	create := manager.CreateClinic{
		Clinic:            *clinic,
		CreatorUserId:     authData.SubjectId,
		CreateDemoPatient: true,
	}

	result, err := h.ClinicsManager.CreateClinic(ctx, &create)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) GetClinic(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	clinic, err := h.Clinics.Get(ctx, string(clinicId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(clinic))
}

func (h *Handler) UpdateClinic(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := ClinicV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}
	result, err := h.Clinics.Update(ctx, string(clinicId), NewClinic(dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) DeleteClinic(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	var metadata deletions.Metadata
	authData := auth.GetAuthData(ctx)
	if authData != nil && authData.ServerAccess == false {
		metadata.DeletedByUserId = &authData.SubjectId
	}

	err := h.ClinicsManager.DeleteClinic(ctx, clinicId, metadata)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) GetClinicByShareCode(ec echo.Context, shareCode string) error {
	if shareCode == "" {
		return fmt.Errorf("share code is missing")
	}

	ctx := ec.Request().Context()
	filter := clinics.Filter{
		ShareCodes: []string{shareCode},
	}

	list, err := h.Clinics.List(ctx, &filter, store.Pagination{Limit: 2})
	if err != nil {
		return err
	}

	if len(list) == 0 {
		return errors.NotFound
	} else if len(list) > 1 {
		return fmt.Errorf("duplicate sharecode %v", shareCode)
	}

	return ec.JSON(http.StatusOK, NewClinicDto(list[0]))
}

func (h *Handler) TriggerInitialMigration(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	migration, err := h.ClinicsMigrator.TriggerInitialMigration(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) ListMigrations(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	migrations, err := h.ClinicsMigrator.ListMigrations(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDtos(migrations))
}

func (h *Handler) MigrateLegacyClinicianPatients(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	dto := MigrationV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	migration, err := h.ClinicsMigrator.MigrateLegacyClinicianPatients(ctx, clinicId, dto.UserId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) GetMigration(ec echo.Context, clinicId ClinicIdV1, userId UserId) error {
	ctx := ec.Request().Context()

	migration, err := h.ClinicsMigrator.GetMigration(ctx, string(clinicId), string(userId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) UpdateMigration(ec echo.Context, clinicId ClinicIdV1, userId UserId) error {
	ctx := ec.Request().Context()
	dto := MigrationUpdateV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	migration, err := h.ClinicsMigrator.UpdateMigrationStatus(ctx, string(clinicId), string(userId), string(dto.Status))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) DeleteUserFromClinics(ec echo.Context, userId UserId) error {
	ctx := ec.Request().Context()

	var metadata deletions.Metadata
	authData := auth.GetAuthData(ctx)
	if authData != nil && authData.ServerAccess == false {
		metadata.DeletedByUserId = &authData.SubjectId
	}

	if _, err := h.Patients.DeleteFromAllClinics(ctx, userId, metadata); err != nil {
		return err
	}
	if err := h.Clinicians.DeleteFromAllClinics(ctx, userId, metadata); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) UpdateClinicUserDetails(ec echo.Context, userId UserId) error {
	ctx := ec.Request().Context()
	dto := UpdateUserDetailsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	id := string(userId)

	var email *string
	if dto.Email != nil {
		email = strp(string(*dto.Email))
		update := &clinicians.CliniciansUpdate{
			UserId: id,
			Email:  *email,
		}
		if err := h.Clinicians.UpdateAll(ctx, update); err != nil {
			return err
		}
	}

	if err := h.Patients.UpdateEmail(ctx, id, email); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) UpdateTier(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := UpdateTier{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	if err := h.Clinics.UpdateTier(ctx, string(clinicId), string(dto.Tier)); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) UpdateSuppressedNotifications(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	var dto UpdateSuppressedNotifications
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	if err := h.Clinics.UpdateSuppressedNotifications(ctx, string(clinicId), clinics.SuppressedNotifications(dto.SuppressedNotifications)); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) CreatePatientTag(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := clinics.PatientTag{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	updated, err := h.Clinics.CreatePatientTag(ctx, string(clinicId), dto.Name)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientTagDto(updated))
}

func (h *Handler) UpdatePatientTag(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()
	dto := clinics.PatientTag{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	updated, err := h.Clinics.UpdatePatientTag(ctx, string(clinicId), string(patientTagId), dto.Name)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientTagDto(updated))
}

func (h *Handler) DeletePatientTag(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()

	err := h.Clinics.DeletePatientTag(ctx, string(clinicId), string(patientTagId))
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) ListMembershipRestrictions(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	updated, err := h.Clinics.ListMembershipRestrictions(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMembershipRestrictionsDto(updated))
}

func (h *Handler) UpdateMembershipRestrictions(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := MembershipRestrictionsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	if err := h.Clinics.UpdateMembershipRestrictions(ctx, clinicId, NewMembershipRestrictions(dto)); err != nil {
		return err
	}

	return h.ListMembershipRestrictions(ec, clinicId)
}

func (h *Handler) GetEHRSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	settings, err := h.Clinics.GetEHRSettings(ctx, clinicId)
	if err != nil {
		return err
	}

	if settings == nil {
		return errors.NotFound
	}

	response := NewEHRSettingsDto(settings)
	return ec.JSON(http.StatusOK, response)
}

func (h *Handler) UpdateEHRSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := EhrSettingsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	settings := NewEHRSettings(dto)
	err := h.Clinics.UpdateEHRSettings(ctx, clinicId, settings)
	if err != nil {
		return err
	}

	return h.GetEHRSettings(ec, clinicId)
}

func (h *Handler) GetMRNSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	settings, err := h.Clinics.GetMRNSettings(ctx, clinicId)
	if err != nil {
		return err
	}

	if settings == nil {
		return errors.NotFound
	}

	return ec.JSON(http.StatusOK, MrnSettingsV1{
		Required: settings.Required,
		Unique:   settings.Unique,
	})
}

func (h *Handler) UpdateMRNSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := MrnSettingsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	err := h.Clinics.UpdateMRNSettings(ctx, clinicId, &clinics.MRNSettings{
		Required: dto.Required,
		Unique:   dto.Unique,
	})

	if err != nil {
		return err
	}

	return h.GetMRNSettings(ec, clinicId)
}

func (h *Handler) GetPatientCountSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	patientCountSettings, err := h.Clinics.GetPatientCountSettings(ctx, clinicId)
	if err != nil {
		return err
	} else if patientCountSettings == nil {
		return errors.NotFound
	}

	return ec.JSON(http.StatusOK, NewPatientCountSettingsDto(patientCountSettings))
}

func (h *Handler) UpdatePatientCountSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := PatientCountSettingsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	patientCountSettings := NewPatientCountSettings(dto)
	if patientCountSettings != nil && !patientCountSettings.IsValid() {
		return errors.BadRequest
	}

	if err := h.Clinics.UpdatePatientCountSettings(ctx, clinicId, patientCountSettings); err != nil {
		return err
	}

	return h.GetPatientCountSettings(ec, clinicId)
}

func (h *Handler) GetPatientCount(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	patientCount, err := h.ClinicsManager.GetClinicPatientCount(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientCountDto(patientCount))
}

func (h *Handler) RefreshPatientCount(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	if err := h.ClinicsManager.RefreshClinicPatientCount(ctx, clinicId); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) GenerateMergeReport(ec echo.Context, clinicId ClinicId) (err error) {
	ctx := ec.Request().Context()
	format := "xlsx"
	if ec.Request().Header.Get("Accept") == "application/json" {
		format = "json"
	}
	dto := GenerateMergeReportV1{}
	defer func() {
		h.logMergeAudit(ctx, "GenerateMergeReport invoked", err, "clinicId", clinicId, "sourceId", dto.SourceId, "format", format)
	}()
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	// Without a source clinic, report the path clinic's duplicates alone
	var plan any
	var generate func() (*xlsx.File, error)
	filename := "merge-report"
	if dto.SourceId == nil {
		duplicatesPlan, err := merge.PlanDuplicatesReport(ctx, h.Clinics, h.Patients, clinicId)
		if err != nil {
			return err
		}
		plan, generate, filename = duplicatesPlan, merge.NewDuplicatesReport(duplicatesPlan).Generate, "duplicates-report"
	} else {
		if *dto.SourceId == "" || *dto.SourceId == clinicId {
			return fmt.Errorf("a distinct source clinic is required: %w", errors.BadRequest)
		}
		if err := h.authorizeMergeSource(ctx, *dto.SourceId); err != nil {
			return err
		}

		planner := merge.NewClinicMergePlanner(h.Clinics, h.Patients, h.Clinicians, *dto.SourceId, clinicId)
		mergePlan, err := planner.Plan(ctx)
		if err != nil {
			return err
		}
		plan, generate = mergePlan, merge.NewReport(mergePlan).Generate
	}

	if format == "json" {
		return ec.JSON(http.StatusOK, plan)
	}

	file, err := generate()
	if err != nil {
		return err
	}

	disposition := fmt.Sprintf("attachment; filename=%s-%d.xlsx", filename, time.Now().Unix())
	ec.Response().Header().Set(echo.HeaderContentDisposition, disposition)
	ec.Response().Header().Set(echo.HeaderContentType, "application/vnd.ms-excel")
	ec.Response().WriteHeader(http.StatusOK)
	return file.Write(ec.Response())
}

func (h *Handler) MergeClinic(ec echo.Context, clinicId ClinicId) (err error) {
	ctx := ec.Request().Context()
	dto := MergeClinicV1{}
	defer func() {
		h.logMergeAudit(ctx, "MergeClinic invoked", err, "clinicId", clinicId, "sourceId", dto.SourceId)
	}()
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	if dto.SourceId == nil || *dto.SourceId == "" || *dto.SourceId == clinicId {
		return fmt.Errorf("a distinct source clinic is required: %w", errors.BadRequest)
	}
	if err := h.authorizeMergeSource(ctx, *dto.SourceId); err != nil {
		return err
	}

	var consent *merge.Consent
	authData := auth.GetAuthData(ctx)
	if !auth.IsServerAuth(authData) {
		if dto.Confirmed == nil || !*dto.Confirmed {
			return fmt.Errorf("merge confirmation is required: %w", errors.BadRequest)
		}
		clinician, err := h.Clinicians.Get(ctx, clinicId, authData.SubjectId)
		if err != nil {
			return err
		}
		if clinician == nil || !clinician.IsAdmin() {
			return errors.HttpError{Code: http.StatusForbidden, Err: auth.ErrUnauthorized}
		}
		consent = &merge.Consent{ConfirmingUserId: authData.SubjectId, ConfirmingName: pointer.ToString(clinician.Name),
			ConfirmingEmail: pointer.ToString(clinician.Email), ConfirmedAt: time.Now().UTC(), Confirmed: *dto.Confirmed}
	}

	options := merge.Options{SourcePatientTag: dto.SourcePatientTag, SourcePatientSite: dto.SourcePatientSite}
	planner := merge.NewClinicMergePlanner(h.Clinics, h.Patients, h.Clinicians, *dto.SourceId, clinicId, options)
	plan, err := planner.Plan(ctx)
	if err != nil {
		return err
	}
	plan.Consent = consent

	_, err = h.ClinicMergePlanExecutor.Execute(ctx, plan)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

// logMergeAudit writes one audit entry per merge report or merge request. Never add PHI here.
func (h *Handler) logMergeAudit(ctx context.Context, msg string, err error, args ...any) {
	if authData := auth.GetAuthData(ctx); authData != nil {
		args = append(args, "clinicianId", authData.SubjectId, "serverAccess", authData.ServerAccess)
	}
	if err != nil {
		args = append(args, "error", err)
	}
	h.Logger.Infow(msg, args...)
}

func (h *Handler) authorizeMergeSource(ctx context.Context, sourceId string) error {
	authData := auth.GetAuthData(ctx)
	if auth.IsServerAuth(authData) {
		return nil
	}
	if authData == nil || authData.SubjectId == "" {
		return errors.Unauthorized
	}

	clinician, err := h.Clinicians.Get(ctx, sourceId, authData.SubjectId)
	if err != nil && !stderrors.Is(err, errors.NotFound) {
		return err
	}
	if err != nil || clinician == nil || !clinician.IsAdmin() {
		return fmt.Errorf("source clinic admin access required: %w", errors.HttpError{
			Code: http.StatusForbidden,
			Err:  auth.ErrUnauthorized,
		})
	}
	return nil
}

func (h *Handler) CreateSite(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	site := &SiteV1{}
	if err := ec.Bind(site); err != nil {
		return errors.BadRequest
	}
	created, err := h.ClinicsManager.CreateSite(ctx, clinicId, site.Name)
	if err != nil {
		return err
	}
	return ec.JSON(http.StatusOK, created)
}

func (h *Handler) DeleteSite(ec echo.Context, clinicId ClinicId, siteId SiteId) error {
	ctx := ec.Request().Context()
	if err := h.ClinicsManager.DeleteSite(ctx, clinicId, siteId); err != nil {
		return err
	}
	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) UpdateSite(ec echo.Context, clinicId ClinicId, siteId SiteId) error {
	ctx := ec.Request().Context()
	site := &sites.Site{}
	if err := ec.Bind(site); err != nil {
		return errors.BadRequest
	}
	updated, err := h.ClinicsManager.UpdateSite(ctx, clinicId, siteId, site)
	if err != nil {
		return err
	}
	return ec.JSON(http.StatusOK, updated)
}

func (h *Handler) MergeSite(ec echo.Context, clinicId ClinicId, targetSiteId SiteId) error {
	ctx := ec.Request().Context()
	site := &SiteByIdV1{}
	if err := ec.Bind(site); err != nil {
		return errors.BadRequest
	}
	merged, err := h.ClinicsManager.MergeSite(ctx, clinicId, *site.Id, targetSiteId)
	if err != nil {
		return err
	}
	return ec.JSON(http.StatusOK, merged)
}

func (h *Handler) ConvertPatientTagToSite(ec echo.Context,
	clinicId ClinicId, patientTagId PatientTagId) error {

	ctx := ec.Request().Context()
	created, err := h.ClinicsManager.ConvertPatientTagToSite(ctx, clinicId, patientTagId)
	if err != nil {
		return err
	}
	return ec.JSON(http.StatusOK, created)
}
