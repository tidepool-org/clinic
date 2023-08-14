package patients

import "time"

type PatientBGMPeriod struct {
	AverageDailyRecords        *float64 `json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta   *float64 `json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol         *float64 `json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta    *float64 `json:"averageGlucoseMmolDelta,omitempty"`
	HasAverageDailyRecords     *bool    `json:"hasAverageDailyRecords,omitempty"`
	HasAverageGlucoseMmol      *bool    `json:"hasAverageGlucoseMmol,omitempty"`
	HasTimeInHighPercent       *bool    `json:"hasTimeInHighPercent,omitempty"`
	HasTimeInHighRecords       *bool    `json:"hasTimeInHighRecords,omitempty"`
	HasTimeInLowPercent        *bool    `json:"hasTimeInLowPercent,omitempty"`
	HasTimeInLowRecords        *bool    `json:"hasTimeInLowRecords,omitempty"`
	HasTimeInTargetPercent     *bool    `json:"hasTimeInTargetPercent,omitempty"`
	HasTimeInTargetRecords     *bool    `json:"hasTimeInTargetRecords,omitempty"`
	HasTimeInVeryHighPercent   *bool    `json:"hasTimeInVeryHighPercent,omitempty"`
	HasTimeInVeryHighRecords   *bool    `json:"hasTimeInVeryHighRecords,omitempty"`
	HasTimeInVeryLowPercent    *bool    `json:"hasTimeInVeryLowPercent,omitempty"`
	HasTimeInVeryLowRecords    *bool    `json:"hasTimeInVeryLowRecords,omitempty"`
	HasTotalRecords            *bool    `json:"hasTotalRecords,omitempty"`
	TimeInHighPercent          *float64 `json:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta     *float64 `json:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords          *int     `json:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta     *int     `json:"timeInHighRecordsDelta,omitempty"`
	TimeInLowPercent           *float64 `json:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta      *float64 `json:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords           *int     `json:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta      *int     `json:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetPercent        *float64 `json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta   *float64 `json:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords        *int     `json:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta   *int     `json:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighPercent      *float64 `json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta *float64 `json:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords      *int     `json:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta *int     `json:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowPercent       *float64 `json:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta  *float64 `json:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords       *int     `json:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta  *int     `json:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords               *int     `json:"totalRecords,omitempty"`
	TotalRecordsDelta          *int     `json:"totalRecordsDelta,omitempty"`
}

type PatientBGMPeriods map[string]PatientBGMPeriod

type PatientBGMStats struct {
	Config        *PatientSummaryConfig `json:"config,omitempty"`
	Dates         *PatientSummaryDates  `json:"dates,omitempty"`
	OffsetPeriods *PatientBGMPeriods    `json:"offsetPeriods,omitempty"`
	Periods       *PatientBGMPeriods    `json:"periods,omitempty"`
	TotalHours    *int                  `json:"totalHours,omitempty"`
}

