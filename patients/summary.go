package patients

import "time"

type PatientBGMPeriod struct {
	AverageDailyRecords        *float64 `json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta   *float64 `json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol         *float64 `json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta    *float64 `json:"averageGlucoseMmolDelta,omitempty"`
	HasAverageDailyRecords     bool     `json:"hasAverageDailyRecords"`
	HasAverageGlucoseMmol      bool     `json:"hasAverageGlucoseMmol"`
	HasTimeInAnyHighPercent    bool     `json:"hasTimeInAnyHighPercent"`
	HasTimeInAnyHighRecords    bool     `json:"hasTimeInAnyHighRecords"`
	HasTimeInAnyLowPercent     bool     `json:"hasTimeInAnyLowPercent"`
	HasTimeInAnyLowRecords     bool     `json:"hasTimeInAnyLowRecords"`
	HasTimeInHighPercent       bool     `json:"hasTimeInHighPercent"`
	HasTimeInHighRecords       bool     `json:"hasTimeInHighRecords"`
	HasTimeInLowPercent        bool     `json:"hasTimeInLowPercent"`
	HasTimeInLowRecords        bool     `json:"hasTimeInLowRecords"`
	HasTimeInTargetPercent     bool     `json:"hasTimeInTargetPercent"`
	HasTimeInTargetRecords     bool     `json:"hasTimeInTargetRecords"`
	HasTimeInVeryHighPercent   bool     `json:"hasTimeInVeryHighPercent"`
	HasTimeInVeryHighRecords   bool     `json:"hasTimeInVeryHighRecords"`
	HasTimeInVeryLowPercent    bool     `json:"hasTimeInVeryLowPercent"`
	HasTimeInVeryLowRecords    bool     `json:"hasTimeInVeryLowRecords"`
	HasTotalRecords            bool     `json:"hasTotalRecords"`
	TimeInAnyHighPercent       *float64 `json:"timeInAnyHighPercent,omitempty"`
	TimeInAnyHighPercentDelta  *float64 `json:"timeInAnyHighPercentDelta,omitempty"`
	TimeInAnyHighRecords       *int     `json:"timeInAnyHighRecords,omitempty"`
	TimeInAnyHighRecordsDelta  *int     `json:"timeInAnyHighRecordsDelta,omitempty"`
	TimeInAnyLowPercent        *float64 `json:"timeInAnyLowPercent,omitempty"`
	TimeInAnyLowPercentDelta   *float64 `json:"timeInAnyLowPercentDelta,omitempty"`
	TimeInAnyLowRecords        *int     `json:"timeInAnyLowRecords,omitempty"`
	TimeInAnyLowRecordsDelta   *int     `json:"timeInAnyLowRecordsDelta,omitempty"`
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
	Config        PatientSummaryConfig `json:"config,omitempty"`
	Dates         PatientSummaryDates  `json:"dates,omitempty"`
	OffsetPeriods PatientBGMPeriods    `json:"offsetPeriods,omitempty"`
	Periods       PatientBGMPeriods    `json:"periods,omitempty"`
	TotalHours    int                  `json:"totalHours"`
}

type PatientCGMPeriod struct {
	AverageDailyRecords             *float64 `json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta        *float64 `json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol              *float64 `json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta         *float64 `json:"averageGlucoseMmolDelta,omitempty"`
	GlucoseManagementIndicator      *float64 `json:"glucoseManagementIndicator,omitempty"`
	GlucoseManagementIndicatorDelta *float64 `json:"glucoseManagementIndicatorDelta,omitempty"`
	HasAverageDailyRecords          bool     `json:"hasAverageDailyRecords"`
	HasAverageGlucoseMmol           bool     `json:"hasAverageGlucoseMmol"`
	HasGlucoseManagementIndicator   bool     `json:"hasGlucoseManagementIndicator"`
	HasTimeCGMUseMinutes            bool     `json:"hasTimeCGMUseMinutes"`
	HasTimeCGMUsePercent            bool     `json:"hasTimeCGMUsePercent"`
	HasTimeCGMUseRecords            bool     `json:"hasTimeCGMUseRecords"`
	HasTimeInAnyHighMinutes         bool     `json:"hasTimeInAnyHighMinutes"`
	HasTimeInAnyHighPercent         bool     `json:"hasTimeInAnyHighPercent"`
	HasTimeInAnyHighRecords         bool     `json:"hasTimeInAnyHighRecords"`
	HasTimeInAnyLowMinutes          bool     `json:"hasTimeInAnyLowMinutes"`
	HasTimeInAnyLowPercent          bool     `json:"hasTimeInAnyLowPercent"`
	HasTimeInAnyLowRecords          bool     `json:"hasTimeInAnyLowRecords"`
	HasTimeInHighMinutes            bool     `json:"hasTimeInHighMinutes"`
	HasTimeInHighPercent            bool     `json:"hasTimeInHighPercent"`
	HasTimeInHighRecords            bool     `json:"hasTimeInHighRecords"`
	HasTimeInLowMinutes             bool     `json:"hasTimeInLowMinutes"`
	HasTimeInLowPercent             bool     `json:"hasTimeInLowPercent"`
	HasTimeInLowRecords             bool     `json:"hasTimeInLowRecords"`
	HasTimeInTargetMinutes          bool     `json:"hasTimeInTargetMinutes"`
	HasTimeInTargetPercent          bool     `json:"hasTimeInTargetPercent"`
	HasTimeInTargetRecords          bool     `json:"hasTimeInTargetRecords"`
	HasTimeInVeryHighMinutes        bool     `json:"hasTimeInVeryHighMinutes"`
	HasTimeInVeryHighPercent        bool     `json:"hasTimeInVeryHighPercent"`
	HasTimeInVeryHighRecords        bool     `json:"hasTimeInVeryHighRecords"`
	HasTimeInVeryLowMinutes         bool     `json:"hasTimeInVeryLowMinutes"`
	HasTimeInVeryLowPercent         bool     `json:"hasTimeInVeryLowPercent"`
	HasTimeInVeryLowRecords         bool     `json:"hasTimeInVeryLowRecords"`
	HasTotalRecords                 bool     `json:"hasTotalRecords"`
	TimeCGMUseMinutes               *int     `json:"timeCGMUseMinutes,omitempty"`
	TimeCGMUseMinutesDelta          *int     `json:"timeCGMUseMinutesDelta,omitempty"`
	TimeCGMUsePercent               *float64 `json:"timeCGMUsePercent,omitempty"`
	TimeCGMUsePercentDelta          *float64 `json:"timeCGMUsePercentDelta,omitempty"`
	TimeCGMUseRecords               *int     `json:"timeCGMUseRecords,omitempty"`
	TimeCGMUseRecordsDelta          *int     `json:"timeCGMUseRecordsDelta,omitempty"`
	TimeInAnyHighMinutes            *int     `json:"timeInAnyHighMinutes,omitempty"`
	TimeInAnyHighMinutesDelta       *int     `json:"timeInAnyHighMinutesDelta,omitempty"`
	TimeInAnyHighPercent            *float64 `json:"timeInAnyHighPercent,omitempty"`
	TimeInAnyHighPercentDelta       *float64 `json:"timeInAnyHighPercentDelta,omitempty"`
	TimeInAnyHighRecords            *int     `json:"timeInAnyHighRecords,omitempty"`
	TimeInAnyHighRecordsDelta       *int     `json:"timeInAnyHighRecordsDelta,omitempty"`
	TimeInAnyLowMinutes             *int     `json:"timeInAnyLowMinutes,omitempty"`
	TimeInAnyLowMinutesDelta        *int     `json:"timeInAnyLowMinutesDelta,omitempty"`
	TimeInAnyLowPercent             *float64 `json:"timeInAnyLowPercent,omitempty"`
	TimeInAnyLowPercentDelta        *float64 `json:"timeInAnyLowPercentDelta,omitempty"`
	TimeInAnyLowRecords             *int     `json:"timeInAnyLowRecords,omitempty"`
	TimeInAnyLowRecordsDelta        *int     `json:"timeInAnyLowRecordsDelta,omitempty"`
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
	Config        PatientSummaryConfig `json:"config,omitempty"`
	Dates         PatientSummaryDates  `json:"dates,omitempty"`
	OffsetPeriods PatientCGMPeriods    `json:"offsetPeriods,omitempty"`
	Periods       PatientCGMPeriods    `json:"periods,omitempty"`
	TotalHours    int                  `json:"totalHours"`
}

