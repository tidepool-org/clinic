package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/redox"
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

	clinic, err := h.redox.FindMatchingClinic(ctx, redox.ClinicMatchingCriteria{
		SourceId:     request.Clinic.SourceId,
		FacilityName: request.Clinic.FacilityName,
	})
	if err != nil {
		return err
	}

	response := EHRMatchResponse{
		Clinic:   NewClinicDto(clinic),
		SourceId: clinic.EHRSettings.SourceId,
	}
	if clinic.EHRSettings.Facility != nil && clinic.EHRSettings.Facility.Name != "" {
		response.Facility = EHRFacility{
			Name: clinic.EHRSettings.Facility.Name,
		}
	}

	if request.Patient != nil {
		patients, err := h.redox.MatchPatient(ctx, redox.PatientMatchingCriteria{
			Mrn:         request.Patient.Mrn,
			DateOfBirth: request.Patient.DateOfBirth,
		})
		if err != nil {
			return err
		}

		dto := NewPatientsDto(patients)
		response.Patients = &dto
	}

	return ec.JSON(http.StatusOK, response)
}
