// Package api provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.9.0 DO NOT EDIT.
package api

import (
	"time"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
)

const (
	SessionTokenScopes = "sessionToken.Scopes"
)

// Defines values for AverageGlucoseUnits.
const (
	AverageGlucoseUnitsMmolL AverageGlucoseUnits = "mmol/L"

	AverageGlucoseUnitsMmoll AverageGlucoseUnits = "mmol/l"
)

// Defines values for ClinicClinicSize.
const (
	ClinicClinicSizeN0249 ClinicClinicSize = "0-249"

	ClinicClinicSizeN1000 ClinicClinicSize = "1000+"

	ClinicClinicSizeN250499 ClinicClinicSize = "250-499"

	ClinicClinicSizeN500999 ClinicClinicSize = "500-999"
)

// Defines values for ClinicClinicType.
const (
	ClinicClinicTypeHealthcareSystem ClinicClinicType = "healthcare_system"

	ClinicClinicTypeOther ClinicClinicType = "other"

	ClinicClinicTypeProviderPractice ClinicClinicType = "provider_practice"

	ClinicClinicTypeResearcher ClinicClinicType = "researcher"

	ClinicClinicTypeVeterinaryClinic ClinicClinicType = "veterinary_clinic"
)

// Defines values for ClinicPreferredBgUnits.
const (
	ClinicPreferredBgUnitsMgdL ClinicPreferredBgUnits = "mg/dL"

	ClinicPreferredBgUnitsMmolL ClinicPreferredBgUnits = "mmol/L"
)

// Defines values for DataSourceState.
const (
	DataSourceStateConnected DataSourceState = "connected"

	DataSourceStateDisconnected DataSourceState = "disconnected"

	DataSourceStateError DataSourceState = "error"

	DataSourceStatePending DataSourceState = "pending"

	DataSourceStatePendingReconnect DataSourceState = "pendingReconnect"
)

// Defines values for MigrationStatus.
const (
	MigrationStatusCOMPLETED MigrationStatus = "COMPLETED"

	MigrationStatusPENDING MigrationStatus = "PENDING"

	MigrationStatusRUNNING MigrationStatus = "RUNNING"
)

// Defines values for Tier.
const (
	TierTier0100 Tier = "tier0100"

	TierTier0200 Tier = "tier0200"

	TierTier0300 Tier = "tier0300"

	TierTier0400 Tier = "tier0400"
)

// AssociateClinicianToUser defines model for AssociateClinicianToUser.
type AssociateClinicianToUser struct {
	UserId string `json:"userId"`
}

// Blood glucose value, in `mmol/L`
type AverageGlucose struct {
	Units AverageGlucoseUnits `json:"units"`

	// A floating point value representing a `mmol/L` value.
	Value float32 `json:"value"`
}

// AverageGlucoseUnits defines model for AverageGlucose.Units.
type AverageGlucoseUnits string

// Clinic
type Clinic struct {
	// Street address.
	Address    *string `json:"address,omitempty"`
	CanMigrate bool    `json:"canMigrate"`

	// City name.
	City       *string           `json:"city,omitempty"`
	ClinicSize *ClinicClinicSize `json:"clinicSize,omitempty"`
	ClinicType *ClinicClinicType `json:"clinicType,omitempty"`

	// Country name.
	Country     *string   `json:"country,omitempty"`
	CreatedTime time.Time `json:"createdTime"`

	// Clinic identifier.
	Id                    Id          `json:"id"`
	LastDeletedPatientTag *PatientTag `json:"lastDeletedPatientTag,omitempty"`

	// Name of the clinic.
	Name        string        `json:"name"`
	PatientTags *[]PatientTag `json:"patientTags,omitempty"`

	// An array of phone numbers.
	PhoneNumbers *[]PhoneNumber `json:"phoneNumbers,omitempty"`

	// Postal code. In the U.S., typically the zip code such as `94301` or `94301-1704`.
	PostalCode       *string                `json:"postalCode,omitempty"`
	PreferredBgUnits ClinicPreferredBgUnits `json:"preferredBgUnits"`

	// Globally unique share code for a clinic. The share code is 3 groups of 4 uppercase alphanumeric characters in each group. Ambiguous characters such as `I` and `1`, or `O` and `0` are excluded.
	ShareCode string `json:"shareCode"`

	// State or province. In the U.S., typically something like `CA` or `California`.
	State           *string   `json:"state,omitempty"`
	Tier            string    `json:"tier"`
	TierDescription string    `json:"tierDescription"`
	UpdatedTime     time.Time `json:"updatedTime"`
	Website         *string   `json:"website,omitempty"`
}

