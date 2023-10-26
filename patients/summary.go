package patients

import "time"

type PatientBGMPeriod struct {
	AverageDailyRecords        *float64 `bson:"averageDailyRecords,omitempty"  json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta   *float64 `bson:"averageDailyRecordsDelta,omitempty"  json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol         *float64 `bson:"averageGlucoseMmol,omitempty"  json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta    *float64 `bson:"averageGlucoseMmolDelta,omitempty"  json:"averageGlucoseMmolDelta,omitempty"`
	HasAverageDailyRecords     *bool    `bson:"hasAverageDailyRecords,omitempty"  json:"hasAverageDailyRecords,omitempty"`
	HasAverageGlucoseMmol      *bool    `bson:"hasAverageGlucoseMmol,omitempty"  json:"hasAverageGlucoseMmol,omitempty"`
	HasTimeInHighPercent       *bool    `bson:"hasTimeInHighPercent,omitempty"  json:"hasTimeInHighPercent,omitempty"`
	HasTimeInHighRecords       *bool    `bson:"hasTimeInHighRecords,omitempty"  json:"hasTimeInHighRecords,omitempty"`
	HasTimeInLowPercent        *bool    `bson:"hasTimeInLowPercent,omitempty"  json:"hasTimeInLowPercent,omitempty"`
	HasTimeInLowRecords        *bool    `bson:"hasTimeInLowRecords,omitempty"  json:"hasTimeInLowRecords,omitempty"`
	HasTimeInTargetPercent     *bool    `bson:"hasTimeInTargetPercent,omitempty"  json:"hasTimeInTargetPercent,omitempty"`
	HasTimeInTargetRecords     *bool    `bson:"hasTimeInTargetRecords,omitempty"  json:"hasTimeInTargetRecords,omitempty"`
	HasTimeInVeryHighPercent   *bool    `bson:"hasTimeInVeryHighPercent,omitempty"  json:"hasTimeInVeryHighPercent,omitempty"`
	HasTimeInVeryHighRecords   *bool    `bson:"hasTimeInVeryHighRecords,omitempty"  json:"hasTimeInVeryHighRecords,omitempty"`
	HasTimeInVeryLowPercent    *bool    `bson:"hasTimeInVeryLowPercent,omitempty"  json:"hasTimeInVeryLowPercent,omitempty"`
	HasTimeInVeryLowRecords    *bool    `bson:"hasTimeInVeryLowRecords,omitempty"  json:"hasTimeInVeryLowRecords,omitempty"`
	HasTotalRecords            *bool    `bson:"hasTotalRecords,omitempty"  json:"hasTotalRecords,omitempty"`
	TimeInHighPercent          *float64 `bson:"timeInHighPercent,omitempty"  json:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta     *float64 `bson:"timeInHighPercentDelta,omitempty"  json:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords          *int     `bson:"timeInHighRecords,omitempty"  json:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta     *int     `bson:"timeInHighRecordsDelta,omitempty"  json:"timeInHighRecordsDelta,omitempty"`
	TimeInLowPercent           *float64 `bson:"timeInLowPercent,omitempty"  json:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta      *float64 `bson:"timeInLowPercentDelta,omitempty"  json:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords           *int     `bson:"timeInLowRecords,omitempty"  json:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta      *int     `bson:"timeInLowRecordsDelta,omitempty"  json:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetPercent        *float64 `bson:"timeInTargetPercent,omitempty"  json:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta   *float64 `bson:"timeInTargetPercentDelta,omitempty"  json:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords        *int     `bson:"timeInTargetRecords,omitempty"  json:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta   *int     `bson:"timeInTargetRecordsDelta,omitempty"  json:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighPercent      *float64 `bson:"timeInVeryHighPercent,omitempty"  json:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta *float64 `bson:"timeInVeryHighPercentDelta,omitempty" json:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords      *int     `bson:"timeInVeryHighRecords,omitempty"  json:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta *int     `bson:"timeInVeryHighRecordsDelta,omitempty" json:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowPercent       *float64 `bson:"timeInVeryLowPercent,omitempty"  json:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta  *float64 `bson:"timeInVeryLowPercentDelta,omitempty"  json:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords       *int     `bson:"timeInVeryLowRecords,omitempty"  json:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta  *int     `bson:"timeInVeryLowRecordsDelta,omitempty"  json:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords               *int     `bson:"totalRecords,omitempty"  json:"totalRecords,omitempty"`
	TotalRecordsDelta          *int     `bson:"totalRecordsDelta,omitempty"  json:"totalRecordsDelta,omitempty"`
}

type PatientBGMPeriods map[string]PatientBGMPeriod

type PatientBGMStats struct {
	Config        *PatientSummaryConfig `bson:"config,omitempty" json:"config,omitempty"`
	Dates         *PatientSummaryDates  `bson:"dates,omitempty" json:"dates,omitempty"`
	OffsetPeriods *PatientBGMPeriods    `bson:"offsetPeriods,omitempty" json:"offsetPeriods,omitempty"`
	Periods       *PatientBGMPeriods    `bson:"periods,omitempty" json:"periods,omitempty"`
	TotalHours    *int                  `bson:"totalHours,omitempty" json:"totalHours,omitempty"`
}

