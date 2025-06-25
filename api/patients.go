package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/errors"

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
		ClinicId:     strp(string(clinicId)),
		Search:       searchToString(params.Search),
		Tags:         params.Tags,
		LastReviewed: params.LastReviewed,
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

	list, err := h.Patients.List(ctx, &filter, page, sorts)
	if err != nil {
		return err
	}

	clinicPatientsCount, err := h.Patients.Count(ctx, &patients.Filter{
		ClinicId:     strp(clinicId),
	})
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientsResponseDto(list, clinicPatientsCount))
}

func (h *Handler) CreatePatientAccount(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	dto := PatientV1{}
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

	result, err := h.Patients.Create(ctx, patient)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(result))
}

func (h *Handler) GetPatient(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	patient, err := h.Patients.Get(ctx, string(clinicId), string(patientId))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) CreatePatientFromUser(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	dto := CreatePatientV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return err
	}

	patient := NewPatientFromCreate(dto)
	patient.UserId = strp(patientId)
	patient.ClinicId = &clinicObjId

	if err = h.Users.PopulatePatientDetailsFromExistingUser(ctx, &patient); err != nil {
		return err
	}
	patient.Email = pstrToLower(patient.Email)

	result, err := h.Patients.Create(ctx, patient)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(result))
}

func (h *Handler) UpdatePatient(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	dto := PatientV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	update := patients.PatientUpdate{
		ClinicId: clinicId,
		UserId:   patientId,
		Patient:  NewPatient(dto),
	}

	patient, err := h.Patients.Update(ctx, update)
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
	patient, err := h.Patients.UpdateLastUploadReminderTime(ctx, &update)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewPatientDto(patient))
}

func (h *Handler) UpdatePatientPermissions(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()
	dto := PatientPermissionsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	patient, err := h.Patients.UpdatePermissions(ctx, string(clinicId), string(patientId), NewPermissions(&dto))
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
	_, err := h.Patients.DeletePermission(ctx, string(clinicId), string(patientId), permission)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) ListClinicsForPatient(ec echo.Context, patientId UserId, params ListClinicsForPatientParams) error {
	ctx := ec.Request().Context()
	page := pagination(params.Offset, params.Limit)
	list, err := h.Patients.List(ctx, &patients.Filter{UserId: strp(string(patientId))}, page, nil)
	if err != nil {
		return err
	}

	var clinicIds []string
	for _, patient := range list.Patients {
		clinicIds = append(clinicIds, patient.ClinicId.Hex())
	}

	clinicList, err := h.Clinics.List(ctx, &clinics.Filter{Ids: clinicIds}, store.Pagination{})
	dtos, err := NewPatientClinicRelationshipsDto(list.Patients, clinicList)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, dtos)
}

func (h *Handler) DeletePatient(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()

	var deletedByUserId *string
	authData := auth.GetAuthData(ctx)
	if authData != nil && authData.ServerAccess == false {
		deletedByUserId = &authData.SubjectId
	}

	err := h.Patients.Remove(ctx, clinicId, patientId, deletedByUserId)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}

func (h *Handler) UpdatePatientSummary(ec echo.Context, patientId PatientId) error {
	ctx := ec.Request().Context()
	var dto *PatientSummaryV1
	if ec.Request().ContentLength != 0 {
		dto = &PatientSummaryV1{}
		if err := ec.Bind(dto); err != nil {
			return err
		}
	}

	err := h.Patients.UpdateSummaryInAllClinics(ctx, patientId, NewSummary(dto))
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) DeletePatientSummary(ec echo.Context, summaryId SummaryId) error {
	ctx := ec.Request().Context()
	err := h.Patients.DeleteSummaryInAllClinics(ctx, summaryId)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) TideReport(ec echo.Context, clinicId ClinicId, params TideReportParams) error {
	ctx := ec.Request().Context()
	tide, err := h.Patients.TideReport(ctx, clinicId, patients.TideReportParams(params))
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewTideDto(tide))
}

func (h *Handler) DeletePatientTagFromClinicPatients(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()

	dto := TidepoolUserIdsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	// We pass an empty request body as nil which will target all clinic patients for tag deletion
	if ec.Request().Body == http.NoBody {
		dto = nil
	}

	err := h.Patients.DeletePatientTagFromClinicPatients(ctx, string(clinicId), string(patientTagId), dto)

	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusOK)
}

func (h *Handler) AssignPatientTagToClinicPatients(ec echo.Context, clinicId ClinicId, patientTagId PatientTagId) error {
	ctx := ec.Request().Context()

	dto := TidepoolUserIdsV1{}
	if err := ec.Bind(&dto); err != nil {
		return err
	}

	// We pass an empty request body as nil which will target all clinic patients for tag assignment
	if ec.Request().Body == http.NoBody {
		dto = nil
	}

	err := h.Patients.AssignPatientTagToClinicPatients(ctx, clinicId, patientTagId, dto)

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

	err := h.Patients.UpdatePatientDataSources(ctx, string(userId), &dto)
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
	clinicianList, err := h.Clinicians.List(ctx, cliniciansFilter, maxClinics)
	if err != nil {
		return err
	}

	clinicIds := make([]string, 0, len(clinicianList))
	for _, clinician := range clinicianList {
		if clinician != nil && clinician.ClinicId != nil {
			clinicIds = append(clinicIds, clinician.ClinicId.Hex())
		}
	}

	clinicList, err := h.Clinics.List(ctx, &clinics.Filter{Ids: clinicIds}, maxClinics)
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

		clinicIds = make([]string, 0, len(clinicList))
		for _, clinic := range clinicList {
			clinicIds = append(clinicIds, clinic.Id.Hex())
		}
	}

	page := pagination(params.Offset, params.Limit)
	filter := patients.Filter{
		ClinicIds: clinicIds,
		Mrn:       params.Mrn,
		BirthDate: params.BirthDate,
	}
	list, err := h.Patients.List(ctx, &filter, page, nil)
	if err != nil {
		return err
	}

	dtos, err := NewPatientClinicRelationshipsDto(list.Patients, clinicList)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, dtos)
}

func (h *Handler) UpdatePatientReviews(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
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

	clinicianId := authData.SubjectId

	review := patients.Review{
		ClinicianId: clinicianId,
		Time:        time.Now().UTC().Truncate(time.Millisecond),
	}

	reviews, err := h.Patients.AddReview(ctx, clinicId, patientId, review)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewReviewsDto(reviews))
}

func (h *Handler) DeletePatientReviews(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
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

	clinicianId := authData.SubjectId

	reviews, err := h.Patients.DeleteReview(ctx, clinicId, clinicianId, patientId)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, NewReviewsDto(reviews))
}

func (h *Handler) ConnectProvider(ec echo.Context, clinicId ClinicId, patientId PatientId, providerId ProviderId) error {
	ctx := ec.Request().Context()

	authData := auth.GetAuthData(ctx)
	if err := authData.AssertAuthenticatedUser(); err != nil {
		return err
	}

	provider, err := NewDataProvider(providerId)
	if err != nil {
		return err
	}

	request := patients.ConnectionRequest{
		ProviderName: provider,
		CreatedTime:  time.Now(),
	}

	err = h.Patients.AddProviderConnectionRequest(ctx, clinicId, patientId, request)
	if err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}
