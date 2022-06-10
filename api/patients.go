package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

func (h *Handler) ListPatients(ec echo.Context, clinicId ClinicId, params ListPatientsParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	filter := patients.Filter{
		ClinicId:           strp(string(clinicId)),
		Search:             searchToString(params.Search),
		LastUploadDateFrom: params.SummaryLastUploadDateFrom,
		LastUploadDateTo:   params.SummaryLastUploadDateTo,
	}
	if params.SummaryPeriods14dTimeCGMUsePercent != nil && *params.SummaryPeriods14dTimeCGMUsePercent != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPeriods14dTimeCGMUsePercent)
		if err != nil {
			return err
		}
		filter.TimeCGMUsePercentCmp14d = cmp
		filter.TimeCGMUsePercentValue14d = value
	}
	if params.SummaryPeriods14dTimeInVeryLowPercent != nil && *params.SummaryPeriods14dTimeInVeryLowPercent != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPeriods14dTimeInVeryLowPercent)
		if err != nil {
			return err
		}
		filter.TimeInVeryLowPercentCmp14d = cmp
		filter.TimeInVeryLowPercentValue14d = value
	}
	if params.SummaryPeriods14dTimeInLowPercent != nil && *params.SummaryPeriods14dTimeInLowPercent != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPeriods14dTimeInLowPercent)
		if err != nil {
			return err
		}
		filter.TimeInLowPercentCmp14d = cmp
		filter.TimeInLowPercentValue14d = value
	}
	if params.SummaryPeriods14dTimeInTargetPercent != nil && *params.SummaryPeriods14dTimeInTargetPercent != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPeriods14dTimeInTargetPercent)
		if err != nil {
			return err
		}
		filter.TimeInTargetPercentCmp14d = cmp
		filter.TimeInTargetPercentValue14d = value
	}
	if params.SummaryPeriods14dTimeInHighPercent != nil && *params.SummaryPeriods14dTimeInHighPercent != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPeriods14dTimeInHighPercent)
		if err != nil {
			return err
		}
		filter.TimeInHighPercentCmp14d = cmp
		filter.TimeInHighPercentValue14d = value
	}
	if params.SummaryPeriods14dTimeInVeryHighPercent != nil && *params.SummaryPeriods14dTimeInVeryHighPercent != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPeriods14dTimeInVeryHighPercent)
		if err != nil {
			return err
		}
		filter.TimeInVeryHighPercentCmp14d = cmp
		filter.TimeInVeryHighPercentValue14d = value
	}

	sort, err := ParseSort(params.Sort)
	if err != nil {
		return err
	}

	list, err := h.patients.List(ctx, &filter, page, sort)
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

	clinicObjId, err := primitive.ObjectIDFromHex(string(clinicId))
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

	err := h.patients.UpdateSummaryInAllClinics(ctx, string(patientId), NewSummary(dto))
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}
