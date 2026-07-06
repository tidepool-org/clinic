package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tidepool-org/clinic/ehr"
	"github.com/tidepool-org/clinic/errors"
)

func (h *Handler) GetPatientFlowsheet(ec echo.Context, clinicId ClinicId, patientId PatientId) error {
	ctx := ec.Request().Context()

	clinic, err := h.Clinics.Get(ctx, clinicId)
	if err != nil {
		return err
	}

	patient, err := h.Patients.Get(ctx, clinicId, patientId)
	if err != nil {
		return err
	}
	if patient.Summary == nil {
		return errors.NotFound
	}

	icode := false
	if clinic.EHRSettings != nil {
		icode = clinic.EHRSettings.Flowsheets.Icode
	}

	stats := ehr.Compute(patient.Summary, ehr.GlucoseUnits(clinic.PreferredBgUnits), icode)

	response := make([]FlowsheetObservationV1, 0, len(stats))
	for _, s := range stats {
		var units *string
		if s.Unit != "" {
			units = &s.Unit
		}
		response = append(response, FlowsheetObservationV1{
			Code:        s.Code,
			Value:       s.Value,
			ValueType:   FlowsheetObservationV1ValueType(s.ValueType()),
			Units:       units,
			Description: s.Description,
			DateTime:    s.DateTime,
		})
	}

	return ec.JSON(http.StatusOK, response)
}
