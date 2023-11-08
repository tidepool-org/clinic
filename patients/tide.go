package patients

import "time"

type Tide struct {
	Config  *TideConfig  `json:"config,omitempty"`
	Results *TideResults `json:"results,omitempty"`
}

type TideConfig struct {
	ClinicId             *string      `json:"clinicId,omitempty"`
	Filters              *TideFilters `json:"filters,omitempty"`
	HighGlucoseThreshold *float64     `json:"highGlucoseThreshold,omitempty"`
	LastUploadDateFrom   *time.Time   `json:"lastUploadDateFrom,omitempty"`
	LastUploadDateTo     *time.Time   `json:"lastUploadDateTo,omitempty"`
	LowGlucoseThreshold  *float64     `json:"lowGlucoseThreshold,omitempty"`
	Period               *string      `json:"period,omitempty"`
	SchemaVersion        *int         `json:"schemaVersion,omitempty"`
	Tags                 *[]string    `json:"tags"`

	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold,omitempty"`
	VeryLowGlucoseThreshold  *float64 `json:"veryLowGlucoseThreshold,omitempty"`
}

type TideFilters struct {
	DropInTimeInTargetPercent *string `json:"dropInTimeInTargetPercent,omitempty"`
	TimeCGMUsePercent         *string `json:"timeCGMUsePercent,omitempty"`
	TimeInAnyLowPercent       *string `json:"timeInAnyLowPercent,omitempty"`
	TimeInTargetPercent       *string `json:"timeInTargetPercent,omitempty"`
	TimeInVeryLowPercent      *string `json:"timeInVeryLowPercent,omitempty"`
}

type TidePatient struct {
	Email    *string   `json:"email,omitempty"`
	FullName *string   `json:"fullName,omitempty"`
	Id       *string   `json:"id,omitempty"`
	Tags     *[]string `json:"tags"`
}

type TideResultPatient struct {
	AverageGlucoseMmol         *float64     `json:"averageGlucoseMmol,omitempty"`
	GlucoseManagementIndicator *float64     `json:"glucoseManagementIndicator,omitempty"`
	Patient                    *TidePatient `json:"patient,omitempty"`
	TimeCGMUseMinutes          *int         `json:"timeCGMUseMinutes,omitempty"`
	TimeCGMUsePercent          *float64     `json:"timeCGMUsePercent,omitempty"`
	TimeInHighPercent          *float64     `json:"timeInHighPercent,omitempty"`
	TimeInLowPercent           *float64     `json:"timeInLowPercent,omitempty"`
	TimeInTargetPercent        *float64     `json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta   *float64     `json:"timeInTargetPercentDelta,omitempty"`
	TimeInVeryHighPercent      *float64     `json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryLowPercent       *float64     `json:"timeInVeryLowPercent,omitempty"`
	TimeInAnyLowPercent        *float64     `json:"timeInAnyLowPercent,omitempty"`
	TimeInAnyHighPercent       *float64     `json:"timeInAnyHighPercent,omitempty"`
}

type TideResults map[string]*[]TideResultPatient

type PatientTagIds = []string