// ClinicClinicSize defines model for Clinic.ClinicSize.
type ClinicClinicSize string

// ClinicClinicType defines model for Clinic.ClinicType.
type ClinicClinicType string

// ClinicPreferredBgUnits defines model for Clinic.PreferredBgUnits.
type ClinicPreferredBgUnits string

// The `id` may be empty if the clinician invite has not been accepted.
type Clinician struct {
	CreatedTime time.Time `json:"createdTime"`
	Email       string    `json:"email"`

	// String representation of a Tidepool User ID. Old style IDs are 10-digit strings consisting of only hexadeximcal digits. New style IDs are 36-digit [UUID v4](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random))
	Id *TidepoolUserId `json:"id,omitempty"`

	// The id of the invite if it hasn't been accepted
	InviteId *string `json:"inviteId,omitempty"`

	// The name of the clinician
	Name        *string        `json:"name,omitempty"`
	Roles       ClinicianRoles `json:"roles"`
	UpdatedTime time.Time      `json:"updatedTime"`
}

// ClinicianClinicRelationship defines model for ClinicianClinicRelationship.
type ClinicianClinicRelationship struct {
	// Clinic
	Clinic Clinic `json:"clinic"`

	// The `id` may be empty if the clinician invite has not been accepted.
	Clinician Clinician `json:"clinician"`
}

// ClinicianClinicRelationships defines model for ClinicianClinicRelationships.
type ClinicianClinicRelationships []ClinicianClinicRelationship

// ClinicianRoles defines model for ClinicianRoles.
type ClinicianRoles []string

// Clinicians defines model for Clinicians.
type Clinicians []Clinician

// Clinics defines model for Clinics.
type Clinics []Clinic

// CreatePatient defines model for CreatePatient.
type CreatePatient struct {
	AttestationSubmitted *bool `json:"attestationSubmitted,omitempty"`
	IsMigrated           *bool `json:"isMigrated,omitempty"`

	// String representation of a Tidepool User ID. Old style IDs are 10-digit strings consisting of only hexadeximcal digits. New style IDs are 36-digit [UUID v4](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random))
	LegacyClinicianId *TidepoolUserId     `json:"legacyClinicianId,omitempty"`
	Permissions       *PatientPermissions `json:"permissions,omitempty"`
}

// DataSource defines model for DataSource.
type DataSource struct {
	// String representation of a resource id
	DataSourceId *string `json:"dataSourceId,omitempty"`

	// [RFC 3339](https://www.ietf.org/rfc/rfc3339.txt) / [ISO 8601](https://www.iso.org/iso-8601-date-and-time-format.html) timestamp _with_ timezone information
	ExpirationTime *DateTime `json:"expirationTime,omitempty"`

	// [RFC 3339](https://www.ietf.org/rfc/rfc3339.txt) / [ISO 8601](https://www.iso.org/iso-8601-date-and-time-format.html) timestamp _with_ timezone information
	ModifiedTime *DateTime       `json:"modifiedTime,omitempty"`
	ProviderName string          `json:"providerName"`
	State        DataSourceState `json:"state"`
}

// DataSourceState defines model for DataSource.State.
type DataSourceState string

// DataSources defines model for DataSources.
type DataSources []DataSource

// [RFC 3339](https://www.ietf.org/rfc/rfc3339.txt) / [ISO 8601](https://www.iso.org/iso-8601-date-and-time-format.html) timestamp _with_ timezone information
type DateTime string

// Error defines model for Error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Clinic identifier.
type Id string

// Meta defines model for Meta.
type Meta struct {
	Count *int `json:"count,omitempty"`
}

// Migration defines model for Migration.
type Migration struct {
	AttestationTime *time.Time `json:"attestationTime,omitempty"`
	CreatedTime     time.Time  `json:"createdTime"`

	// The current status of the migration
	Status      *MigrationStatus `json:"status,omitempty"`
	UpdatedTime time.Time        `json:"updatedTime"`

	// The user id of the legacy clinician account that needs to be migrated.
	UserId string `json:"userId"`
}

