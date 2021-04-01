package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/clinicians"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
)

func (h *Handler) ListClinicians(ec echo.Context, clinicId string, params ListCliniciansParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := clinicians.Filter{
		ClinicId: clinicId,
		Search:   params.Search,
		Email:    params.Email,
	}

	list, err := h.clinicians.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewCliniciansDto(list))
}

func (h *Handler) DeleteClinician(ec echo.Context, clinicId string, clinicianId string) error {
	ctx := ec.Request().Context()
	err := h.clinicians.Delete(ctx, clinicId, clinicianId)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) GetClinician(ec echo.Context, clinicId string, clinicianId string) error {
	ctx := ec.Request().Context()
	clinician, err := h.clinicians.Get(ctx, clinicId, clinicianId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(clinician))
}

func (h *Handler) CreateClinician(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	dto := Clinician{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return ec.JSON(http.StatusBadRequest, "invalid clinic id")
	}

	clinician := NewClinician(dto)
	clinician.ClinicId = &clinicObjId
	result, err := h.clinicians.Create(ctx, clinician)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(result))
}

func (h *Handler) UpdateClinician(ec echo.Context, clinicId string, clinicianId string) error {
	ctx := ec.Request().Context()
	dto := Clinician{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	result, err := h.clinicians.Update(ctx, clinicId, clinicianId, NewClinicianUpdate(dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(result))
}


func (h *Handler) GetInvitedClinician(ec echo.Context, clinicId string, inviteId string) error {
	ctx := ec.Request().Context()
	clinician, err := h.clinicians.GetInvite(ctx, clinicId, inviteId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(clinician))
}

func (h *Handler) DeleteInvitedClinician(ec echo.Context, clinicId string, inviteId string) error {
	ctx := ec.Request().Context()
	if err := h.clinicians.DeleteInvite(ctx, clinicId, inviteId); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) AssociateClinicianToUser(ec echo.Context, clinicId string, inviteId string) error {
	ctx := ec.Request().Context()
	dto := AssociateClinicianToUser{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	clinician, err := h.clinicians.AssociateInvite(ctx, clinicId, inviteId, dto.UserId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(clinician))
}
