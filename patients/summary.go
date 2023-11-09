package patients

import "time"

type PatientBGMPeriod struct {
	AverageDailyRecords        *float64 `bson:"averageDailyRecords,omitempty" json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta   *float64 `bson:"averageDailyRecordsDelta,omitempty" json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol         *float64 `bson:"averageGlucoseMmol,omitempty" json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta    *float64 `bson:"averageGlucoseMmolDelta,omitempty" json:"averageGlucoseMmolDelta,omitempty"`
	HasAverageDailyRecords     bool     `bson:"hasAverageDailyRecords" json:"hasAverageDailyRecords"`
	HasAverageGlucoseMmol      bool     `bson:"hasAverageGlucoseMmol" json:"hasAverageGlucoseMmol"`
	HasTimeInAnyHighPercent    bool     `bson:"hasTimeInAnyHighPercent" json:"hasTimeInAnyHighPercent"`
	HasTimeInAnyHighRecords    bool     `bson:"hasTimeInAnyHighRecords" json:"hasTimeInAnyHighRecords"`
	HasTimeInAnyLowPercent     bool     `bson:"hasTimeInAnyLowPercent" json:"hasTimeInAnyLowPercent"`
	HasTimeInAnyLowRecords     bool     `bson:"hasTimeInAnyLowRecords" json:"hasTimeInAnyLowRecords"`
	HasTimeInHighPercent       bool     `bson:"hasTimeInHighPercent" json:"hasTimeInHighPercent"`
	HasTimeInHighRecords       bool     `bson:"hasTimeInHighRecords" json:"hasTimeInHighRecords"`
	HasTimeInLowPercent        bool     `bson:"hasTimeInLowPercent" json:"hasTimeInLowPercent"`
	HasTimeInLowRecords        bool     `bson:"hasTimeInLowRecords" json:"hasTimeInLowRecords"`
	HasTimeInTargetPercent     bool     `bson:"hasTimeInTargetPercent" json:"hasTimeInTargetPercent"`
	HasTimeInTargetRecords     bool     `bson:"hasTimeInTargetRecords" json:"hasTimeInTargetRecords"`
	HasTimeInVeryHighPercent   bool     `bson:"hasTimeInVeryHighPercent" json:"hasTimeInVeryHighPercent"`
	HasTimeInVeryHighRecords   bool     `bson:"hasTimeInVeryHighRecords" json:"hasTimeInVeryHighRecords"`
	HasTimeInVeryLowPercent    bool     `bson:"hasTimeInVeryLowPercent" json:"hasTimeInVeryLowPercent"`
	HasTimeInVeryLowRecords    bool     `bson:"hasTimeInVeryLowRecords" json:"hasTimeInVeryLowRecords"`
	HasTotalRecords            bool     `bson:"hasTotalRecords" json:"hasTotalRecords"`
	TimeInAnyHighPercent       *float64 `bson:"timeInAnyHighPercent,omitempty" json:"timeInAnyHighPercent,omitempty"`
	TimeInAnyHighPercentDelta  *float64 `bson:"timeInAnyHighPercentDelta,omitempty" json:"timeInAnyHighPercentDelta,omitempty"`
	TimeInAnyHighRecords       *int     `bson:"timeInAnyHighRecords,omitempty" json:"timeInAnyHighRecords,omitempty"`
	TimeInAnyHighRecordsDelta  *int     `bson:"timeInAnyHighRecordsDelta,omitempty" json:"timeInAnyHighRecordsDelta,omitempty"`
	TimeInAnyLowPercent        *float64 `bson:"timeInAnyLowPercent,omitempty" json:"timeInAnyLowPercent,omitempty"`
	TimeInAnyLowPercentDelta   *float64 `bson:"timeInAnyLowPercentDelta,omitempty" json:"timeInAnyLowPercentDelta,omitempty"`
	TimeInAnyLowRecords        *int     `bson:"timeInAnyLowRecords,omitempty" json:"timeInAnyLowRecords,omitempty"`
	TimeInAnyLowRecordsDelta   *int     `bson:"timeInAnyLowRecordsDelta,omitempty" json:"timeInAnyLowRecordsDelta,omitempty"`
	TimeInHighPercent          *float64 `bson:"timeInHighPercent,omitempty" json:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta     *float64 `bson:"timeInHighPercentDelta,omitempty" json:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords          *int     `bson:"timeInHighRecords,omitempty" json:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta     *int     `bson:"timeInHighRecordsDelta,omitempty" json:"timeInHighRecordsDelta,omitempty"`
	TimeInLowPercent           *float64 `bson:"timeInLowPercent,omitempty" json:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta      *float64 `bson:"timeInLowPercentDelta,omitempty" json:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords           *int     `bson:"timeInLowRecords,omitempty" json:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta      *int     `bson:"timeInLowRecordsDelta,omitempty" json:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetPercent        *float64 `bson:"timeInTargetPercent,omitempty" json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta   *float64 `bson:"timeInTargetPercentDelta,omitempty" json:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords        *int     `bson:"timeInTargetRecords,omitempty" json:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta   *int     `bson:"timeInTargetRecordsDelta,omitempty" json:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighPercent      *float64 `bson:"timeInVeryHighPercent,omitempty" json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta *float64 `bson:"timeInVeryHighPercentDelta,omitempty" json:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords      *int     `bson:"timeInVeryHighRecords,omitempty" json:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta *int     `bson:"timeInVeryHighRecordsDelta,omitempty" json:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowPercent       *float64 `bson:"timeInVeryLowPercent,omitempty" json:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta  *float64 `bson:"timeInVeryLowPercentDelta,omitempty" json:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords       *int     `bson:"timeInVeryLowRecords,omitempty" json:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta  *int     `bson:"timeInVeryLowRecordsDelta,omitempty" json:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords               *int     `bson:"totalRecords,omitempty" json:"totalRecords,omitempty"`
	TotalRecordsDelta          *int     `bson:"totalRecordsDelta,omitempty" json:"totalRecordsDelta,omitempty"`
}