// The current status of the migration
type MigrationStatus string

// MigrationUpdate defines model for MigrationUpdate.
type MigrationUpdate struct {
	// The current status of the migration
	Status MigrationStatus `json:"status"`
}

// Migrations defines model for Migrations.
type Migrations []Migration

// Patient defines model for Patient.
type Patient struct {
	AttestationSubmitted *bool              `json:"attestationSubmitted,omitempty"`
	BirthDate            openapi_types.Date `json:"birthDate"`
	CreatedTime          time.Time          `json:"createdTime"`
	DataSources          *[]DataSource      `json:"dataSources"`
	Email                *string            `json:"email,omitempty"`

	// The full name of the patient
	FullName string `json:"fullName"`

	// String representation of a Tidepool User ID. Old style IDs are 10-digit strings consisting of only hexadeximcal digits. New style IDs are 36-digit [UUID v4](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random))
	Id                             TidepoolUserId `json:"id"`
	LastRequestedDexcomConnectTime *time.Time     `json:"lastRequestedDexcomConnectTime,omitempty"`
	LastUploadReminderTime         *time.Time     `json:"lastUploadReminderTime,omitempty"`

	// The medical record number of the patient
	Mrn         *string             `json:"mrn,omitempty"`
	Permissions *PatientPermissions `json:"permissions,omitempty"`

	// A summary of a patients recent data
	Summary       *PatientSummary `json:"summary,omitempty"`
	Tags          *[]string       `json:"tags"`
	TargetDevices *[]string       `json:"targetDevices,omitempty"`
	UpdatedTime   time.Time       `json:"updatedTime"`
}

// Summary of a specific BGM time period (currently: 1d, 7d, 14d, 30d)
type PatientBGMPeriod struct {
	// Average daily readings
	AverageDailyRecords *float64 `json:"averageDailyRecords,omitempty"`

	// Blood glucose value, in `mmol/L`
	AverageGlucose           *AverageGlucose `json:"averageGlucose,omitempty"`
	HasAverageGlucose        *bool           `json:"hasAverageGlucose,omitempty"`
	HasTimeInHighPercent     *bool           `json:"hasTimeInHighPercent,omitempty"`
	HasTimeInLowPercent      *bool           `json:"hasTimeInLowPercent,omitempty"`
	HasTimeInTargetPercent   *bool           `json:"hasTimeInTargetPercent,omitempty"`
	HasTimeInVeryHighPercent *bool           `json:"hasTimeInVeryHighPercent,omitempty"`
	HasTimeInVeryLowPercent  *bool           `json:"hasTimeInVeryLowPercent,omitempty"`

	// Percentage of time spent in high glucose range
	TimeInHighPercent *float64 `json:"timeInHighPercent,omitempty"`

	// Counter of records in high glucose range
	TimeInHighRecords *int `json:"timeInHighRecords,omitempty"`

	// Percentage of time spent in low glucose range
	TimeInLowPercent *float64 `json:"timeInLowPercent,omitempty"`

	// Counter of records in low glucose range
	TimeInLowRecords *int `json:"timeInLowRecords,omitempty"`

	// Percentage of time spent in target glucose range
	TimeInTargetPercent *float64 `json:"timeInTargetPercent,omitempty"`

	// Counter of records in target glucose range
	TimeInTargetRecords *int `json:"timeInTargetRecords,omitempty"`

	// Percentage of time spent in very high glucose range
	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent,omitempty"`

	// Counter of records in very high glucose range
	TimeInVeryHighRecords *int `json:"timeInVeryHighRecords,omitempty"`

	// Percentage of time spent in very low glucose range
	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent,omitempty"`

	// Counter of records in very low glucose range
	TimeInVeryLowRecords *int `json:"timeInVeryLowRecords,omitempty"`

	// Counter of records
	TotalRecords *int `json:"totalRecords,omitempty"`
}

