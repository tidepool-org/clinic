package patients

import "time"

type PatientBGMPeriod struct {
	AverageDailyRecords           *float64 `bson:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta      *float64 `bson:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol            *float64 `bson:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta       *float64 `bson:"averageGlucoseMmolDelta,omitempty"`
	HasAverageDailyRecords        bool     `bson:"hasAverageDailyRecords"`
	HasAverageGlucoseMmol         bool     `bson:"hasAverageGlucoseMmol"`
	HasTimeInAnyHighPercent       bool     `bson:"hasTimeInAnyHighPercent"`
	HasTimeInAnyHighRecords       bool     `bson:"hasTimeInAnyHighRecords"`
	HasTimeInAnyLowPercent        bool     `bson:"hasTimeInAnyLowPercent"`
	HasTimeInAnyLowRecords        bool     `bson:"hasTimeInAnyLowRecords"`
	HasTimeInExtremeHighPercent   bool     `bson:"hasTimeInExtremeHighPercent"`
	HasTimeInExtremeHighRecords   bool     `bson:"hasTimeInExtremeHighRecords"`
	HasTimeInHighPercent          bool     `bson:"hasTimeInHighPercent"`
	HasTimeInHighRecords          bool     `bson:"hasTimeInHighRecords"`
	HasTimeInLowPercent           bool     `bson:"hasTimeInLowPercent"`
	HasTimeInLowRecords           bool     `bson:"hasTimeInLowRecords"`
	HasTimeInTargetPercent        bool     `bson:"hasTimeInTargetPercent"`
	HasTimeInTargetRecords        bool     `bson:"hasTimeInTargetRecords"`
	HasTimeInVeryHighPercent      bool     `bson:"hasTimeInVeryHighPercent"`
	HasTimeInVeryHighRecords      bool     `bson:"hasTimeInVeryHighRecords"`
	HasTimeInVeryLowPercent       bool     `bson:"hasTimeInVeryLowPercent"`
	HasTimeInVeryLowRecords       bool     `bson:"hasTimeInVeryLowRecords"`
	HasTotalRecords               bool     `bson:"hasTotalRecords"`
	TimeInAnyHighPercent          *float64 `bson:"timeInAnyHighPercent,omitempty"`
	TimeInAnyHighPercentDelta     *float64 `bson:"timeInAnyHighPercentDelta,omitempty"`
	TimeInAnyHighRecords          *int     `bson:"timeInAnyHighRecords,omitempty"`
	TimeInAnyHighRecordsDelta     *int     `bson:"timeInAnyHighRecordsDelta,omitempty"`
	TimeInAnyLowPercent           *float64 `bson:"timeInAnyLowPercent,omitempty"`
	TimeInAnyLowPercentDelta      *float64 `bson:"timeInAnyLowPercentDelta,omitempty"`
	TimeInAnyLowRecords           *int     `bson:"timeInAnyLowRecords,omitempty"`
	TimeInAnyLowRecordsDelta      *int     `bson:"timeInAnyLowRecordsDelta,omitempty"`
	TimeInExtremeHighPercent      *float64 `bson:"timeInExtremeHighPercent,omitempty"`
	TimeInExtremeHighPercentDelta *float64 `bson:"timeInExtremeHighPercentDelta,omitempty"`
	TimeInExtremeHighRecords      *int     `bson:"timeInExtremeHighRecords,omitempty"`
	TimeInExtremeHighRecordsDelta *int     `bson:"timeInExtremeHighRecordsDelta,omitempty"`
	TimeInHighPercent             *float64 `bson:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta        *float64 `bson:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords             *int     `bson:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta        *int     `bson:"timeInHighRecordsDelta,omitempty"`
	TimeInLowPercent              *float64 `bson:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta         *float64 `bson:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords              *int     `bson:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta         *int     `bson:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetPercent           *float64 `bson:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta      *float64 `bson:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords           *int     `bson:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta      *int     `bson:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighPercent         *float64 `bson:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta    *float64 `bson:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords         *int     `bson:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta    *int     `bson:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowPercent          *float64 `bson:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta     *float64 `bson:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords          *int     `bson:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta     *int     `bson:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords                  *int     `bson:"totalRecords,omitempty"`
	TotalRecordsDelta             *int     `bson:"totalRecordsDelta,omitempty"`
}