type PatientBGMPeriods map[string]PatientBGMPeriod

type PatientBGMStats struct {
	Config        PatientSummaryConfig `bson:"config,omitempty" json:"config,omitempty"`
	Dates         PatientSummaryDates  `bson:"dates,omitempty" json:"dates,omitempty"`
	OffsetPeriods PatientBGMPeriods    `bson:"offsetPeriods,omitempty" json:"offsetPeriods,omitempty"`
	Periods       PatientBGMPeriods    `bson:"periods,omitempty" json:"periods,omitempty"`
	TotalHours    int                  `bson:"totalHours" json:"totalHours"`
}

type PatientCGMPeriod struct {
	AverageDailyRecords             *float64 `bson:"averageDailyRecords,omitempty" json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta        *float64 `bson:"averageDailyRecordsDelta,omitempty" json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol              *float64 `bson:"averageGlucoseMmol,omitempty" json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta         *float64 `bson:"averageGlucoseMmolDelta,omitempty" json:"averageGlucoseMmolDelta,omitempty"`
	GlucoseManagementIndicator      *float64 `bson:"glucoseManagementIndicator,omitempty" json:"glucoseManagementIndicator,omitempty"`
	GlucoseManagementIndicatorDelta *float64 `bson:"glucoseManagementIndicatorDelta,omitempty" json:"glucoseManagementIndicatorDelta,omitempty"`
	HasAverageDailyRecords          bool     `bson:"hasAverageDailyRecords" json:"hasAverageDailyRecords"`
	HasAverageGlucoseMmol           bool     `bson:"hasAverageGlucoseMmol" json:"hasAverageGlucoseMmol"`
	HasGlucoseManagementIndicator   bool     `bson:"hasGlucoseManagementIndicator" json:"hasGlucoseManagementIndicator"`
	HasTimeCGMUseMinutes            bool     `bson:"hasTimeCGMUseMinutes" json:"hasTimeCGMUseMinutes"`
	HasTimeCGMUsePercent            bool     `bson:"hasTimeCGMUsePercent" json:"hasTimeCGMUsePercent"`
	HasTimeCGMUseRecords            bool     `bson:"hasTimeCGMUseRecords" json:"hasTimeCGMUseRecords"`
	HasTimeInAnyHighMinutes         bool     `bson:"hasTimeInAnyHighMinutes" json:"hasTimeInAnyHighMinutes"`
	HasTimeInAnyHighPercent         bool     `bson:"hasTimeInAnyHighPercent" json:"hasTimeInAnyHighPercent"`
	HasTimeInAnyHighRecords         bool     `bson:"hasTimeInAnyHighRecords" json:"hasTimeInAnyHighRecords"`
	HasTimeInAnyLowMinutes          bool     `bson:"hasTimeInAnyLowMinutes" json:"hasTimeInAnyLowMinutes"`
	HasTimeInAnyLowPercent          bool     `bson:"hasTimeInAnyLowPercent" json:"hasTimeInAnyLowPercent"`
	HasTimeInAnyLowRecords          bool     `bson:"hasTimeInAnyLowRecords" json:"hasTimeInAnyLowRecords"`
	HasTimeInHighMinutes            bool     `bson:"hasTimeInHighMinutes" json:"hasTimeInHighMinutes"`
	HasTimeInHighPercent            bool     `bson:"hasTimeInHighPercent" json:"hasTimeInHighPercent"`
	HasTimeInHighRecords            bool     `bson:"hasTimeInHighRecords" json:"hasTimeInHighRecords"`
	HasTimeInLowMinutes             bool     `bson:"hasTimeInLowMinutes" json:"hasTimeInLowMinutes"`
	HasTimeInLowPercent             bool     `bson:"hasTimeInLowPercent" json:"hasTimeInLowPercent"`
	HasTimeInLowRecords             bool     `bson:"hasTimeInLowRecords" json:"hasTimeInLowRecords"`
	HasTimeInTargetMinutes          bool     `bson:"hasTimeInTargetMinutes" json:"hasTimeInTargetMinutes"`
	HasTimeInTargetPercent          bool     `bson:"hasTimeInTargetPercent" json:"hasTimeInTargetPercent"`
	HasTimeInTargetRecords          bool     `bson:"hasTimeInTargetRecords" json:"hasTimeInTargetRecords"`
	HasTimeInVeryHighMinutes        bool     `bson:"hasTimeInVeryHighMinutes" json:"hasTimeInVeryHighMinutes"`
	HasTimeInVeryHighPercent        bool     `bson:"hasTimeInVeryHighPercent" json:"hasTimeInVeryHighPercent"`
	HasTimeInVeryHighRecords        bool     `bson:"hasTimeInVeryHighRecords" json:"hasTimeInVeryHighRecords"`
	HasTimeInVeryLowMinutes         bool     `bson:"hasTimeInVeryLowMinutes" json:"hasTimeInVeryLowMinutes"`
	HasTimeInVeryLowPercent         bool     `bson:"hasTimeInVeryLowPercent" json:"hasTimeInVeryLowPercent"`
	HasTimeInVeryLowRecords         bool     `bson:"hasTimeInVeryLowRecords" json:"hasTimeInVeryLowRecords"`
	HasTotalRecords                 bool     `bson:"hasTotalRecords" json:"hasTotalRecords"`
	TimeCGMUseMinutes               *int     `bson:"timeCGMUseMinutes,omitempty" json:"timeCGMUseMinutes,omitempty"`
	TimeCGMUseMinutesDelta          *int     `bson:"timeCGMUseMinutesDelta,omitempty" json:"timeCGMUseMinutesDelta,omitempty"`
	TimeCGMUsePercent               *float64 `bson:"timeCGMUsePercent,omitempty" json:"timeCGMUsePercent,omitempty"`
	TimeCGMUsePercentDelta          *float64 `bson:"timeCGMUsePercentDelta,omitempty" json:"timeCGMUsePercentDelta,omitempty"`
	TimeCGMUseRecords               *int     `bson:"timeCGMUseRecords,omitempty" json:"timeCGMUseRecords,omitempty"`
	TimeCGMUseRecordsDelta          *int     `bson:"timeCGMUseRecordsDelta,omitempty" json:"timeCGMUseRecordsDelta,omitempty"`
	TimeInAnyHighMinutes            *int     `bson:"timeInAnyHighMinutes,omitempty" json:"timeInAnyHighMinutes,omitempty"`
	TimeInAnyHighMinutesDelta       *int     `bson:"timeInAnyHighMinutesDelta,omitempty" json:"timeInAnyHighMinutesDelta,omitempty"`
	TimeInAnyHighPercent            *float64 `bson:"timeInAnyHighPercent,omitempty" json:"timeInAnyHighPercent,omitempty"`
	TimeInAnyHighPercentDelta       *float64 `bson:"timeInAnyHighPercentDelta,omitempty" json:"timeInAnyHighPercentDelta,omitempty"`
	TimeInAnyHighRecords            *int     `bson:"timeInAnyHighRecords,omitempty" json:"timeInAnyHighRecords,omitempty"`
	TimeInAnyHighRecordsDelta       *int     `bson:"timeInAnyHighRecordsDelta,omitempty" json:"timeInAnyHighRecordsDelta,omitempty"`
	TimeInAnyLowMinutes             *int     `bson:"timeInAnyLowMinutes,omitempty" json:"timeInAnyLowMinutes,omitempty"`
	TimeInAnyLowMinutesDelta        *int     `bson:"timeInAnyLowMinutesDelta,omitempty" json:"timeInAnyLowMinutesDelta,omitempty"`
	TimeInAnyLowPercent             *float64 `bson:"timeInAnyLowPercent,omitempty" json:"timeInAnyLowPercent,omitempty"`
	TimeInAnyLowPercentDelta        *float64 `bson:"timeInAnyLowPercentDelta,omitempty" json:"timeInAnyLowPercentDelta,omitempty"`
	TimeInAnyLowRecords             *int     `bson:"timeInAnyLowRecords,omitempty" json:"timeInAnyLowRecords,omitempty"`
	TimeInAnyLowRecordsDelta        *int     `bson:"timeInAnyLowRecordsDelta,omitempty" json:"timeInAnyLowRecordsDelta,omitempty"`
	TimeInHighMinutes               *int     `bson:"timeInHighMinutes,omitempty" json:"timeInHighMinutes,omitempty"`
	TimeInHighMinutesDelta          *int     `bson:"timeInHighMinutesDelta,omitempty" json:"timeInHighMinutesDelta,omitempty"`
	TimeInHighPercent               *float64 `bson:"timeInHighPercent,omitempty" json:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta          *float64 `bson:"timeInHighPercentDelta,omitempty" json:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords               *int     `bson:"timeInHighRecords,omitempty" json:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta          *int     `bson:"timeInHighRecordsDelta,omitempty" json:"timeInHighRecordsDelta,omitempty"`
	TimeInLowMinutes                *int     `bson:"timeInLowMinutes,omitempty" json:"timeInLowMinutes,omitempty"`
	TimeInLowMinutesDelta           *int     `bson:"timeInLowMinutesDelta,omitempty" json:"timeInLowMinutesDelta,omitempty"`
	TimeInLowPercent                *float64 `bson:"timeInLowPercent,omitempty" json:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta           *float64 `bson:"timeInLowPercentDelta,omitempty" json:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords                *int     `bson:"timeInLowRecords,omitempty" json:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta           *int     `bson:"timeInLowRecordsDelta,omitempty" json:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetMinutes             *int     `bson:"timeInTargetMinutes,omitempty" json:"timeInTargetMinutes,omitempty"`
	TimeInTargetMinutesDelta        *int     `bson:"timeInTargetMinutesDelta,omitempty" json:"timeInTargetMinutesDelta,omitempty"`
	TimeInTargetPercent             *float64 `bson:"timeInTargetPercent,omitempty" json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta        *float64 `bson:"timeInTargetPercentDelta,omitempty" json:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords             *int     `bson:"timeInTargetRecords,omitempty" json:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta        *int     `bson:"timeInTargetRecordsDelta,omitempty" json:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighMinutes           *int     `bson:"timeInVeryHighMinutes,omitempty" json:"timeInVeryHighMinutes,omitempty"`
	TimeInVeryHighMinutesDelta      *int     `bson:"timeInVeryHighMinutesDelta,omitempty" json:"timeInVeryHighMinutesDelta,omitempty"`
	TimeInVeryHighPercent           *float64 `bson:"timeInVeryHighPercent,omitempty" json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta      *float64 `bson:"timeInVeryHighPercentDelta,omitempty" json:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords           *int     `bson:"timeInVeryHighRecords,omitempty" json:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta      *int     `bson:"timeInVeryHighRecordsDelta,omitempty" json:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowMinutes            *int     `bson:"timeInVeryLowMinutes,omitempty" json:"timeInVeryLowMinutes,omitempty"`
	TimeInVeryLowMinutesDelta       *int     `bson:"timeInVeryLowMinutesDelta,omitempty" json:"timeInVeryLowMinutesDelta,omitempty"`
	TimeInVeryLowPercent            *float64 `bson:"timeInVeryLowPercent,omitempty" json:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta       *float64 `bson:"timeInVeryLowPercentDelta,omitempty" json:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords            *int     `bson:"timeInVeryLowRecords,omitempty" json:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta       *int     `bson:"timeInVeryLowRecordsDelta,omitempty" json:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords                    *int     `bson:"totalRecords,omitempty" json:"totalRecords,omitempty"`
	TotalRecordsDelta               *int     `bson:"totalRecordsDelta,omitempty" json:"totalRecordsDelta,omitempty"`
}

