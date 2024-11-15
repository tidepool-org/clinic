package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/redox"
	models "github.com/tidepool-org/clinic/redox_models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"net/http"
)

func (h *Handler) VerifyEndpoint(ec echo.Context) error {
	request := redox.VerificationRequest{}
	if err := ec.Bind(&request); err != nil {
		return err
	}
	result, err := h.Redox.VerifyEndpoint(request)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, result)
}

func (h *Handler) ProcessEHRMessage(ec echo.Context) error {
	ctx := ec.Request().Context()

	// Make sure the request is initiated by redox
	if err := h.Redox.AuthorizeRequest(ec.Request()); err != nil {
		return err
	}

	// Capture raw json for later processing
	raw, err := io.ReadAll(ec.Request().Body)
	if err != nil {
		return err
	}

	return h.Redox.ProcessEHRMessage(ctx, raw)
}

func (h *Handler) MatchClinicAndPatient(ec echo.Context) error {
	ctx := ec.Request().Context()

	request := EHRMatchRequest{}
	if err := ec.Bind(&request); err != nil {
		return err
	}

	if request.MessageRef == nil {
		return fmt.Errorf("%w: messageRef is required", errors.BadRequest)
	}
	documentId, err := primitive.ObjectIDFromHex(request.MessageRef.DocumentId)
	if err != nil {
		return fmt.Errorf("%w: invalid documentId", errors.BadRequest)
	}
	criteria, err := NewMatchOrderCriteria(request.Criteria)
	if err != nil {
		return fmt.Errorf("%w: invalid criteria", errors.BadRequest)
	}

	// We only support new order messages for now
	if request.MessageRef.DataModel != Order || request.MessageRef.EventType != EHRMatchMessageRefEventTypeNew {
		return fmt.Errorf("%w: only new order messages are supported", errors.BadRequest)
	}
	msg, err := h.Redox.FindMessage(
		ctx,
		request.MessageRef.DocumentId,
		string(request.MessageRef.DataModel),
		string(request.MessageRef.EventType),
	)
	if err != nil {
		return err
	}

	order, err := redox.UnmarshallMessage[*models.NewOrder](*msg)
	if err != nil {
		return err
	}

	result, err := h.Redox.MatchNewOrderToPatient(ctx, redox.MatchOrder{
		DocumentId:        documentId,
		Order:             *order,
		PatientAttributes: criteria,
	})

	if err != nil {
		return err
	}

	response := EHRMatchResponse{
		Clinic:   NewClinicDto(&result.Clinic),
		Settings: *NewEHRSettingsDto(result.Clinic.EHRSettings),
	}

	if result.Patients != nil {
		dto := NewPatientsDto(result.Patients)
		response.Patients = &dto
	}

	return ec.JSON(http.StatusOK, response)
}

func (h *Handler) SyncEHRData(ec echo.Context, clinicId ClinicId) error {
	ctx := ec.Request().Context()
	if err := h.Redox.RescheduleSubscriptionOrders(ctx, clinicId); err != nil {
		return err
	}

	return ec.NoContent(http.StatusAccepted)
}

func (h *Handler) SyncEHRDataForPatient(ec echo.Context, patientId PatientId) error {
	ctx := ec.Request().Context()
	if err := h.Redox.RescheduleSubscriptionOrdersForPatient(ctx, patientId); err != nil {
		return err
	}

	return ec.NoContent(http.StatusAccepted)
}
