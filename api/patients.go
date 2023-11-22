package api

import (
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var defaultPeriod = "14d"

func (h *Handler) ListPatients(ec echo.Context, clinicId ClinicId, params ListPatientsParams) (err error) {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := patients.Filter{
		ClinicId: strp(string(clinicId)),
		Search:   searchToString(params.Search),
		Tags:     params.Tags,
	}

	if params.Period == nil || *params.Period == "" {
		filter.Period = &defaultPeriod
		params.Period = &defaultPeriod
	} else {
		filter.Period = params.Period
	}

	var sorts []*store.Sort

	filter.CGM, err = ParseCGMSummaryFilters(params)
	if err != nil {
		return err
	}

	filter.BGM, err = ParseBGMSummaryFilters(params)
	if err != nil {
		return err
	}

	filter.CGMTime = ParseCGMSummaryDateFilters(params)
	filter.BGMTime = ParseBGMSummaryDateFilters(params)

	sorts, err = ParseSort(params.Sort, params.SortType, filter.Period, params.OffsetPeriods)
	if err != nil {
		return err
	}

	list, err := h.patients.List(ctx, &filter, page, sorts)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientsResponseDto(list))
}

func (h *Handler) CreatePatientAccount(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := Patient{}
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

	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return err
	}

	patient := NewPatient(dto)
	patient.ClinicId = &clinicObjId
	patient.Permissions = &patients.CustodialAccountPermissions

	if !authData.ServerAccess {
		patient.InvitedBy = &authData.SubjectId
	}

	result, err := h.patients.Create(ctx, patient)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(result))
}

func (h *Handler) GetPatient(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	patient, err := h.patients.Get(ctx, string(clinicId), string(patientId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) CreatePatientFromUser(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	dto := CreatePatient{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	clinicObjId, err := primitive.ObjectIDFromHex(string(clinicId))
	if err != nil {
		return err
	}

	patient := patients.Patient{
		UserId:      strp(string(patientId)),
		ClinicId:    &clinicObjId,
		Permissions: NewPermissions(dto.Permissions),
	}
	if dto.IsMigrated != nil {
		patient.IsMigrated = *dto.IsMigrated
	}
	if dto.LegacyClinicianId != nil {
		patient.LegacyClinicianIds = []string{string(*dto.LegacyClinicianId)}
	}

	if err = h.users.GetPatientFromExistingUser(ctx, &patient); err != nil {
		return err
	}
	patient.Email = pstrToLower(patient.Email)

	result, err := h.patients.Create(ctx, patient)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(result))
}

func (h *Handler) UpdatePatient(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	dto := Patient{}
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
	if authData.ServerAccess {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "expected user access token",
		}
	}

	update := patients.PatientUpdate{
		ClinicId:  string(clinicId),
		UserId:    string(patientId),
		Patient:   NewPatient(dto),
		UpdatedBy: authData.SubjectId,
	}
	patient, err := h.patients.Update(ctx, update)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) SendUploadReminder(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()

	authData := auth.GetAuthData(ctx)
	if authData == nil || authData.SubjectId == "" {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "expected authenticated user id",
		}
	}
	if authData.ServerAccess {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "expected user access token",
		}
	}

	update := patients.UploadReminderUpdate{
		ClinicId: string(clinicId),
		UserId:   string(patientId),
		Time:     time.Now(),
	}
	patient, err := h.patients.UpdateLastUploadReminderTime(ctx, &update)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) SendDexcomConnectRequest(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()

	authData := auth.GetAuthData(ctx)
	if authData == nil || authData.SubjectId == "" {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "expected authenticated user id",
		}
	}
	if authData.ServerAccess {
		return &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: "expected user access token",
		}
	}

	update := patients.LastRequestedDexcomConnectUpdate{
		ClinicId:  string(clinicId),
		Time:      time.Now(),
		UserId:    string(patientId),
		UpdatedBy: authData.SubjectId,
	}
	patient, err := h.patients.UpdateLastRequestedDexcomConnectTime(ctx, &update)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) UpdatePatientPermissions(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	dto := PatientPermissions{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	patient, err := h.patients.UpdatePermissions(ctx, string(clinicId), string(patientId), NewPermissions(&dto))
	if err != nil {
		return err
	}

	// patient was deleted after all permissions were revoked
	if patient == nil {
		return ec.NoContent(http.StatusNoContent)
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient).Permissions)
}