type PatientCGMPeriods map[string]PatientCGMPeriod

type PatientCGMStats struct {
	Config        PatientSummaryConfig `bson:"config,omitempty" json:"config,omitempty"`
	Dates         PatientSummaryDates  `bson:"dates,omitempty" json:"dates,omitempty"`
	OffsetPeriods PatientCGMPeriods    `bson:"offsetPeriods,omitempty" json:"offsetPeriods,omitempty"`
	Periods       PatientCGMPeriods    `bson:"periods,omitempty" json:"periods,omitempty"`
	TotalHours    int                  `bson:"totalHours" json:"totalHours"`
}

type PatientSummary struct {
	BgmStats *PatientBGMStats `bson:"bgmStats,omitempty" json:"bgmStats,omitempty"`
	CgmStats *PatientCGMStats `bson:"cgmStats,omitempty" json:"cgmStats,omitempty"`
}

type PatientSummaryConfig struct {
	HighGlucoseThreshold     float64 `bson:"highGlucoseThreshold" json:"highGlucoseThreshold"`
	LowGlucoseThreshold      float64 `bson:"lowGlucoseThreshold" json:"lowGlucoseThreshold"`
	SchemaVersion            int     `bson:"schemaVersion" json:"schemaVersion"`
	VeryHighGlucoseThreshold float64 `bson:"veryHighGlucoseThreshold" json:"veryHighGlucoseThreshold"`
	VeryLowGlucoseThreshold  float64 `bson:"veryLowGlucoseThreshold" json:"veryLowGlucoseThreshold"`
}