// A map to each supported BGM summary period
type PatientBGMPeriods struct {
	// Summary of a specific BGM time period (currently: 1d, 7d, 14d, 30d)
	N14d *PatientBGMPeriod `json:"14d,omitempty"`

	// Summary of a specific BGM time period (currently: 1d, 7d, 14d, 30d)
	N1d *PatientBGMPeriod `json:"1d,omitempty"`

	// Summary of a specific BGM time period (currently: 1d, 7d, 14d, 30d)
	N30d *PatientBGMPeriod `json:"30d,omitempty"`

	// Summary of a specific BGM time period (currently: 1d, 7d, 14d, 30d)
	N7d *PatientBGMPeriod `json:"7d,omitempty"`
}

// A summary of a users recent BGM glucose values
type PatientBGMStats struct {
	// Summary schema version and calculation configuration
	Config *PatientSummaryConfig `json:"config,omitempty"`

	// dates tracked for summary calculation
	Dates *PatientSummaryDates `json:"dates,omitempty"`

	// A map to each supported BGM summary period
	Periods *PatientBGMPeriods `json:"periods,omitempty"`

	// Total hours represented in the hourly stats
	TotalHours *int `json:"totalHours,omitempty"`
}

// Summary of a specific CGM time period (currently: 1d, 7d, 14d, 30d)
type PatientCGMPeriod struct {
	// Average daily readings
	AverageDailyRecords *float64 `json:"averageDailyRecords,omitempty"`

	// Blood glucose value, in `mmol/L`
	AverageGlucose *AverageGlucose `json:"averageGlucose,omitempty"`

	// A derived value which emulates A1C
	GlucoseManagementIndicator    *float64 `json:"glucoseManagementIndicator,omitempty"`
	HasAverageGlucose             *bool    `json:"hasAverageGlucose,omitempty"`
	HasGlucoseManagementIndicator *bool    `json:"hasGlucoseManagementIndicator,omitempty"`
	HasTimeCGMUsePercent          *bool    `json:"hasTimeCGMUsePercent,omitempty"`
	HasTimeInHighPercent          *bool    `json:"hasTimeInHighPercent,omitempty"`
	HasTimeInLowPercent           *bool    `json:"hasTimeInLowPercent,omitempty"`
	HasTimeInTargetPercent        *bool    `json:"hasTimeInTargetPercent,omitempty"`
	HasTimeInVeryHighPercent      *bool    `json:"hasTimeInVeryHighPercent,omitempty"`
	HasTimeInVeryLowPercent       *bool    `json:"hasTimeInVeryLowPercent,omitempty"`

	// Counter of minutes spent wearing a cgm
	TimeCGMUseMinutes *int `json:"timeCGMUseMinutes,omitempty"`

	// Percentage of time spent wearing a cgm
	TimeCGMUsePercent *float64 `json:"timeCGMUsePercent,omitempty"`

	// Counter of minutes spent wearing a cgm
	TimeCGMUseRecords *int `json:"timeCGMUseRecords,omitempty"`

	// Counter of minutes spent in high glucose range
	TimeInHighMinutes *int `json:"timeInHighMinutes,omitempty"`

	// Percentage of time spent in high glucose range
	TimeInHighPercent *float64 `json:"timeInHighPercent,omitempty"`

	// Counter of records in high glucose range
	TimeInHighRecords *int `json:"timeInHighRecords,omitempty"`

	// Counter of minutes spent in low glucose range
	TimeInLowMinutes *int `json:"timeInLowMinutes,omitempty"`

	// Percentage of time spent in low glucose range
	TimeInLowPercent *float64 `json:"timeInLowPercent,omitempty"`

	// Counter of records in low glucose range
	TimeInLowRecords *int `json:"timeInLowRecords,omitempty"`

	// Counter of minutes spent in target glucose range
	TimeInTargetMinutes *int `json:"timeInTargetMinutes,omitempty"`

	// Percentage of time spent in target glucose range
	TimeInTargetPercent *float64 `json:"timeInTargetPercent,omitempty"`

	// Counter of records in target glucose range
	TimeInTargetRecords *int `json:"timeInTargetRecords,omitempty"`

	// Counter of minutes spent in very high glucose range
	TimeInVeryHighMinutes *int `json:"timeInVeryHighMinutes,omitempty"`

	// Percentage of time spent in very high glucose range
	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent,omitempty"`

	// Counter of records in very high glucose range
	TimeInVeryHighRecords *int `json:"timeInVeryHighRecords,omitempty"`

	// Counter of minutes spent in very low glucose range
	TimeInVeryLowMinutes *int `json:"timeInVeryLowMinutes,omitempty"`

	// Percentage of time spent in very low glucose range
	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent,omitempty"`

	// Counter of records in very low glucose range
	TimeInVeryLowRecords *int `json:"timeInVeryLowRecords,omitempty"`

	// Counter of records
	TotalRecords *int `json:"totalRecords,omitempty"`
}