func (h *Handler) DeletePatientPermission(ec echo.Context, clinicId ClinicId, patientId PatientId, permission string) error {
	ctx := ec.Request().Context()
	_, err := h.patients.DeletePermission(ctx, string(clinicId), string(patientId), permission)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) ListClinicsForPatient(ec echo.Context, patientId UserId, params ListClinicsForPatientParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	list, err := h.patients.List(ctx, &patients.Filter{UserId: strp(string(patientId))}, page, nil)
	if err != nil {
		return err
	}

	var clinicIds []string
	for _, patient := range list.Patients {
		clinicIds = append(clinicIds, patient.ClinicId.Hex())
	}

	clinicList, err := h.clinics.List(ctx, &clinics.Filter{Ids: clinicIds}, store.Pagination{})
	dtos, err := NewPatientClinicRelationshipsDto(list.Patients, clinicList)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, dtos)
}

func (h *Handler) DeletePatient(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	err := h.patients.Remove(ctx, string(clinicId), string(patientId))
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) UpdatePatientSummary(ec echo.Context, patientId PatientId) error {
	ctx := ec.Request().Context()
	var dto *PatientSummary
	if ec.Request().ContentLength != 0 {
		dto = &PatientSummary{}
		if err := ec.Bind(dto); err != nil {
			return err
		}
	}

	err := h.patients.UpdateSummaryInAllClinics(ctx, patientId, NewSummary(dto))
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) TideReport(ec echo.Context, clinicId ClinicId, params TideReportParams) error {
	ctx := ec.Request().Context()
	tide, err := h.patients.TideReport(ctx, clinicId, patients.TideReportParams(params))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewTideDto(tide))
}

func (h *Handler) DeletePatientTagFromClinicPatients(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()

	dto := TidepoolUserIds{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	// We pass an empty request body as nil which will target all clinic patients for tag deletion
	if ec.Request().Body == http.NoBody {
		dto = nil
	}

	err := h.patients.DeletePatientTagFromClinicPatients(ctx, string(clinicId), string(patientTagId), dto)

	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) AssignPatientTagToClinicPatients(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()

	dto := TidepoolUserIds{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	err := h.patients.AssignPatientTagToClinicPatients(ctx, string(clinicId), string(patientTagId), dto)

	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) UpdatePatientDataSources(ec echo.Context, userId UserId) error {
	ctx := ec.Request().Context()
	dto := patients.DataSources{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	err := h.patients.UpdatePatientDataSources(ctx, string(userId), &dto)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) FindPatients(ec echo.Context, params FindPatientsParams) error {
	ctx := ec.Request().Context()
	authData := auth.GetAuthData(ctx)
	if authData == nil || authData.SubjectId == "" || authData.ServerAccess {
		return &echo.HTTPError{
			Code:    http.StatusUnauthorized,
			Message: "expected authenticated user id",
		}
	}

	maxClinics := store.DefaultPagination().WithLimit(1000)
	cliniciansFilter := &clinicians.Filter{
		UserId: &authData.SubjectId,
	}
	clinicianList, err := h.clinicians.List(ctx, cliniciansFilter, maxClinics)
	if err != nil {
		return err
	}

	clinicIds := make([]string, 0, len(clinicianList))
	for _, clinician := range clinicianList {
		if clinician != nil && clinician.ClinicId != nil {
			clinicIds = append(clinicIds, clinician.ClinicId.Hex())
		}
	}

	clinicList, err := h.clinics.List(ctx, &clinics.Filter{Ids: clinicIds}, maxClinics)
	if err != nil {
		return err
	}

	workspaceId := pstr(params.WorkspaceId)
	if workspaceId != "" {
		if params.WorkspaceIdType == nil {
			return fmt.Errorf("%w: workspace id type is required", errors.BadRequest)
		}

		workspaceIdType := string(*params.WorkspaceIdType)
		clinicList, err = clinics.FilterByWorkspaceId(clinicList, workspaceId, workspaceIdType)
		if err != nil {
			return err
		}
	}

	page := pagination(params.Offset, params.Limit)
	filter := patients.Filter{
		ClinicIds: clinicIds,
		Mrn:       params.Mrn,
		BirthDate: params.BirthDate,
	}
	list, err := h.patients.List(ctx, &filter, page, nil)
	if err != nil {
		return err
	}

	dtos, err := NewPatientClinicRelationshipsDto(list.Patients, clinicList)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, dtos)
}
