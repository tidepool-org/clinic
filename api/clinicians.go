package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

func (h *Handler) ListAllClinicians(ec echo.Context, params ListAllCliniciansParams) error {
	page := pagination(params.Offset, params.Limit)
	filter := clinicians.Filter{}
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
	return h.listClinics(ec, filter, page)
}

func (h *Handler) ListClinicians(ec echo.Context, clinicId ClinicId, params ListCliniciansParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := clinicians.Filter{
		ClinicId: strp(string(clinicId)),
		Search:   searchToString(params.Search),
		Email:    pstrToLower(emailToString(params.Email)),
		Role:     roleToString(params.Role),
	}

	list, err := h.clinicians.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewCliniciansDto(list))
}

func (h *Handler) DeleteClinician(ec echo.Context, clinicId ClinicId, clinicianId ClinicianId) error {
	ctx := ec.Request().Context()
	err := h.clinicians.Delete(ctx, string(clinicId), string(clinicianId))
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) GetClinician(ec echo.Context, clinicId ClinicId, clinicianId ClinicianId) error {
	ctx := ec.Request().Context()
	clinician, err := h.clinicians.Get(ctx, string(clinicId), string(clinicianId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(clinician))
}

func (h *Handler) CreateClinician(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := Clinician{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	clinicObjId, err := primitive.ObjectIDFromHex(string(clinicId))
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

func (h *Handler) UpdateClinician(ec echo.Context, clinicId ClinicId, clinicianId ClinicianId) error {
	ctx := ec.Request().Context()
	dto := Clinician{}
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

	update := &clinicians.ClinicianUpdate{
		UpdatedBy:   authData.SubjectId,
		ClinicId:    string(clinicId),
		ClinicianId: string(clinicianId),
		Clinician:   NewClinicianUpdate(dto),
	}

	result, err := h.cliniciansUpdater.Update(ctx, update)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(result))
}

func (h *Handler) ListClinicsForClinician(ec echo.Context, userId UserId, params ListClinicsForClinicianParams) error {
	page := pagination(params.Offset, params.Limit)
	filter := clinicians.Filter{
		UserId: strp(string(userId)),
	}

	return h.listClinics(ec, filter, page)
}

func (h *Handler) GetInvitedClinician(ec echo.Context, clinicId ClinicId, inviteId InviteId) error {
	ctx := ec.Request().Context()
	clinician, err := h.clinicians.GetInvite(ctx, string(clinicId), string(inviteId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(clinician))
}

func (h *Handler) DeleteInvitedClinician(ec echo.Context, clinicId ClinicId, inviteId InviteId) error {
	ctx := ec.Request().Context()
	if err := h.clinicians.DeleteInvite(ctx, string(clinicId), string(inviteId)); err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) AssociateClinicianToUser(ec echo.Context, clinicId ClinicId, inviteId InviteId) error {
	ctx := ec.Request().Context()
	dto := AssociateClinicianToUser{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	associate := clinicians.AssociateInvite{
		ClinicId: string(clinicId),
		InviteId: string(inviteId),
		UserId:   dto.UserId,
	}
	clinician, err := h.cliniciansUpdater.AssociateInvite(ctx, associate)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicianDto(clinician))
}

func (h *Handler) EnableNewClinicExperience(ec echo.Context, userId string) error {
	ctx := ec.Request().Context()
	clinic, err := h.clinicsMigrator.CreateEmptyClinic(ctx, userId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(clinic))
}

func (h *Handler) listClinics(ec echo.Context, filter clinicians.Filter, page store.Pagination) error {
	ctx := ec.Request().Context()
	cliniciansList, err := h.clinicians.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	var clinicIds []string
	for _, clinician := range cliniciansList {
		clinicIds = append(clinicIds, clinician.ClinicId.Hex())
	}

	clinicList, err := h.clinics.List(ctx, &clinics.Filter{Ids: clinicIds}, store.Pagination{})
	dtos, err := NewClinicianClinicRelationshipsDto(cliniciansList, clinicList)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, dtos)
}
