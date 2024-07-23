package patients

import "time"

type Tide struct {
	Config  TideConfig  `json:"config"`
	Results TideResults `json:"results"`
}

type TideConfig struct {
	ClinicId                 string      `json:"clinicId,omitempty"`
	Filters                  TideFilters `json:"filters"`
	HighGlucoseThreshold     float64     `json:"highGlucoseThreshold"`
	LastDataFrom             time.Time   `json:"lastDataFrom"`
	LastDataTo               time.Time   `json:"lastDataTo"`
	LowGlucoseThreshold      float64     `json:"lowGlucoseThreshold"`
	Period                   string      `json:"period"`
	SchemaVersion            int         `json:"schemaVersion"`
	Tags                     []string    `json:"tags"`
	VeryHighGlucoseThreshold float64     `json:"veryHighGlucoseThreshold"`
	VeryLowGlucoseThreshold  float64     `json:"veryLowGlucoseThreshold"`
}

type TideFilters struct {
	DropInTimeInTargetPercent string `json:"dropInTimeInTargetPercent"`
	TimeCGMUsePercent         string `json:"timeCGMUsePercent"`
	TimeInAnyLowPercent       string `json:"timeInAnyLowPercent"`
	TimeInTargetPercent       string `json:"timeInTargetPercent"`
	TimeInVeryLowPercent      string `json:"timeInVeryLowPercent"`
}

type TidePatient struct {
	Email    *string   `json:"email"`
	FullName *string   `json:"fullName"`
	Id       *string   `json:"id,omitempty"`
	Tags     *[]string `json:"tags"`
	Reviews  []Review  `json:"reviews"`
}

type TideResultPatient struct {
	AverageGlucoseMmol         *float64    `json:"averageGlucoseMmol,omitempty"`
	GlucoseManagementIndicator *float64    `json:"glucoseManagementIndicator,omitempty"`
	Patient                    TidePatient `json:"patient"`
	TimeCGMUseMinutes          *int        `json:"timeCGMUseMinutes,omitempty"`
	TimeCGMUsePercent          *float64    `json:"timeCGMUsePercent,omitempty"`
	TimeInHighPercent          *float64    `json:"timeInHighPercent,omitempty"`
	TimeInLowPercent           *float64    `json:"timeInLowPercent,omitempty"`
	TimeInTargetPercent        *float64    `json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta   *float64    `json:"timeInTargetPercentDelta,omitempty"`
	TimeInVeryHighPercent      *float64    `json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryLowPercent       *float64    `json:"timeInVeryLowPercent,omitempty"`
	TimeInAnyHighPercent       *float64    `json:"timeInAnyHighPercent,omitempty"`
	TimeInAnyLowPercent        *float64    `json:"timeInAnyLowPercent,omitempty"`
}

type TideResults map[string]*[]TideResultPatient