// A map to each supported CGM summary period
type PatientCGMPeriods struct {
	// Summary of a specific CGM time period (currently: 1d, 7d, 14d, 30d)
	N14d *PatientCGMPeriod `json:"14d,omitempty"`

	// Summary of a specific CGM time period (currently: 1d, 7d, 14d, 30d)
	N1d *PatientCGMPeriod `json:"1d,omitempty"`

	// Summary of a specific CGM time period (currently: 1d, 7d, 14d, 30d)
	N30d *PatientCGMPeriod `json:"30d,omitempty"`

	// Summary of a specific CGM time period (currently: 1d, 7d, 14d, 30d)
	N7d *PatientCGMPeriod `json:"7d,omitempty"`
}

// A summary of a users recent CGM glucose values
type PatientCGMStats struct {
	// Summary schema version and calculation configuration
	Config *PatientSummaryConfig `json:"config,omitempty"`

	// dates tracked for summary calculation
	Dates *PatientSummaryDates `json:"dates,omitempty"`

	// A map to each supported CGM summary period
	Periods *PatientCGMPeriods `json:"periods,omitempty"`

	// Total hours represented in the hourly stats
	TotalHours *int `json:"totalHours,omitempty"`
}

// PatientClinicRelationship defines model for PatientClinicRelationship.
type PatientClinicRelationship struct {
	// Clinic
	Clinic  Clinic  `json:"clinic"`
	Patient Patient `json:"patient"`
}

// PatientClinicRelationships defines model for PatientClinicRelationships.
type PatientClinicRelationships []PatientClinicRelationship

// PatientPermissions defines model for PatientPermissions.
type PatientPermissions struct {
	Custodian *map[string]interface{} `json:"custodian,omitempty"`
	Note      *map[string]interface{} `json:"note,omitempty"`
	Upload    *map[string]interface{} `json:"upload,omitempty"`
	View      *map[string]interface{} `json:"view,omitempty"`
}

// A summary of a patients recent data
type PatientSummary struct {
	// A summary of a users recent BGM glucose values
	BgmStats *PatientBGMStats `json:"bgmStats,omitempty"`

	// A summary of a users recent CGM glucose values
	CgmStats *PatientCGMStats `json:"cgmStats,omitempty"`
}

// Summary schema version and calculation configuration
type PatientSummaryConfig struct {
	// Threshold used for determining if a value is high
	HighGlucoseThreshold *float64 `json:"highGlucoseThreshold,omitempty"`

	// Threshold used for determining if a value is low
	LowGlucoseThreshold *float64 `json:"lowGlucoseThreshold,omitempty"`

	// Summary schema version
	SchemaVersion *int `json:"schemaVersion,omitempty"`

	// Threshold used for determining if a value is very high
	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold,omitempty"`

	// Threshold used for determining if a value is very low
	VeryLowGlucoseThreshold *float64 `json:"veryLowGlucoseThreshold,omitempty"`
}

// dates tracked for summary calculation
type PatientSummaryDates struct {
	// Date of the first included value
	FirstData         *time.Time `json:"firstData,omitempty"`
	HasLastUploadDate *bool      `json:"hasLastUploadDate,omitempty"`

	// Date of the last calculated value
	LastData *time.Time `json:"lastData,omitempty"`

	// Date of the last calculation
	LastUpdatedDate *time.Time `json:"lastUpdatedDate,omitempty"`

	// Created date of the last calculated value
	LastUploadDate *time.Time `json:"lastUploadDate,omitempty"`

	// Date of the first user upload after lastData, removed when calculated
	OutdatedSince *time.Time `json:"outdatedSince,omitempty"`
}

// PatientTag defines model for PatientTag.
type PatientTag struct {
	// String representation of a resource id
	Id string `json:"id"`

	// The tag display name
	Name string `json:"name"`
}

// Patients defines model for Patients.
type Patients []Patient

