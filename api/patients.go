package api

import (
	"github.com/labstack/echo/v4"
)

// deletePatientFromClinic
// (DELETE /clinic/{clinicid}/patient/{patientid})
func (c *ClinicServer) DeleteClinicClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}
// addPatientToClinic
// (POST /clinic/{clinicid}/patient/{patientid})
func (c *ClinicServer) PostClinicClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}
// getCliniciansForClinic
// (GET /clinic/{clinicid}/patients)
func (c *ClinicServer) GetClinicClinicidPatients(ctx echo.Context, clinicid string, params GetClinicClinicidPatientsParams) error {
	return nil
}
