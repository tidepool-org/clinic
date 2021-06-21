package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/authz"
	"github.com/tidepool-org/clinic/cliniccreator"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"net/http"
)

func (h *Handler) ListClinics(ec echo.Context, params ListClinicsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)

	filter := clinics.Filter{}
	if params.ShareCode != nil {
		filter.ShareCodes = []string{string(*params.ShareCode)}
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

	userId := authz.GetAuthUserId(ec.Request())
	if userId == nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "expected authenticated user id",
		}
	}

	create := cliniccreator.CreateClinic{
		Clinic:        *NewClinic(dto),
		CreatorUserId: *userId,
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
