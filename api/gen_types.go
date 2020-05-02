// Package Clinic provides primitives to interact the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen DO NOT EDIT.
package api

// Clinic defines model for Clinic.
type Clinic struct {
	// Embedded fields due to inline allOf schema
	Id string `json:"_id"`
	// Embedded struct due to allOf(#/components/schemas/NewClinic)
	NewClinic
}

// ClinicianPermissions defines model for ClinicianPermissions.
type ClinicianPermissions struct {
	Permissions *[]string `json:"permissions,omitempty"`
}

// ClinicsClinicians defines model for ClinicsClinicians.
type ClinicsClinicians struct {
	// Embedded fields due to inline allOf schema
	ClinicId    *string `json:"clinicId,omitempty"`
	ClinicianId string  `json:"clinicianId"`
	// Embedded struct due to allOf(#/components/schemas/ClinicianPermissions)
	ClinicianPermissions
}

// ClinicsPatients defines model for ClinicsPatients.
type ClinicsPatients struct {
	// Embedded fields due to inline allOf schema
	ClinicId  *string `json:"clinicId,omitempty"`
	Id        *string `json:"id,omitempty"`
	PatientId *string `json:"patientId,omitempty"`
	// Embedded struct due to allOf(#/components/schemas/PatientPermissions)
	PatientPermissions
}

// NewClinic defines model for NewClinic.
type NewClinic struct {
	Address      *string                 `json:"address,omitempty"`
	Location     *string                 `json:"location,omitempty"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
	Name         *string                 `json:"name,omitempty"`
	PhoneNumbers *[]struct {
		Number *string `json:"number,omitempty"`
		Type   *string `json:"type,omitempty"`
	} `json:"phoneNumbers,omitempty"`
}

// PatientPermissions defines model for PatientPermissions.
type PatientPermissions struct {
	Permissions *[]string `json:"permissions,omitempty"`
}

// GetClinicsParams defines parameters for GetClinics.
type GetClinicsParams struct {
	Offset    *int    `json:"offset,omitempty"`
	Limit     *int    `json:"limit,omitempty"`
	SortOrder *string `json:"sortOrder,omitempty"`
}

// PostClinicsJSONBody defines parameters for PostClinics.
type PostClinicsJSONBody NewClinic

// PatchClinicsClinicidJSONBody defines parameters for PatchClinicsClinicid.
type PatchClinicsClinicidJSONBody NewClinic

// GetClinicsClinicidCliniciansParams defines parameters for GetClinicsClinicidClinicians.
type GetClinicsClinicidCliniciansParams struct {
	Offset    *int    `json:"offset,omitempty"`
	Limit     *int    `json:"limit,omitempty"`
	SortOrder *string `json:"sortOrder,omitempty"`
}

// PostClinicsClinicidCliniciansJSONBody defines parameters for PostClinicsClinicidClinicians.
type PostClinicsClinicidCliniciansJSONBody ClinicsClinicians

// PatchClinicsClinicidCliniciansClinicianidJSONBody defines parameters for PatchClinicsClinicidCliniciansClinicianid.
type PatchClinicsClinicidCliniciansClinicianidJSONBody ClinicianPermissions

// GetClinicsClinicidPatientsParams defines parameters for GetClinicsClinicidPatients.
type GetClinicsClinicidPatientsParams struct {
	Offset    *int    `json:"offset,omitempty"`
	Limit     *int    `json:"limit,omitempty"`
	SortOrder *string `json:"sortOrder,omitempty"`
}

// PostClinicsClinicidPatientsJSONBody defines parameters for PostClinicsClinicidPatients.
type PostClinicsClinicidPatientsJSONBody ClinicsPatients

// PatchClinicsClinicidPatientsPatientidJSONBody defines parameters for PatchClinicsClinicidPatientsPatientid.
type PatchClinicsClinicidPatientsPatientidJSONBody PatientPermissions

// PostClinicsRequestBody defines body for PostClinics for application/json ContentType.
type PostClinicsJSONRequestBody PostClinicsJSONBody

// PatchClinicsClinicidRequestBody defines body for PatchClinicsClinicid for application/json ContentType.
type PatchClinicsClinicidJSONRequestBody PatchClinicsClinicidJSONBody

// PostClinicsClinicidCliniciansRequestBody defines body for PostClinicsClinicidClinicians for application/json ContentType.
type PostClinicsClinicidCliniciansJSONRequestBody PostClinicsClinicidCliniciansJSONBody

// PatchClinicsClinicidCliniciansClinicianidRequestBody defines body for PatchClinicsClinicidCliniciansClinicianid for application/json ContentType.
type PatchClinicsClinicidCliniciansClinicianidJSONRequestBody PatchClinicsClinicidCliniciansClinicianidJSONBody

// PostClinicsClinicidPatientsRequestBody defines body for PostClinicsClinicidPatients for application/json ContentType.
type PostClinicsClinicidPatientsJSONRequestBody PostClinicsClinicidPatientsJSONBody

// PatchClinicsClinicidPatientsPatientidRequestBody defines body for PatchClinicsClinicidPatientsPatientid for application/json ContentType.
type PatchClinicsClinicidPatientsPatientidJSONRequestBody PatchClinicsClinicidPatientsPatientidJSONBody
