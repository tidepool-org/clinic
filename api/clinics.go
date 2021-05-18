package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/authz"
	"github.com/tidepool-org/clinic/clinics"
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

	create := clinics.CreateClinic{
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