type PatientSummary struct {
	BgmStats *PatientBGMStats `json:"bgmStats,omitempty"`
	CgmStats *PatientCGMStats `json:"cgmStats,omitempty"`
}

type PatientSummaryConfig struct {
	HighGlucoseThreshold     float64 `json:"highGlucoseThreshold"`
	LowGlucoseThreshold      float64 `json:"lowGlucoseThreshold"`
	SchemaVersion            int     `json:"schemaVersion"`
	VeryHighGlucoseThreshold float64 `json:"veryHighGlucoseThreshold"`
	VeryLowGlucoseThreshold  float64 `json:"veryLowGlucoseThreshold"`
}

// PatientSummaryDates dates tracked for summary calculation
type PatientSummaryDates struct {
	FirstData          *time.Time `json:"firstData,omitempty"`
	HasFirstData       bool       `json:"hasFirstData"`
	HasLastData        bool       `json:"hasLastData"`
	HasLastUploadDate  bool       `json:"hasLastUploadDate"`
	HasOutdatedSince   bool       `json:"hasOutdatedSince"`
	LastData           *time.Time `json:"lastData,omitempty"`
	LastUpdatedDate    *time.Time `json:"lastUpdatedDate,omitempty"`
	LastUpdatedReason  *[]string  `json:"lastUpdatedReason,omitempty"`
	LastUploadDate     *time.Time `json:"lastUploadDate,omitempty"`
	OutdatedReason     *[]string  `json:"outdatedReason,omitempty"`
	OutdatedSince      *time.Time `json:"outdatedSince,omitempty"`
	OutdatedSinceLimit *time.Time `json:"outdatedSinceLimit,omitempty"`
}
