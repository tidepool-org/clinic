package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
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
	if params.SummaryPercentTimeInVeryLow != nil && *params.SummaryPercentTimeInVeryLow != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPercentTimeInVeryLow)
		if err != nil {
			return err
		}
		filter.PercentTimeInVeryLowCmp = cmp
		filter.PercentTimeInVeryLowValue = value
	}
	if params.SummaryPercentTimeInLow != nil && *params.SummaryPercentTimeInLow != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPercentTimeInLow)
		if err != nil {
			return err
		}
		filter.PercentTimeInLowCmp = cmp
		filter.PercentTimeInLowValue = value
	}
	if params.SummaryPercentTimeInTarget != nil && *params.SummaryPercentTimeInTarget != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPercentTimeInTarget)
		if err != nil {
			return err
		}
		filter.PercentTimeInTargetCmp = cmp
		filter.PercentTimeInTargetValue = value
	}
	if params.SummaryPercentTimeInHigh != nil && *params.SummaryPercentTimeInHigh != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPercentTimeInHigh)
		if err != nil {
			return err
		}
		filter.PercentTimeInHighCmp = cmp
		filter.PercentTimeInHighValue = value
	}
	if params.SummaryPercentTimeInVeryHigh != nil && *params.SummaryPercentTimeInVeryHigh != "" {
		cmp, value, err := parseRangeFilter(*params.SummaryPercentTimeInVeryHigh)
		if err != nil {
			return err
		}
		filter.PercentTimeInVeryHighCmp = cmp
		filter.PercentTimeInVeryHighValue = value
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