type PatientBGMPeriods map[string]PatientBGMPeriod

type PatientBGMStats struct {
	Config        PatientSummaryConfig `bson:"config,omitempty" json:"config,omitempty"`
	Dates         PatientSummaryDates  `bson:"dates,omitempty" json:"dates,omitempty"`
	OffsetPeriods PatientBGMPeriods    `bson:"offsetPeriods,omitempty" json:"offsetPeriods,omitempty"`
	Periods       PatientBGMPeriods    `bson:"periods,omitempty" json:"periods,omitempty"`
	TotalHours    int                  `bson:"totalHours" json:"totalHours"`
}

func (s *PatientBGMStats) GetLastUploadDate() time.Time {
	last := time.Time{}
	if s.Dates.LastUploadDate != nil {
		last = *s.Dates.LastUploadDate
	}
	return last
}

func (s *PatientBGMStats) GetLastUpdatedDate() time.Time {
	last := time.Time{}
	if s.Dates.LastUpdatedDate != nil {
		last = *s.Dates.LastUpdatedDate
	}
	return last
}

// PatientCGMPeriod Summary of a specific CGM time period (currently: 1d, 7d, 14d, 30d)
type PatientCGMPeriod struct {
	AverageDailyRecords             *float64 `bson:"averageDailyRecords,omitempty"`
	AverageDailyRecordsDelta        *float64 `bson:"averageDailyRecordsDelta,omitempty"`
	AverageGlucoseMmol              *float64 `bson:"averageGlucoseMmol,omitempty"`
	AverageGlucoseMmolDelta         *float64 `bson:"averageGlucoseMmolDelta,omitempty"`
	CoefficientOfVariation          float64  `bson:"coefficientOfVariation"`
	CoefficientOfVariationDelta     float64  `bson:"coefficientOfVariationDelta"`
	DaysWithData                    int      `bson:"daysWithData"`
	DaysWithDataDelta               int      `bson:"daysWithDataDelta"`
	GlucoseManagementIndicator      *float64 `bson:"glucoseManagementIndicator,omitempty"`
	GlucoseManagementIndicatorDelta *float64 `bson:"glucoseManagementIndicatorDelta,omitempty"`
	HasAverageDailyRecords          bool     `bson:"hasAverageDailyRecords"`
	HasAverageGlucoseMmol           bool     `bson:"hasAverageGlucoseMmol"`
	HasGlucoseManagementIndicator   bool     `bson:"hasGlucoseManagementIndicator"`
	HasTimeCGMUseMinutes            bool     `bson:"hasTimeCGMUseMinutes"`
	HasTimeCGMUsePercent            bool     `bson:"hasTimeCGMUsePercent"`
	HasTimeCGMUseRecords            bool     `bson:"hasTimeCGMUseRecords"`
	HasTimeInAnyHighMinutes         bool     `bson:"hasTimeInAnyHighMinutes"`
	HasTimeInAnyHighPercent         bool     `bson:"hasTimeInAnyHighPercent"`
	HasTimeInAnyHighRecords         bool     `bson:"hasTimeInAnyHighRecords"`
	HasTimeInAnyLowMinutes          bool     `bson:"hasTimeInAnyLowMinutes"`
	HasTimeInAnyLowPercent          bool     `bson:"hasTimeInAnyLowPercent"`
	HasTimeInAnyLowRecords          bool     `bson:"hasTimeInAnyLowRecords"`
	HasTimeInExtremeHighMinutes     bool     `bson:"hasTimeInExtremeHighMinutes"`
	HasTimeInExtremeHighPercent     bool     `bson:"hasTimeInExtremeHighPercent"`
	HasTimeInExtremeHighRecords     bool     `bson:"hasTimeInExtremeHighRecords"`
	HasTimeInHighMinutes            bool     `bson:"hasTimeInHighMinutes"`
	HasTimeInHighPercent            bool     `bson:"hasTimeInHighPercent"`
	HasTimeInHighRecords            bool     `bson:"hasTimeInHighRecords"`
	HasTimeInLowMinutes             bool     `bson:"hasTimeInLowMinutes"`
	HasTimeInLowPercent             bool     `bson:"hasTimeInLowPercent"`
	HasTimeInLowRecords             bool     `bson:"hasTimeInLowRecords"`
	HasTimeInTargetMinutes          bool     `bson:"hasTimeInTargetMinutes"`
	HasTimeInTargetPercent          bool     `bson:"hasTimeInTargetPercent"`
	HasTimeInTargetRecords          bool     `bson:"hasTimeInTargetRecords"`
	HasTimeInVeryHighMinutes        bool     `bson:"hasTimeInVeryHighMinutes"`
	HasTimeInVeryHighPercent        bool     `bson:"hasTimeInVeryHighPercent"`
	HasTimeInVeryHighRecords        bool     `bson:"hasTimeInVeryHighRecords"`
	HasTimeInVeryLowMinutes         bool     `bson:"hasTimeInVeryLowMinutes"`
	HasTimeInVeryLowPercent         bool     `bson:"hasTimeInVeryLowPercent"`
	HasTimeInVeryLowRecords         bool     `bson:"hasTimeInVeryLowRecords"`
	HasTotalRecords                 bool     `bson:"hasTotalRecords"`
	HoursWithData                   int      `bson:"hoursWithData"`
	HoursWithDataDelta              int      `bson:"hoursWithDataDelta"`
	StandardDeviation               float64  `bson:"standardDeviation"`
	StandardDeviationDelta          float64  `bson:"standardDeviationDelta"`
	TimeCGMUseMinutes               *int     `bson:"timeCGMUseMinutes,omitempty"`
	TimeCGMUseMinutesDelta          *int     `bson:"timeCGMUseMinutesDelta,omitempty"`
	TimeCGMUsePercent               *float64 `bson:"timeCGMUsePercent,omitempty"`
	TimeCGMUsePercentDelta          *float64 `bson:"timeCGMUsePercentDelta,omitempty"`
	TimeCGMUseRecords               *int     `bson:"timeCGMUseRecords,omitempty"`
	TimeCGMUseRecordsDelta          *int     `bson:"timeCGMUseRecordsDelta,omitempty"`
	TimeInAnyHighMinutes            *int     `bson:"timeInAnyHighMinutes,omitempty"`
	TimeInAnyHighMinutesDelta       *int     `bson:"timeInAnyHighMinutesDelta,omitempty"`
	TimeInAnyHighPercent            *float64 `bson:"timeInAnyHighPercent,omitempty"`
	TimeInAnyHighPercentDelta       *float64 `bson:"timeInAnyHighPercentDelta,omitempty"`
	TimeInAnyHighRecords            *int     `bson:"timeInAnyHighRecords,omitempty"`
	TimeInAnyHighRecordsDelta       *int     `bson:"timeInAnyHighRecordsDelta,omitempty"`
	TimeInAnyLowMinutes             *int     `bson:"timeInAnyLowMinutes,omitempty"`
	TimeInAnyLowMinutesDelta        *int     `bson:"timeInAnyLowMinutesDelta,omitempty"`
	TimeInAnyLowPercent             *float64 `bson:"timeInAnyLowPercent,omitempty"`
	TimeInAnyLowPercentDelta        *float64 `bson:"timeInAnyLowPercentDelta,omitempty"`
	TimeInAnyLowRecords             *int     `bson:"timeInAnyLowRecords,omitempty"`
	TimeInAnyLowRecordsDelta        *int     `bson:"timeInAnyLowRecordsDelta,omitempty"`
	TimeInExtremeHighMinutes        *int     `bson:"timeInExtremeHighMinutes,omitempty"`
	TimeInExtremeHighMinutesDelta   *int     `bson:"timeInExtremeHighMinutesDelta,omitempty"`
	TimeInExtremeHighPercent        *float64 `bson:"timeInExtremeHighPercent,omitempty"`
	TimeInExtremeHighPercentDelta   *float64 `bson:"timeInExtremeHighPercentDelta,omitempty"`
	TimeInExtremeHighRecords        *int     `bson:"timeInExtremeHighRecords,omitempty"`
	TimeInExtremeHighRecordsDelta   *int     `bson:"timeInExtremeHighRecordsDelta,omitempty"`
	TimeInHighMinutes               *int     `bson:"timeInHighMinutes,omitempty"`
	TimeInHighMinutesDelta          *int     `bson:"timeInHighMinutesDelta,omitempty"`
	TimeInHighPercent               *float64 `bson:"timeInHighPercent,omitempty"`
	TimeInHighPercentDelta          *float64 `bson:"timeInHighPercentDelta,omitempty"`
	TimeInHighRecords               *int     `bson:"timeInHighRecords,omitempty"`
	TimeInHighRecordsDelta          *int     `bson:"timeInHighRecordsDelta,omitempty"`
	TimeInLowMinutes                *int     `bson:"timeInLowMinutes,omitempty"`
	TimeInLowMinutesDelta           *int     `bson:"timeInLowMinutesDelta,omitempty"`
	TimeInLowPercent                *float64 `bson:"timeInLowPercent,omitempty"`
	TimeInLowPercentDelta           *float64 `bson:"timeInLowPercentDelta,omitempty"`
	TimeInLowRecords                *int     `bson:"timeInLowRecords,omitempty"`
	TimeInLowRecordsDelta           *int     `bson:"timeInLowRecordsDelta,omitempty"`
	TimeInTargetMinutes             *int     `bson:"timeInTargetMinutes,omitempty"`
	TimeInTargetMinutesDelta        *int     `bson:"timeInTargetMinutesDelta,omitempty"`
	TimeInTargetPercent             *float64 `bson:"timeInTargetPercent,omitempty"`
	TimeInTargetPercentDelta        *float64 `bson:"timeInTargetPercentDelta,omitempty"`
	TimeInTargetRecords             *int     `bson:"timeInTargetRecords,omitempty"`
	TimeInTargetRecordsDelta        *int     `bson:"timeInTargetRecordsDelta,omitempty"`
	TimeInVeryHighMinutes           *int     `bson:"timeInVeryHighMinutes,omitempty"`
	TimeInVeryHighMinutesDelta      *int     `bson:"timeInVeryHighMinutesDelta,omitempty"`
	TimeInVeryHighPercent           *float64 `bson:"timeInVeryHighPercent,omitempty"`
	TimeInVeryHighPercentDelta      *float64 `bson:"timeInVeryHighPercentDelta,omitempty"`
	TimeInVeryHighRecords           *int     `bson:"timeInVeryHighRecords,omitempty"`
	TimeInVeryHighRecordsDelta      *int     `bson:"timeInVeryHighRecordsDelta,omitempty"`
	TimeInVeryLowMinutes            *int     `bson:"timeInVeryLowMinutes,omitempty"`
	TimeInVeryLowMinutesDelta       *int     `bson:"timeInVeryLowMinutesDelta,omitempty"`
	TimeInVeryLowPercent            *float64 `bson:"timeInVeryLowPercent,omitempty"`
	TimeInVeryLowPercentDelta       *float64 `bson:"timeInVeryLowPercentDelta,omitempty"`
	TimeInVeryLowRecords            *int     `bson:"timeInVeryLowRecords,omitempty"`
	TimeInVeryLowRecordsDelta       *int     `bson:"timeInVeryLowRecordsDelta,omitempty"`
	TotalRecords                    *int     `bson:"totalRecords,omitempty"`
	TotalRecordsDelta               *int     `bson:"totalRecordsDelta,omitempty"`
}

type PatientCGMPeriods map[string]PatientCGMPeriod

type PatientCGMStats struct {
	Config        PatientSummaryConfig `bson:"config,omitempty" json:"config,omitempty"`
	Dates         PatientSummaryDates  `bson:"dates,omitempty" json:"dates,omitempty"`
	OffsetPeriods PatientCGMPeriods    `bson:"offsetPeriods,omitempty" json:"offsetPeriods,omitempty"`
	Periods       PatientCGMPeriods    `bson:"periods,omitempty" json:"periods,omitempty"`
	TotalHours    int                  `bson:"totalHours" json:"totalHours"`
}

func (s *PatientCGMStats) GetLastUploadDate() time.Time {
	last := time.Time{}
	if s.Dates.LastUploadDate != nil {
		last = *s.Dates.LastUploadDate
	}
	return last
}

func (s *PatientCGMStats) GetLastUpdatedDate() time.Time {
	last := time.Time{}
	if s.Dates.LastUpdatedDate != nil {
		last = *s.Dates.LastUpdatedDate
	}
	return last
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
