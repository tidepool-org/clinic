package api

import (
	"github.com/labstack/echo/v4"
)

// GetPatientsForClinic
// (GET /clinics/{clinicid}/patients)
func (c *ClinicServer) GetClinicsClinicidPatients(ctx echo.Context, clinicid string, params GetClinicsClinicidPatientsParams) error {
	return nil
}

// AddPatientToClinic
// (POST /clinics/{clinicid}/patients)
func (c *ClinicServer) PostClinicsClinicidPatients(ctx echo.Context, clinicid string) error {
	return nil
}

// DeletePatientFromClinic
// (DELETE /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) DeleteClinicClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}


// GetPatientFromClinic
// (GET /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) GetClinicsClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}

// ModifyClinicPatient
// (PATCH /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) PatchClinicsClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}