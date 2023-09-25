package patients

import (
	"fmt"
	"time"
)

type Tide struct {
	Config  *TideConfig  `json:"config,omitempty"`
	Results *TideResults `json:"results,omitempty"`
}

type TideConfig struct {
	ClinicId             *string     `json:"clinicId,omitempty"`
	Filters              TideFilters `json:"filters,omitempty"`
	HighGlucoseThreshold *float64    `json:"highGlucoseThreshold,omitempty"`
	LastUploadDateFrom   *time.Time  `json:"lastUploadDateFrom,omitempty"`
	LastUploadDateTo     *time.Time  `json:"lastUploadDateTo,omitempty"`
	LowGlucoseThreshold  *float64    `json:"lowGlucoseThreshold,omitempty"`
	Period               *string     `json:"period,omitempty"`
	SchemaVersion        *int        `json:"schemaVersion,omitempty"`
	Tags                 *[]string   `json:"tags"`

	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold,omitempty"`
	VeryLowGlucoseThreshold  *float64 `json:"veryLowGlucoseThreshold,omitempty"`
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
}

type TideResults map[string]*[]TideResultPatient

type TideFilter struct {
	Comparison *string  `json:"comp,omitempty"`
	Field      *string  `json:"field,omitempty"`
	Id         *[]byte  `json:"-"`
	Value      *float64 `json:"value,omitempty"`
}

type TideFilters []*TideFilter

func DefaultTideReport() (config TideFilters) {
	config = TideFilters{
		{
			Field:      ptr("timeInVeryLowPercent"),
			Comparison: ptr(">"),
			Value:      ptr(0.01),
		},
		{
			Field:      ptr("timeInLowPercent"),
			Comparison: ptr(">"),
			Value:      ptr(0.04),
		},
		{
			Field:      ptr("timeInTargetPercentDelta"),
			Comparison: ptr("<"),
			Value:      ptr(-0.15),
		},
		{
			Field:      ptr("timeInTargetPercent"),
			Comparison: ptr("<"),
			Value:      ptr(0.7),
		},
		{
			Field:      ptr("timeCGMUsePercent"),
			Comparison: ptr("<"),
			Value:      ptr(0.7),
		},
	}

	for _, category := range config {
		category.Id = ptr([]byte(fmt.Sprintf("%s-%s-%f", *category.Field, *category.Comparison, *category.Value)))
	}

	return
}