type PatientCGMPeriod struct {
	AverageDailyRecords             *float64 `bson:"averageDailyRecords,omitempty" json:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta        *float64 `bson:"averageDailyRecordsDelta,omitempty" json:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol              *float64 `bson:"averageGlucoseMmol,omitempty" json:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta         *float64 `bson:"averageGlucoseMmolDelta,omitempty" json:"averageGlucoseMmolDelta,omitempty"`
	GlucoseManagementIndicator      *float64 `bson:"glucoseManagementIndicator,omitempty" json:"glucoseManagementIndicator,omitempty"`
	GlucoseManagementIndicatorDelta *float64 `bson:"glucoseManagementIndicatorDelta,omitempty"json:"glucoseManagementIndicatorDelta,omitempty"`
	HasAverageDailyRecords          *bool    `bson:"hasAverageDailyRecords,omitempty" json:"hasAverageDailyRecords,omitempty"`
	HasAverageGlucoseMmol           *bool    `bson:"hasAverageGlucoseMmol,omitempty" json:"hasAverageGlucoseMmol,omitempty"`
	HasGlucoseManagementIndicator   *bool    `bson:"hasGlucoseManagementIndicator,omitempty" json:"hasGlucoseManagementIndicator,omitempty"`
	HasTimeCGMUseMinutes            *bool    `bson:"hasTimeCGMUseMinutes,omitempty" json:"hasTimeCGMUseMinutes,omitempty"`
	HasTimeCGMUsePercent            *bool    `bson:"hasTimeCGMUsePercent,omitempty" json:"hasTimeCGMUsePercent,omitempty"`
	HasTimeCGMUseRecords            *bool    `bson:"hasTimeCGMUseRecords,omitempty" json:"hasTimeCGMUseRecords,omitempty"`
	HasTimeInHighMinutes            *bool    `bson:"hasTimeInHighMinutes,omitempty" json:"hasTimeInHighMinutes,omitempty"`
	HasTimeInHighPercent            *bool    `bson:"hasTimeInHighPercent,omitempty" json:"hasTimeInHighPercent,omitempty"`
	HasTimeInHighRecords            *bool    `bson:"hasTimeInHighRecords,omitempty" json:"hasTimeInHighRecords,omitempty"`
	HasTimeInLowMinutes             *bool    `bson:"hasTimeInLowMinutes,omitempty" json:"hasTimeInLowMinutes,omitempty"`
	HasTimeInLowPercent             *bool    `bson:"hasTimeInLowPercent,omitempty" json:"hasTimeInLowPercent,omitempty"`
	HasTimeInLowRecords             *bool    `bson:"hasTimeInLowRecords,omitempty" json:"hasTimeInLowRecords,omitempty"`
	HasTimeInTargetMinutes          *bool    `bson:"hasTimeInTargetMinutes,omitempty" json:"hasTimeInTargetMinutes,omitempty"`
	HasTimeInTargetPercent          *bool    `bson:"hasTimeInTargetPercent,omitempty" json:"hasTimeInTargetPercent,omitempty"`
	HasTimeInTargetRecords          *bool    `bson:"hasTimeInTargetRecords,omitempty" json:"hasTimeInTargetRecords,omitempty"`
	HasTimeInVeryHighMinutes        *bool    `bson:"hasTimeInVeryHighMinutes,omitempty" json:"hasTimeInVeryHighMinutes,omitempty"`
	HasTimeInVeryHighPercent        *bool    `bson:"hasTimeInVeryHighPercent,omitempty" json:"hasTimeInVeryHighPercent,omitempty"`
	HasTimeInVeryHighRecords        *bool    `bson:"hasTimeInVeryHighRecords,omitempty" json:"hasTimeInVeryHighRecords,omitempty"`
	HasTimeInVeryLowMinutes         *bool    `bson:"hasTimeInVeryLowMinutes,omitempty" json:"hasTimeInVeryLowMinutes,omitempty"`
	HasTimeInVeryLowPercent         *bool    `bson:"hasTimeInVeryLowPercent,omitempty" json:"hasTimeInVeryLowPercent,omitempty"`
	HasTimeInVeryLowRecords         *bool    `bson:"hasTimeInVeryLowRecords,omitempty" json:"hasTimeInVeryLowRecords,omitempty"`
	HasTotalRecords                 *bool    `bson:"hasTotalRecords,omitempty" json:"hasTotalRecords,omitempty"`
	TimeCGMUseMinutes               *int     `bson:"timeCGMUseMinutes,omitempty" json:"timeCGMUseMinutes,omitempty"`
	TimeCGMUseMinutesDelta          *int     `bson:"timeCGMUseMinutesDelta,omitempty" json:"timeCGMUseMinutesDelta,omitempty"`
	TimeCGMUsePercent               *float64 `bson:"timeCGMUsePercent,omitempty" json:"timeCGMUsePercent,omitempty"`
	TimeCGMUsePercentDelta          *float64 `bson:"timeCGMUsePercentDelta,omitempty" json:"timeCGMUsePercentDelta,omitempty"`
	TimeCGMUseRecords               *int     `bson:"timeCGMUseRecords,omitempty" json:"timeCGMUseRecords,omitempty"`
	TimeCGMUseRecordsDelta          *int     `bson:"timeCGMUseRecordsDelta,omitempty" json:"timeCGMUseRecordsDelta,omitempty"`
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
	Config        *PatientSummaryConfig `bson:"config,omitempty" json:"config,omitempty"`
	Dates         *PatientSummaryDates  `bson:"dates,omitempty" json:"dates,omitempty"`
	OffsetPeriods *PatientCGMPeriods    `bson:"offsetPeriods,omitempty" json:"offsetPeriods,omitempty"`
	Periods       *PatientCGMPeriods    `bson:"periods,omitempty" json:"periods,omitempty"`
	TotalHours    *int                  `bson:"totalHours,omitempty" json:"totalHours,omitempty"`
}

type PatientSummaryConfig struct {
	HighGlucoseThreshold     *float64 `bson:"highGlucoseThreshold,omitempty" json:"highGlucoseThreshold,omitempty"`
	LowGlucoseThreshold      *float64 `bson:"lowGlucoseThreshold,omitempty" json:"lowGlucoseThreshold,omitempty"`
	SchemaVersion            *int     `bson:"schemaVersion,omitempty" json:"schemaVersion,omitempty"`
	VeryHighGlucoseThreshold *float64 `bson:"veryHighGlucoseThreshold,omitempty" json:"veryHighGlucoseThreshold,omitempty"`
	VeryLowGlucoseThreshold  *float64 `bson:"veryLowGlucoseThreshold,omitempty" json:"veryLowGlucoseThreshold,omitempty"`
}

// PatientSummaryDates dates tracked for summary calculation
//type PatientSummaryDates struct {
//	FirstData          *time.Time `bson:"firstData,omitempty" json:"firstData,omitempty"`
//	HasFirstData       *bool      `bson:"hasFirstData,omitempty" json:"hasFirstData,omitempty"`
//	HasLastData        *bool      `bson:"hasLastData,omitempty" json:"hasLastData,omitempty"`
//	HasLastUploadDate  *bool      `bson:"hasLastUploadDate,omitempty" json:"hasLastUploadDate,omitempty"`
//	HasOutdatedSince   *bool      `bson:"hasOutdatedSince,omitempty" json:"hasOutdatedSince,omitempty"`
//	LastData           *time.Time `bson:"lastData,omitempty" json:"lastData,omitempty"`
//	LastUpdatedDate    *time.Time `bson:"lastUpdatedDate,omitempty" json:"lastUpdatedDate,omitempty"`
//	LastUpdatedReason  *[]string  `bson:"lastUpdatedReason,omitempty" json:"lastUpdatedReason,omitempty"`
//	LastUploadDate     *time.Time `bson:"lastUploadDate,omitempty" json:"lastUploadDate,omitempty"`
//	OutdatedSince      *time.Time `bson:"outdatedSince,omitempty" json:"outdatedSince,omitempty"`
//	OutdatedReason     *[]string  `bson:"outdatedReason,omitempty" json:"outdatedReason,omitempty"`
//	OutdatedSinceLimit *time.Time `bson:"outdatedSinceLimit,omitempty" json:"outdatedSinceLimit,omitempty"`
//}

type PatientSummaryDates struct {
	// FirstData Date of the first included value
	FirstData         *time.Time `json:"firstData,omitempty"`
	HasFirstData      *bool      `json:"hasFirstData,omitempty"`
	HasLastData       *bool      `json:"hasLastData,omitempty"`
	HasLastUploadDate *bool      `json:"hasLastUploadDate,omitempty"`
	HasOutdatedSince  *bool      `json:"hasOutdatedSince,omitempty"`

	// LastData Date of the last calculated value
	LastData *time.Time `json:"lastData,omitempty"`

	// LastUpdatedDate Date of the last calculation
	LastUpdatedDate *time.Time `json:"lastUpdatedDate,omitempty"`

	// LastUpdatedReason List of reasons the summary was updated for
	LastUpdatedReason *[]string `json:"lastUpdatedReason,omitempty"`

	// LastUploadDate Created date of the last calculated value
	LastUploadDate *time.Time `json:"lastUploadDate,omitempty"`

	// OutdatedReason List of reasons the summary was marked outdated for
	OutdatedReason *[]string `json:"outdatedReason,omitempty"`

	// OutdatedSince Date of the first user upload after lastData, removed when calculated
	OutdatedSince *time.Time `json:"outdatedSince,omitempty"`

	// OutdatedSinceLimit Upper limit of the OutdatedSince value to prevent infinite queue duration
	OutdatedSinceLimit *time.Time `json:"outdatedSinceLimit,omitempty"`
}
