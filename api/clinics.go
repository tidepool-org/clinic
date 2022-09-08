package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/creator"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
)

func (h *Handler) ListClinics(ec echo.Context, params ListClinicsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)

	filter := clinics.Filter{}
	if params.ShareCode != nil {
		filter.ShareCodes = []string{string(*params.ShareCode)}
	}
	if params.CreatedTimeStart != nil {
		start := time.Time(*params.CreatedTimeStart)
		if !start.IsZero() {
			filter.CreatedTimeStart = &start
		}
	}
	if params.CreatedTimeEnd != nil {
		end := time.Time(*params.CreatedTimeEnd)
		if !end.IsZero() {
			filter.CreatedTimeEnd = &end
		}
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

	create := creator.CreateClinic{
		Clinic:            *NewClinic(dto),
		CreatorUserId:     authData.SubjectId,
		CreateDemoPatient: true,
	}

	result, err := h.clinicsCreator.CreateClinic(ctx, &create)
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
	if err := h.patients.DeleteFromAllClinics(ctx, string(userId)); err != nil {
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

func (h *Handler) CreatePatientTag(ec echo.Context, clinicId ClinicId) error {
	return ec.NoContent(http.StatusOK)
}

func (h *Handler) DeletePatientTag(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	return ec.NoContent(http.StatusOK)
}

func (h *Handler) ListPatientTags(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	fmt.Println("clinicId", clinicId)
	clinic, err := h.clinics.Get(ctx, string(clinicId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(clinic).PatientTags)
}

func (h *Handler) UpdatePatientTag(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	return ec.NoContent(http.StatusOK)
}