// PatientsResponse defines model for PatientsResponse.
type PatientsResponse struct {
	Data *Patients `json:"data,omitempty"`
	Meta *Meta     `json:"meta,omitempty"`
}

// PhoneNumber defines model for PhoneNumber.
type PhoneNumber struct {
	Number string  `json:"number"`
	Type   *string `json:"type,omitempty"`
}

// String representation of a Tidepool User ID. Old style IDs are 10-digit strings consisting of only hexadeximcal digits. New style IDs are 36-digit [UUID v4](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random))
type TidepoolUserId string

// Tier defines model for Tier.
type Tier string

// TriggerMigration defines model for TriggerMigration.
type TriggerMigration struct {
	AttestationSubmitted *bool `json:"attestationSubmitted,omitempty"`
}

// UpdateTier defines model for UpdateTier.
type UpdateTier struct {
	Tier Tier `json:"tier"`
}

// UpdateUserDetails defines model for UpdateUserDetails.
type UpdateUserDetails struct {
	Email *openapi_types.Email `json:"email,omitempty"`
}

// ClinicId defines model for clinicId.
type ClinicId string

// ClinicianId defines model for clinicianId.
type ClinicianId string

// CreatedTimeEnd defines model for createdTimeEnd.
type CreatedTimeEnd time.Time

// CreatedTimeStart defines model for createdTimeStart.
type CreatedTimeStart time.Time

// Email defines model for email.
type Email openapi_types.Email

// InviteId defines model for inviteId.
type InviteId string

// Limit defines model for limit.
type Limit int

// Offset defines model for offset.
type Offset int

// PatientId defines model for patientId.
type PatientId string

// PatientTagId defines model for patientTagId.
type PatientTagId string

// Role defines model for role.
type Role string

// Search defines model for search.
type Search string

// ShareCode defines model for shareCode.
type ShareCode string

// Sort defines model for sort.
type Sort string

// String representation of a Tidepool User ID. Old style IDs are 10-digit strings consisting of only hexadeximcal digits. New style IDs are 36-digit [UUID v4](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random))
type UserId TidepoolUserId

// ListAllCliniciansParams defines parameters for ListAllClinicians.
type ListAllCliniciansParams struct {
	Offset *Offset `json:"offset,omitempty"`
	Limit  *Limit  `json:"limit,omitempty"`

	// Return records created after the given date (inclusive)
	CreatedTimeStart *CreatedTimeStart `json:"createdTimeStart,omitempty"`

	// Return records created before the given date (exclusive)
	CreatedTimeEnd *CreatedTimeEnd `json:"createdTimeEnd,omitempty"`
}

// ListClinicsForClinicianParams defines parameters for ListClinicsForClinician.
type ListClinicsForClinicianParams struct {
	Offset *Offset `json:"offset,omitempty"`
	Limit  *Limit  `json:"limit,omitempty"`
}

// ListClinicsParams defines parameters for ListClinics.
type ListClinicsParams struct {
	Limit     *Limit     `json:"limit,omitempty"`
	Offset    *Offset    `json:"offset,omitempty"`
	ShareCode *ShareCode `json:"shareCode,omitempty"`

	// Return records created after the given date (inclusive)
	CreatedTimeStart *CreatedTimeStart `json:"createdTimeStart,omitempty"`

	// Return records created before the given date (exclusive)
	CreatedTimeEnd *CreatedTimeEnd `json:"createdTimeEnd,omitempty"`
}

// CreateClinicJSONBody defines parameters for CreateClinic.
type CreateClinicJSONBody Clinic

// UpdateClinicJSONBody defines parameters for UpdateClinic.
type UpdateClinicJSONBody Clinic

// ListCliniciansParams defines parameters for ListClinicians.
type ListCliniciansParams struct {
	// Full text search query
	Search *Search `json:"search,omitempty"`
	Offset *Offset `json:"offset,omitempty"`
	Limit  *Limit  `json:"limit,omitempty"`
	Email  *Email  `json:"email,omitempty"`
	Role   *Role   `json:"role,omitempty"`
}

// CreateClinicianJSONBody defines parameters for CreateClinician.
type CreateClinicianJSONBody Clinician

// UpdateClinicianJSONBody defines parameters for UpdateClinician.
type UpdateClinicianJSONBody Clinician

