package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"net/http"
)

type Handler struct {
	clinics    clinics.Service
	clinicians clinicians.Service
	patients   patients.Service
}

func NewHandler(clinics clinics.Service, clinicians clinicians.Service, patients patients.Service) *Handler {
	return &Handler{
		clinics:    clinics,
		clinicians: clinicians,
		patients:   patients,
	}
}

func pagination(offset, limit *int) store.Pagination {
	page := store.DefaultPagination()
	if offset != nil {
		page.Offset = *offset
	}
	if limit != nil {
		page.Limit = *limit
	}
	return page
}

func (h *Handler) ListClinics(ctx echo.Context, params ListClinicsParams) error {
	c := ctx.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := clinics.Filter{}

	list, err := h.clinics.List(c, &filter, page)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, NewClinicsDto(list))
}

func (h *Handler) CreateClinic(ctx echo.Context) error {
	c := ctx.Request().Context()
	dto := Clinic{}
	if err := ctx.Bind(&dto); err != nil {
		return err
	}
	result, err := h.clinics.Create(c, NewClinic(dto))
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, NewClinicDto(result))
}


func (h *Handler) GetClinic(ctx echo.Context, clinicId string) error {
	c := ctx.Request().Context()
	clinic, err := h.clinics.Get(c, clinicId)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, NewClinicDto(clinic))
}

func (h *Handler) UpdateClinic(ctx echo.Context, clinicId string) error {
	c := ctx.Request().Context()
	dto := Attributes{}
	if err := ctx.Bind(&dto); err != nil {
		return err
	}
	result, err := h.clinics.Update(c, clinicId, NewClinicUpdate(dto))
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) ListClinicians(ctx echo.Context, clinicId string, params ListCliniciansParams) error {
	c := ctx.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := clinicians.Filter{
		ClinicId: clinicId,
		Search: params.Search,
	}

	list, err := h.clinicians.List(c, &filter, page)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, NewClinicsDto(list))
}

func (h *Handler) DeleteClinician(ctx echo.Context, clinicId string, clinicianId string) error {
	panic("implement me")
}

func (h *Handler) GetClinician(ctx echo.Context, clinicId string, clinicianId string) error {
	panic("implement me")
}

func (h *Handler) UpdateClinician(ctx echo.Context, clinicId string, clinicianId string) error {
	panic("implement me")
}

func (h *Handler) InviteClinician(ctx echo.Context, clinicId string) error {
	panic("implement me")
}

func (h *Handler) DeleteInvite(ctx echo.Context, clinicId string, inviteId string) error {
	panic("implement me")
}

func (h *Handler) ResendInvite(ctx echo.Context, clinicId string, inviteId string) error {
	panic("implement me")
}

func (h *Handler) AcceptInvite(ctx echo.Context, clinicId string, inviteId string) error {
	panic("implement me")
}

func (h *Handler) ListPatients(ctx echo.Context, clinicId string, params ListPatientsParams) error {
	panic("implement me")
}

func (h *Handler) CreatePatientAccount(ctx echo.Context, clinicId string) error {
	panic("implement me")
}

func (h *Handler) GetPatient(ctx echo.Context, clinicId string, patientId string) error {
	panic("implement me")
}

func (h *Handler) CreatePatientFromUser(ctx echo.Context, clinicId string, patientId string) error {
	panic("implement me")
}

var _ ServerInterface = &Handler{}
