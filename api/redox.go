package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/redox"
	"github.com/tidepool-org/clinic/redox/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"net/http"
)

func (h *Handler) VerifyEndpoint(ec echo.Context) error {
	request := redox.VerificationRequest{}
	if err := ec.Bind(&request); err != nil {
		return err
	}
	result, err := h.redox.VerifyEndpoint(request)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, result)
}

func (h *Handler) ProcessEHRMessage(ec echo.Context) error {
	ctx := ec.Request().Context()

	// Make sure the request is initiated by redox
	if err := h.redox.AuthorizeRequest(ec.Request()); err != nil {
		return err
	}

	// Capture raw json for later processing
	raw, err := io.ReadAll(ec.Request().Body)
	if err != nil {
		return err
	}

	return h.redox.ProcessEHRMessage(ctx, raw)
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

	// We only support new order messages for now
	if request.MessageRef.DataModel != Order || request.MessageRef.EventType != New {
		return fmt.Errorf("%w: only new order messages are supported", errors.BadRequest)
	}
	msg, err := h.redox.FindMessage(
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

	criteria, err := redox.GetClinicMatchingCriteriaFromNewOrder(order)
	if err != nil {
		return err
	}
	clinic, err := h.redox.FindMatchingClinic(ctx, criteria)
	if err != nil {
		return err
	}

	update := redox.GetUpdateFromNewOrder(*clinic, documentId, *order)
	matchedPatients, err := h.redox.MatchNewOrderToPatient(ctx, *clinic, *order, update)
	if err != nil {
		return err
	}

	response := EHRMatchResponse{
		Clinic: NewClinicDto(clinic),
		Settings: EHRSettings{
			Enabled:  clinic.EHRSettings.Enabled,
			SourceId: clinic.EHRSettings.SourceId,
			DestinationIds: EHRDestinationIds{
				Default:   clinic.EHRSettings.DestinationIds.Default,
				Flowsheet: clinic.EHRSettings.DestinationIds.Flowsheet,
				Notes:     clinic.EHRSettings.DestinationIds.Notes,
				Results:   clinic.EHRSettings.DestinationIds.Results,
			},
			ProcedureCodes: EHRProcedureCodes{
				EnableSummaryReports:  clinic.EHRSettings.ProcedureCodes.EnableSummaryReports,
				DisableSummaryReports: clinic.EHRSettings.ProcedureCodes.DisableSummaryReports,
			},
		},
	}
	if clinic.EHRSettings.Facility != nil && clinic.EHRSettings.Facility.Name != "" {
		response.Settings.Facility = &EHRFacility{
			Name: clinic.EHRSettings.Facility.Name,
		}
	}
	if matchedPatients != nil {
		dto := NewPatientsDto(matchedPatients)
		response.Patients = &dto
	}
	if update != nil {
		action := &EHRMatchAction{}
		if update.Name == patients.SummaryAndReportsSubscription && update.Active {
			action.ActionType = ENABLESUMARYANDREPORTSSUBSCRIPTION
		} else if update.Name == patients.SummaryAndReportsSubscription && !update.Active {
			action.ActionType = DISABLESUMARYANDREPORTSSUBSCRIPTION
		}
		if action.ActionType != "" {
			response.Action = action
		}
	}

	return ec.JSON(http.StatusOK, response)
}