// AssociateClinicianToUserJSONBody defines parameters for AssociateClinicianToUser.
type AssociateClinicianToUserJSONBody AssociateClinicianToUser

// TriggerInitialMigrationJSONBody defines parameters for TriggerInitialMigration.
type TriggerInitialMigrationJSONBody TriggerMigration

// MigrateLegacyClinicianPatientsJSONBody defines parameters for MigrateLegacyClinicianPatients.
type MigrateLegacyClinicianPatientsJSONBody Migration

// UpdateMigrationJSONBody defines parameters for UpdateMigration.
type UpdateMigrationJSONBody MigrationUpdate

// CreatePatientTagJSONBody defines parameters for CreatePatientTag.
type CreatePatientTagJSONBody PatientTag

// UpdatePatientTagJSONBody defines parameters for UpdatePatientTag.
type UpdatePatientTagJSONBody PatientTag

// ListPatientsParams defines parameters for ListPatients.
type ListPatientsParams struct {
	// Full text search query
	Search *Search `json:"search,omitempty"`
	Offset *Offset `json:"offset,omitempty"`
	Limit  *Limit  `json:"limit,omitempty"`

	// Sort order and attribute (e.g. +name or -name)
	Sort *Sort `json:"sort,omitempty"`

	// Summary type to sort by
	SortType *string `json:"sortType,omitempty"`

	// Time Period to sort
	SortPeriod *string `json:"sortPeriod,omitempty"`

	// Time Period to filter
	Period *string `json:"period,omitempty"`

	// Percentage of time of CGM use
	CgmTimeCGMUsePercent *string `json:"cgm.timeCGMUsePercent,omitempty"`

	// Percentage of time below 54 mg/dL
	CgmTimeInVeryLowPercent *string `json:"cgm.timeInVeryLowPercent,omitempty"`

	// Percentage of time in range 54-70 mg/dL
	CgmTimeInLowPercent *string `json:"cgm.timeInLowPercent,omitempty"`

	// Percentage of time in range 70-180 mg/dL
	CgmTimeInTargetPercent *string `json:"cgm.timeInTargetPercent,omitempty"`

	// Percentage of time in range 180-250 mg/dL
	CgmTimeInHighPercent *string `json:"cgm.timeInHighPercent,omitempty"`

	// Percentage of time above range 250 mg/dL
	CgmTimeInVeryHighPercent *string `json:"cgm.timeInVeryHighPercent,omitempty"`

	// Inclusive
	CgmLastUploadDateFrom *time.Time `json:"cgm.lastUploadDateFrom,omitempty"`

	// Exclusive
	CgmLastUploadDateTo *time.Time `json:"cgm.lastUploadDateTo,omitempty"`

	// Percentage of time below 54 mg/dL
	BgmTimeInVeryLowPercent *string `json:"bgm.timeInVeryLowPercent,omitempty"`

	// Percentage of time in range 54-70 mg/dL
	BgmTimeInLowPercent *string `json:"bgm.timeInLowPercent,omitempty"`

	// Percentage of time in range 70-180 mg/dL
	BgmTimeInTargetPercent *string `json:"bgm.timeInTargetPercent,omitempty"`

	// Percentage of time in range 180-250 mg/dL
	BgmTimeInHighPercent *string `json:"bgm.timeInHighPercent,omitempty"`

	// Percentage of time above range 250 mg/dL
	BgmTimeInVeryHighPercent *string `json:"bgm.timeInVeryHighPercent,omitempty"`

	// Inclusive
	BgmLastUploadDateFrom *time.Time `json:"bgm.lastUploadDateFrom,omitempty"`

	// Exclusive
	BgmLastUploadDateTo *time.Time `json:"bgm.lastUploadDateTo,omitempty"`

	// Comma-separated list of patient tag IDs
	Tags *[]string `json:"tags,omitempty"`
}

// CreatePatientAccountJSONBody defines parameters for CreatePatientAccount.
type CreatePatientAccountJSONBody Patient

// CreatePatientFromUserJSONBody defines parameters for CreatePatientFromUser.
type CreatePatientFromUserJSONBody CreatePatient

// UpdatePatientJSONBody defines parameters for UpdatePatient.
type UpdatePatientJSONBody Patient

// UpdatePatientPermissionsJSONBody defines parameters for UpdatePatientPermissions.
type UpdatePatientPermissionsJSONBody PatientPermissions

