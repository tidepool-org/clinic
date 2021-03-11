package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx"
	"net/http"
)

type Handler struct {
	clinics    clinics.Service
	clinicians clinicians.Service
	patients   patients.Service
	users      users.Service
}

var _ ServerInterface = &Handler{}

type Params struct {
	fx.In

	Clinics    clinics.Service
	Clinicians clinicians.Service
	Patients   patients.Service
	Users      users.Service
}

func NewHandler(p Params) *Handler {
	return &Handler{
		clinics:    p.Clinics,
		clinicians: p.Clinicians,
		patients:   p.Patients,
		users:      p.Users,
	}
}

func (h *Handler) ListClinics(ec echo.Context, params ListClinicsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := clinics.Filter{
		Email: params.Email,
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
	result, err := h.clinics.Create(ctx, NewClinic(dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) GetClinic(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	clinic, err := h.clinics.Get(ctx, clinicId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(clinic))
}

func (h *Handler) UpdateClinic(ec echo.Context, clinicId string) error {
	ctx := ec.Request().Context()
	dto := Clinic{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}
	result, err := h.clinics.Update(ctx, clinicId, NewClinic(dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewClinicDto(result))
}

func (h *Handler) ListClinicians(ec echo.Context, clinicId string, params ListCliniciansParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := clinicians.Filter{
		ClinicId: clinicId,
		Search:   params.Search,
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

func (h *Handler) ListPatients(ec echo.Context, clinicId string, params ListPatientsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := patients.Filter{
		ClinicId: &clinicId,
		Search:   params.Search,
	}

	list, err := h.patients.List(ctx, &filter, page)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientsDto(list))
}

func (h *Handler) CreatePatientAccount(ec echo.Context, clinicId string) error {
	panic("implement me")
}

func (h *Handler) GetPatient(ec echo.Context, clinicId string, patientId string) error {
	ctx := ec.Request().Context()
	patient, err := h.patients.Get(ctx, clinicId, patientId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) CreatePatientFromUser(ec echo.Context, clinicId string, patientId string) error {
	ctx := ec.Request().Context()
	dto := &CreatePatient{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return err
	}

	patient := patients.Patient{
		ClinicId:    &clinicObjId,
		UserId:      &patientId,
		Permissions: NewPermissions(dto.Permissions),
	}

	result, err := h.users.CreatePatientFromExistingUser(ctx, patient)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(result))
}

func (h *Handler) UpdatePatient(ec echo.Context, clinicId string, patientId string) error {
	ctx := ec.Request().Context()
	dto := Patient{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	patient, err := h.patients.Update(ctx, clinicId, patientId, NewPatientUpdate(dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) UpdatePatientPermissions(ec echo.Context, clinicId string, patientId string) error {
	ctx := ec.Request().Context()
	dto := PatientPermissions{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	patient, err := h.patients.UpdatePermissions(ctx, clinicId, patientId, NewPermissions(&dto))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient).Permissions)
}

func (h *Handler) GetPatientClinicRelationships(ec echo.Context, patientId string, params GetPatientClinicRelationshipsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	patientList, err := h.patients.List(ctx, &patients.Filter{UserId: &patientId}, page)
	if err != nil {
		return err
	}

	var clinicIds []string
	for _, patient := range patientList {
		clinicIds = append(clinicIds, patient.ClinicId.Hex())
	}

	clinicList, err := h.clinics.List(ctx, &clinics.Filter{Ids: clinicIds}, store.Pagination{})
	dtos, err := NewPatientClinicRelationshipsDto(patientList, clinicList)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, dtos)
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