type PatientCGMPeriod struct {
	AverageDailyRecords             *float64 `json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta        *float64 `json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol              *float64 `json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta         *float64 `json:"averageGlucoseMmolDelta,omitempty"`
	GlucoseManagementIndicator      *float64 `json:"glucoseManagementIndicator,omitempty"`
	GlucoseManagementIndicatorDelta *float64 `json:"glucoseManagementIndicatorDelta,omitempty"`
	HasAverageDailyRecords          *bool    `json:"hasAverageDailyRecords,omitempty"`
	HasAverageGlucoseMmol           *bool    `json:"hasAverageGlucoseMmol,omitempty"`
	HasGlucoseManagementIndicator   *bool    `json:"hasGlucoseManagementIndicator,omitempty"`
	HasTimeCGMUseMinutes            *bool    `json:"hasTimeCGMUseMinutes,omitempty"`
	HasTimeCGMUsePercent            *bool    `json:"hasTimeCGMUsePercent,omitempty"`
	HasTimeCGMUseRecords            *bool    `json:"hasTimeCGMUseRecords,omitempty"`
	HasTimeInHighMinutes            *bool    `json:"hasTimeInHighMinutes,omitempty"`
	HasTimeInHighPercent            *bool    `json:"hasTimeInHighPercent,omitempty"`
	HasTimeInHighRecords            *bool    `json:"hasTimeInHighRecords,omitempty"`
	HasTimeInLowMinutes             *bool    `json:"hasTimeInLowMinutes,omitempty"`
	HasTimeInLowPercent             *bool    `json:"hasTimeInLowPercent,omitempty"`
	HasTimeInLowRecords             *bool    `json:"hasTimeInLowRecords,omitempty"`
	HasTimeInTargetMinutes          *bool    `json:"hasTimeInTargetMinutes,omitempty"`
	HasTimeInTargetPercent          *bool    `json:"hasTimeInTargetPercent,omitempty"`
	HasTimeInTargetRecords          *bool    `json:"hasTimeInTargetRecords,omitempty"`
	HasTimeInVeryHighMinutes        *bool    `json:"hasTimeInVeryHighMinutes,omitempty"`
	HasTimeInVeryHighPercent        *bool    `json:"hasTimeInVeryHighPercent,omitempty"`
	HasTimeInVeryHighRecords        *bool    `json:"hasTimeInVeryHighRecords,omitempty"`
	HasTimeInVeryLowMinutes         *bool    `json:"hasTimeInVeryLowMinutes,omitempty"`
	HasTimeInVeryLowPercent         *bool    `json:"hasTimeInVeryLowPercent,omitempty"`
	HasTimeInVeryLowRecords         *bool    `json:"hasTimeInVeryLowRecords,omitempty"`
	HasTotalRecords                 *bool    `json:"hasTotalRecords,omitempty"`
	TimeCGMUseMinutes               *int     `json:"timeCGMUseMinutes,omitempty"`
	TimeCGMUseMinutesDelta          *int     `json:"timeCGMUseMinutesDelta,omitempty"`
	TimeCGMUsePercent               *float64 `json:"timeCGMUsePercent,omitempty"`
	TimeCGMUsePercentDelta          *float64 `json:"timeCGMUsePercentDelta,omitempty"`
	TimeCGMUseRecords               *int     `json:"timeCGMUseRecords,omitempty"`
	TimeCGMUseRecordsDelta          *int     `json:"timeCGMUseRecordsDelta,omitempty"`
	TimeInHighMinutes               *int     `json:"timeInHighMinutes,omitempty"`
	TimeInHighMinutesDelta          *int     `json:"timeInHighMinutesDelta,omitempty"`
	TimeInHighPercent               *float64 `json:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta          *float64 `json:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords               *int     `json:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta          *int     `json:"timeInHighRecordsDelta,omitempty"`
	TimeInLowMinutes                *int     `json:"timeInLowMinutes,omitempty"`
	TimeInLowMinutesDelta           *int     `json:"timeInLowMinutesDelta,omitempty"`
	TimeInLowPercent                *float64 `json:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta           *float64 `json:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords                *int     `json:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta           *int     `json:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetMinutes             *int     `json:"timeInTargetMinutes,omitempty"`
	TimeInTargetMinutesDelta        *int     `json:"timeInTargetMinutesDelta,omitempty"`
	TimeInTargetPercent             *float64 `json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta        *float64 `json:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords             *int     `json:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta        *int     `json:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighMinutes           *int     `json:"timeInVeryHighMinutes,omitempty"`
	TimeInVeryHighMinutesDelta      *int     `json:"timeInVeryHighMinutesDelta,omitempty"`
	TimeInVeryHighPercent           *float64 `json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta      *float64 `json:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords           *int     `json:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta      *int     `json:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowMinutes            *int     `json:"timeInVeryLowMinutes,omitempty"`
	TimeInVeryLowMinutesDelta       *int     `json:"timeInVeryLowMinutesDelta,omitempty"`
	TimeInVeryLowPercent            *float64 `json:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta       *float64 `json:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords            *int     `json:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta       *int     `json:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords                    *int     `json:"totalRecords,omitempty"`
	TotalRecordsDelta               *int     `json:"totalRecordsDelta,omitempty"`
}

type PatientCGMPeriods map[string]PatientCGMPeriod

type PatientCGMStats struct {
	Config        *PatientSummaryConfig `json:"config,omitempty"`
	Dates         *PatientSummaryDates  `json:"dates,omitempty"`
	OffsetPeriods *PatientCGMPeriods    `json:"offsetPeriods,omitempty"`
	Periods       *PatientCGMPeriods    `json:"periods,omitempty"`
	TotalHours    *int                  `json:"totalHours,omitempty"`
}

type PatientSummaryConfig struct {
	HighGlucoseThreshold     *float64 `json:"highGlucoseThreshold,omitempty"`
	LowGlucoseThreshold      *float64 `json:"lowGlucoseThreshold,omitempty"`
	SchemaVersion            *int     `json:"schemaVersion,omitempty"`
	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold,omitempty"`
	VeryLowGlucoseThreshold  *float64 `json:"veryLowGlucoseThreshold,omitempty"`
}

// PatientSummaryDates dates tracked for summary calculation
type PatientSummaryDates struct {
	FirstData         *time.Time `json:"firstData,omitempty"`
	HasFirstData      *bool      `json:"hasFirstData,omitempty"`
	HasLastData       *bool      `json:"hasLastData,omitempty"`
	HasLastUploadDate *bool      `json:"hasLastUploadDate,omitempty"`
	HasOutdatedSince  *bool      `json:"hasOutdatedSince,omitempty"`
	LastData          *time.Time `json:"lastData,omitempty"`
	LastUpdatedDate   *time.Time `json:"lastUpdatedDate,omitempty"`
	LastUploadDate    *time.Time `json:"lastUploadDate,omitempty"`
	OutdatedSince     *time.Time `json:"outdatedSince,omitempty"`
}
