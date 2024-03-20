package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/errors"
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

	list, err := h.clinics.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicsDto(list))
}

func (h *Handler) CreateClinic(ec echo.Context) error {
	ctx := ec.Request().Context()
	dto := Clinic{}
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

	clinic := *NewClinic(dto)

	// Set new clinic migration status to true.
	// Only clinics created via `EnableNewClinicExperience` handler should be subject to initial clinician patient migration
	clinic.IsMigrated = true

	create := manager.CreateClinic{
		Clinic:            clinic,
		CreatorUserId:     authData.SubjectId,
		CreateDemoPatient: true,
	}

	result, err := h.clinicsManager.CreateClinic(ctx, &create)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) GetClinic(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	clinic, err := h.clinics.Get(ctx, string(clinicId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(clinic))
}

func (h *Handler) UpdateClinic(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := Clinic{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}
	result, err := h.clinics.Update(ctx, string(clinicId), NewClinic(dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) DeleteClinic(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	err := h.clinicsManager.DeleteClinic(ctx, string(clinicId))
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

	list, err := h.clinics.List(ctx, &filter, store.Pagination{Limit: 2})
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
	migration, err := h.clinicsMigrator.TriggerInitialMigration(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) ListMigrations(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	migrations, err := h.clinicsMigrator.ListMigrations(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDtos(migrations))
}

func (h *Handler) MigrateLegacyClinicianPatients(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	dto := Migration{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	migration, err := h.clinicsMigrator.MigrateLegacyClinicianPatients(ctx, clinicId, dto.UserId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) GetMigration(ec echo.Context, clinicId Id, userId UserId) error {
	ctx := ec.Request().Context()

	migration, err := h.clinicsMigrator.GetMigration(ctx, string(clinicId), string(userId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) UpdateMigration(ec echo.Context, clinicId Id, userId UserId) error {
	ctx := ec.Request().Context()
	dto := MigrationUpdate{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	migration, err := h.clinicsMigrator.UpdateMigrationStatus(ctx, string(clinicId), string(userId), string(dto.Status))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMigrationDto(migration))
}

func (h *Handler) DeleteUserFromClinics(ec echo.Context, userId UserId) error {
	ctx := ec.Request().Context()
	if _, err := h.patients.DeleteFromAllClinics(ctx, string(userId)); err != nil {
		return err
	}
	if err := h.clinicians.DeleteFromAllClinics(ctx, string(userId)); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) UpdateClinicUserDetails(ec echo.Context, userId UserId) error {
	ctx := ec.Request().Context()
	dto := UpdateUserDetails{}
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
		if err := h.clinicians.UpdateAll(ctx, update); err != nil {
			return err
		}
	}

	if err := h.patients.UpdateEmail(ctx, id, email); err != nil {
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

	if err := h.clinics.UpdateTier(ctx, string(clinicId), string(dto.Tier)); err != nil {
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

	if err := h.clinics.UpdateSuppressedNotifications(ctx, string(clinicId), clinics.SuppressedNotifications(dto.SuppressedNotifications)); err != nil {
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

	updated, err := h.clinics.CreatePatientTag(ctx, string(clinicId), dto.Name)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(updated).PatientTags)
}

func (h *Handler) UpdatePatientTag(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()
	dto := clinics.PatientTag{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	updated, err := h.clinics.UpdatePatientTag(ctx, string(clinicId), string(patientTagId), dto.Name)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(updated).PatientTags)
}

func (h *Handler) DeletePatientTag(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()

	updated, err := h.clinics.DeletePatientTag(ctx, string(clinicId), string(patientTagId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(updated).PatientTags)
}

func (h *Handler) ListMembershipRestrictions(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	updated, err := h.clinics.ListMembershipRestrictions(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewMembershipRestrictionsDto(updated))
}

func (h *Handler) UpdateMembershipRestrictions(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := MembershipRestrictions{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	if err := h.clinics.UpdateMembershipRestrictions(ctx, clinicId, NewMembershipRestrictions(dto)); err != nil {
		return err
	}

	return h.ListMembershipRestrictions(ec, clinicId)
}

func (h *Handler) GetEHRSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	settings, err := h.clinics.GetEHRSettings(ctx, clinicId)
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
	dto := EHRSettings{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	settings := NewEHRSettings(dto)
	err := h.clinics.UpdateEHRSettings(ctx, clinicId, settings)
	if err != nil {
		return err
	}

	return h.GetEHRSettings(ec, clinicId)
}

func (h *Handler) GetMRNSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	settings, err := h.clinics.GetMRNSettings(ctx, clinicId)
	if err != nil {
		return err
	}

	if settings == nil {
		return errors.NotFound
	}

	return ec.JSON(http.StatusOK, MRNSettings{
		Required: settings.Required,
		Unique:   settings.Unique,
	})
}

func (h *Handler) UpdateMRNSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := MRNSettings{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	err := h.clinics.UpdateMRNSettings(ctx, clinicId, &clinics.MRNSettings{
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

	patientCountSettings, err := h.clinics.GetPatientCountSettings(ctx, clinicId)
	if err != nil {
		return err
	} else if patientCountSettings == nil {
		return errors.NotFound
	}

	return ec.JSON(http.StatusOK, NewPatientCountSettingsDto(patientCountSettings))
}

func (h *Handler) UpdatePatientCountSettings(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := PatientCountSettings{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	patientCountSettings := NewPatientCountSettings(dto)
	if patientCountSettings != nil && !patientCountSettings.IsValid() {
		return errors.BadRequest
	}

	if err := h.clinics.UpdatePatientCountSettings(ctx, clinicId, patientCountSettings); err != nil {
		return err
	}

	return h.GetPatientCountSettings(ec, clinicId)
}

func (h *Handler) GetPatientCount(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()

	patientCount, err := h.clinicsManager.GetClinicPatientCount(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, PatientCount{
		PatientCount: patientCount.PatientCount,
	})
}
