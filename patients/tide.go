package patients

import "time"

const (
	veryLowGlucoseThreshold     = 3.0
	lowGlucoseThreshold         = 3.9
	highGlucoseThreshold        = 10.0
	veryHighGlucoseThreshold    = 13.9
	extremeHighGlucoseThreshold = 19.4
	tideSchemaVersion           = 2
)

type Tide struct {
	Config   TideConfig   `json:"config"`
	Results  TideResults  `json:"results"`
	Metadata TideMetadata `json:"metadata"`
}

type TideConfig struct {
	ClinicId                    string      `json:"clinicId,omitempty"`
	Filters                     TideFilters `json:"filters"`
	HighGlucoseThreshold        float64     `json:"highGlucoseThreshold"`
	LastDataCutoff              time.Time   `json:"lastDataCutoff"`
	LowGlucoseThreshold         float64     `json:"lowGlucoseThreshold"`
	Period                      string      `json:"period"`
	SchemaVersion               int         `json:"schemaVersion"`
	Tags                        []string    `json:"tags"`
	VeryHighGlucoseThreshold    float64     `json:"veryHighGlucoseThreshold"`
	VeryLowGlucoseThreshold     float64     `json:"veryLowGlucoseThreshold"`
	ExtremeHighGlucoseThreshold float64     `json:"extremeHighGlucoseThreshold"`
}

type TideFilters struct {
	DropInTimeInTargetPercent *string `json:"dropInTimeInTargetPercent,omitempty"`
	TimeCGMUsePercent         *string `json:"timeCGMUsePercent,omitempty"`
	TimeInAnyLowPercent       *string `json:"timeInAnyLowPercent,omitempty"`
	TimeInExtremeHighPercent  *string `json:"timeInExtremeHighPercent,omitempty"`
	TimeInHighPercent         *string `json:"timeInHighPercent,omitempty"`
	TimeInTargetPercent       *string `json:"timeInTargetPercent,omitempty"`
	TimeInVeryHighPercent     *string `json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryLowPercent      *string `json:"timeInVeryLowPercent,omitempty"`
}

type TidePatient struct {
	Email       *string       `json:"email"`
	FullName    *string       `json:"fullName"`
	Id          *string       `json:"id,omitempty"`
	Tags        []string      `json:"tags"`
	Reviews     []Review      `json:"reviews"`
	DataSources *[]DataSource `json:"dataSources"`
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
	LastData                   *time.Time  `json:"lastData,omitempty"`
}

type TideResults map[string][]TideResultPatient

type TideMetadata struct {
	// CandidatePatients is the number of patients considered for the Tide Report, after
	// filters, but before limits.
	CandidatePatients int
	// SelectedPatients is the number of patients included in the Tide Report, after filters
	// and limits.
	SelectedPatients int
}