// UpdateTierJSONBody defines parameters for UpdateTier.
type UpdateTierJSONBody UpdateTier

// UpdatePatientSummaryJSONBody defines parameters for UpdatePatientSummary.
type UpdatePatientSummaryJSONBody PatientSummary

// ListClinicsForPatientParams defines parameters for ListClinicsForPatient.
type ListClinicsForPatientParams struct {
	Offset *Offset `json:"offset,omitempty"`
	Limit  *Limit  `json:"limit,omitempty"`
}

// UpdatePatientDataSourcesJSONBody defines parameters for UpdatePatientDataSources.
type UpdatePatientDataSourcesJSONBody DataSources

// UpdateClinicUserDetailsJSONBody defines parameters for UpdateClinicUserDetails.
type UpdateClinicUserDetailsJSONBody UpdateUserDetails

// CreateClinicJSONRequestBody defines body for CreateClinic for application/json ContentType.
type CreateClinicJSONRequestBody CreateClinicJSONBody

// UpdateClinicJSONRequestBody defines body for UpdateClinic for application/json ContentType.
type UpdateClinicJSONRequestBody UpdateClinicJSONBody

// CreateClinicianJSONRequestBody defines body for CreateClinician for application/json ContentType.
type CreateClinicianJSONRequestBody CreateClinicianJSONBody

// UpdateClinicianJSONRequestBody defines body for UpdateClinician for application/json ContentType.
type UpdateClinicianJSONRequestBody UpdateClinicianJSONBody

// AssociateClinicianToUserJSONRequestBody defines body for AssociateClinicianToUser for application/json ContentType.
type AssociateClinicianToUserJSONRequestBody AssociateClinicianToUserJSONBody

// TriggerInitialMigrationJSONRequestBody defines body for TriggerInitialMigration for application/json ContentType.
type TriggerInitialMigrationJSONRequestBody TriggerInitialMigrationJSONBody

// MigrateLegacyClinicianPatientsJSONRequestBody defines body for MigrateLegacyClinicianPatients for application/json ContentType.
type MigrateLegacyClinicianPatientsJSONRequestBody MigrateLegacyClinicianPatientsJSONBody

// UpdateMigrationJSONRequestBody defines body for UpdateMigration for application/json ContentType.
type UpdateMigrationJSONRequestBody UpdateMigrationJSONBody

// CreatePatientTagJSONRequestBody defines body for CreatePatientTag for application/json ContentType.
type CreatePatientTagJSONRequestBody CreatePatientTagJSONBody

// UpdatePatientTagJSONRequestBody defines body for UpdatePatientTag for application/json ContentType.
type UpdatePatientTagJSONRequestBody UpdatePatientTagJSONBody

// CreatePatientAccountJSONRequestBody defines body for CreatePatientAccount for application/json ContentType.
type CreatePatientAccountJSONRequestBody CreatePatientAccountJSONBody

// CreatePatientFromUserJSONRequestBody defines body for CreatePatientFromUser for application/json ContentType.
type CreatePatientFromUserJSONRequestBody CreatePatientFromUserJSONBody

// UpdatePatientJSONRequestBody defines body for UpdatePatient for application/json ContentType.
type UpdatePatientJSONRequestBody UpdatePatientJSONBody

// UpdatePatientPermissionsJSONRequestBody defines body for UpdatePatientPermissions for application/json ContentType.
type UpdatePatientPermissionsJSONRequestBody UpdatePatientPermissionsJSONBody

// UpdateTierJSONRequestBody defines body for UpdateTier for application/json ContentType.
type UpdateTierJSONRequestBody UpdateTierJSONBody

// UpdatePatientSummaryJSONRequestBody defines body for UpdatePatientSummary for application/json ContentType.
type UpdatePatientSummaryJSONRequestBody UpdatePatientSummaryJSONBody

// UpdatePatientDataSourcesJSONRequestBody defines body for UpdatePatientDataSources for application/json ContentType.
type UpdatePatientDataSourcesJSONRequestBody UpdatePatientDataSourcesJSONBody

// UpdateClinicUserDetailsJSONRequestBody defines body for UpdateClinicUserDetails for application/json ContentType.
type UpdateClinicUserDetailsJSONRequestBody UpdateClinicUserDetailsJSONBody
