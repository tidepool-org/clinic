package api

import (
	"github.com/labstack/echo/v4"
)

// DeleteClinicianForClinic
// (DELETE /clinic/{clinicid}/clinician/{clinicianid})
func (c *ClinicServer) DeleteClinicClinicidClinicianClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	return nil
}

// AddClinicianToClinic
// (POST /clinic/{clinicid}/clinician/{clinicianid})
func (c *ClinicServer) PostClinicClinicidClinicianClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	return nil
}
// getCliniciansForClinic
// (GET /clinics/{clinicid}/clinicians)
func (c *ClinicServer) GetClinicsClinicidClinicians(ctx echo.Context, clinicid string, params GetClinicsClinicidCliniciansParams) error {
	return nil
}