// PatientSummaryDates dates tracked for summary calculation
type PatientSummaryDates struct {
	FirstData          *time.Time `bson:"firstData,omitempty" json:"firstData,omitempty"`
	HasFirstData       bool       `bson:"hasFirstData" json:"hasFirstData"`
	HasLastData        bool       `bson:"hasLastData" json:"hasLastData"`
	HasLastUploadDate  bool       `bson:"hasLastUploadDate" json:"hasLastUploadDate"`
	HasOutdatedSince   bool       `bson:"hasOutdatedSince" json:"hasOutdatedSince"`
	LastData           *time.Time `bson:"lastData,omitempty" json:"lastData,omitempty"`
	LastUpdatedDate    *time.Time `bson:"lastUpdatedDate,omitempty" json:"lastUpdatedDate,omitempty"`
	LastUpdatedReason  *[]string  `bson:"lastUpdatedReason,omitempty" json:"lastUpdatedReason,omitempty"`
	LastUploadDate     *time.Time `bson:"lastUploadDate,omitempty" json:"lastUploadDate,omitempty"`
	OutdatedReason     *[]string  `bson:"outdatedReason,omitempty" json:"outdatedReason,omitempty"`
	OutdatedSince      *time.Time `bson:"outdatedSince,omitempty" json:"outdatedSince,omitempty"`
	OutdatedSinceLimit *time.Time `bson:"outdatedSinceLimit,omitempty" json:"outdatedSinceLimit,omitempty"`
}
