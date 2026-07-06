// Package ehr computes summary statistics for EHR writeback (Redox
// flowsheets, Xealth FHIR observations, etc.) from a patient summary.
//
// Compute reads the patient summary directly and returns an ordered list of
// fully-formatted []Statistic: percentages are scaled, glucose is unit
// converted, values are rounded per the (legacy Redox) precision/ICode rules,
// and not-available metrics are dropped. Both EHR destinations consume the same
// formatted output — Redox writes Value verbatim; Xealth parses Value according
// to Kind.
package ehr

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tidepool-org/clinic/patients"
)

const (
	// MmolLToMgdLConversionFactor converts mmol/L glucose values to mg/dL.
	MmolLToMgdLConversionFactor float64 = 18.01559
	// MmolLToMgdLPrecisionFactor controls the rounding precision applied during
	// mmol/L -> mg/dL conversion.
	MmolLToMgdLPrecisionFactor float64 = 100000.0

	// summaryStatsPeriod is the reporting period selected for EHR writeback.
	summaryStatsPeriod = "14d"

	days14 = 14 * 24 * time.Hour

	missingValue = "NOT AVAILABLE"
	percentage   = "%"
	day          = "day"
	hour         = "hour"
)

// GlucoseUnits is the destination glucose unit. Summary values are always
// stored in mmol/L; Compute converts to mg/dL when the destination requests it.
type GlucoseUnits string

const (
	MmolL GlucoseUnits = "mmol/L"
	MgdL  GlucoseUnits = "mg/dL"
)

// ValueKind describes how a Statistic's formatted Value should be interpreted.
type ValueKind int

const (
	KindDecimal ValueKind = iota // a (possibly rounded) decimal number
	KindInteger                  // a whole-number count
	KindDate                     // an RFC3339 date-time
)

// Statistic is a single fully-formatted summary metric, ready to write to an
// EHR. Value is the formatted string (e.g. "56.2871", "143", "20.0", or an
// RFC3339 timestamp); Kind tells consumers how to interpret it. DateTime is the
// reporting time (when the metric was observed).
type Statistic struct {
	Code        string
	Display     string // human-readable metric name
	Description string
	Value       string
	Unit        string // "", "%", "mg/dL", "mmol/L", "day", "hour"
	Kind        ValueKind
	DateTime    string
}

// ValueType returns the coarse value type used by flowsheet consumers
// ("DateTime" for dates, otherwise "Numeric").
func (s Statistic) ValueType() string {
	if s.Kind == KindDate {
		return "DateTime"
	}
	return "Numeric"
}

// Compute returns the ordered, fully-formatted list of summary statistics: the
// CGM block followed by the BGM block, in the order EHR consumers expect today.
// A block is included only when its summary carries non-zero last-updated and
// last-data dates; icode selects the ICode2 rounding rules.
func Compute(summary *patients.Summary, dest GlucoseUnits, icode bool) []Statistic {
	if summary == nil {
		return nil
	}
	if dest == "" {
		dest = MmolL
	}

	var stats []Statistic
	if cgm := summary.CGM; cgm != nil && summaryDatesAvailable(cgm.Dates) {
		stats = append(stats, computeCGM(cgm, dest, icode)...)
	}
	if bgm := summary.BGM; bgm != nil && summaryDatesAvailable(bgm.Dates) {
		stats = append(stats, computeBGM(bgm, dest, icode)...)
	}
	return stats
}

func computeCGM(stats *patients.PatientCGMStats, dest GlucoseUnits, icode bool) []Statistic {
	var period *patients.PatientCGMPeriod
	reportingTime := formatTime(stats.Dates.LastUpdatedDate)
	var firstData, periodEnd, periodStart *time.Time

	if v, ok := stats.Periods[summaryStatsPeriod]; ok {
		period = &v
	}

	firstData = stats.Dates.FirstData
	periodEnd = stats.Dates.LastData
	if periodEnd != nil {
		start := periodEnd.Add(-days14)
		periodStart = &start
		if firstData != nil && firstData.Before(start) {
			firstData = periodStart
		}
	}

	unitsPercentage := percentage
	unitsDay := day
	unitsHour := hour
	sourceGlucoseUnits := string(MmolL)
	destGlucoseUnits := string(dest)

	var cgmUsePercent *float64
	var averageGlucose *float64
	var gmi *float64
	var cgmStdDev *float64
	var cgmCoeffVar *float64
	var cgmDaysWithData *int
	var cgmHoursWithData *int
	var timeInVeryLow *float64
	var timeInLow *float64
	var timeInTarget *float64
	var timeInHigh *float64
	var timeInVeryHigh *float64

	if period != nil {
		if period.AverageGlucoseMmol != nil {
			var val float64
			// Convert blood glucose to preferred units, store unit result overriding preference if unit is not convertable
			val, destGlucoseUnits = bgInUnits(*period.AverageGlucoseMmol, sourceGlucoseUnits, destGlucoseUnits)
			averageGlucose = &val
		}

		cgmStdDevVal, _ := bgInUnits(period.StandardDeviation, sourceGlucoseUnits, destGlucoseUnits)
		cgmStdDev = &cgmStdDevVal

		cgmUsePercent = period.TimeCGMUsePercent
		cgmCoeffVar = &period.CoefficientOfVariation
		cgmDaysWithData = &period.DaysWithData
		cgmHoursWithData = &period.HoursWithData
		gmi = period.GlucoseManagementIndicator
		timeInVeryLow = period.TimeInVeryLowPercent
		timeInLow = period.TimeInLowPercent
		timeInTarget = period.TimeInTargetPercent
		timeInHigh = period.TimeInHighPercent
		timeInVeryHigh = period.TimeInVeryHighPercent
	}

	observations := []Statistic{
		{"REPORTING_PERIOD_START_CGM", "Reporting Period Start CGM", "CGM Reporting Period Start", formatTime(periodStart), "", KindDate, reportingTime},
		{"REPORTING_PERIOD_END_CGM", "Reporting Period End CGM", "CGM Reporting Period End", formatTime(periodEnd), "", KindDate, reportingTime},
		{"REPORTING_PERIOD_START_CGM_DATA", "Reporting Period Start CGM Data", "CGM Reporting Period Start Date of actual Data", formatTime(firstData), "", KindDate, reportingTime},
		{"TIME_ABOVE_RANGE_VERY_HIGH_CGM", "Time Above Range Very High CGM", "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)", formatFloat(unitIntervalToPercent(timeInVeryHigh)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_ABOVE_RANGE_HIGH_CGM", "Time Above Range High CGM", "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)", formatFloat(unitIntervalToPercent(timeInHigh)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_IN_RANGE_CGM", "Time In Range CGM", "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)", formatFloat(unitIntervalToPercent(timeInTarget)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_BELOW_RANGE_LOW_CGM", "Time Below Range Low CGM", "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)", formatFloat(unitIntervalToPercent(timeInLow)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_BELOW_RANGE_VERY_LOW_CGM", "Time Below Range Very Low CGM", "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)", formatFloat(unitIntervalToPercent(timeInVeryLow)), unitsPercentage, KindDecimal, reportingTime},
		{"GLUCOSE_MANAGEMENT_INDICATOR", "Glucose Management Indicator", "CGM Glucose Management Indicator during reporting period", formatFloat(gmi), "", KindDecimal, reportingTime},
		{"AVERAGE_CGM", "Average CGM", "CGM Average Glucose during reporting period", formatFloat(averageGlucose), destGlucoseUnits, KindDecimal, reportingTime},
		{"STANDARD_DEVIATION_CGM", "Standard Deviation CGM", "The standard deviation of CGM measurements during the reporting period", formatFloat(cgmStdDev), destGlucoseUnits, KindDecimal, reportingTime},
		{"COEFFICIENT_OF_VARIATION_CGM", "Coefficient Of Variation CGM", "The coefficient of variation (standard deviation * 100 / mean) of CGM measurements during the reporting period", formatFloat(cgmCoeffVar), "", KindDecimal, reportingTime},
		{"ACTIVE_WEAR_TIME_CGM", "Active Wear Time CGM", "Percentage of time CGM worn during reporting period", formatFloat(unitIntervalToPercent(cgmUsePercent)), unitsPercentage, KindDecimal, reportingTime},
		{"DAYS_WITH_DATA_CGM", "Days With Data CGM", "Number of days with at least one CGM datum during the reporting period", formatInt(cgmDaysWithData), unitsDay, KindInteger, reportingTime},
		{"HOURS_WITH_DATA_CGM", "Hours With Data CGM", "Number of hours with at least one CGM datum during the reporting period", formatInt(cgmHoursWithData), unitsHour, KindInteger, reportingTime},
	}

	observationsMap := map[string]*Statistic{}
	for i := range observations {
		observationsMap[observations[i].Code] = &observations[i]
	}

	// For clinics flagged as icode, replace certain values with alternative formatting, as defined in BACK-3476
	if icode {
		observationsMap["COEFFICIENT_OF_VARIATION_CGM"].Value = formatFloatWithPrecision(unitIntervalToPercent(cgmCoeffVar), 1)
		observationsMap["COEFFICIENT_OF_VARIATION_CGM"].Unit = unitsPercentage

		// ICode2 defines whole-number precision for average glucose, this is only accurate enough for mg/dl
		if strings.ToLower(string(dest)) == "mg/dl" {
			observationsMap["AVERAGE_CGM"].Value = formatFloatConditionalPrecision(averageGlucose)
		} else {
			observationsMap["AVERAGE_CGM"].Value = formatFloatWithPrecision(averageGlucose, 1)
		}

		observationsMap["GLUCOSE_MANAGEMENT_INDICATOR"].Value = formatFloatWithPrecision(gmi, 1)
		observationsMap["ACTIVE_WEAR_TIME_CGM"].Value = formatFloatWithPrecision(unitIntervalToPercent(cgmUsePercent), 2)
		observationsMap["STANDARD_DEVIATION_CGM"].Value = formatFloatWithPrecision(cgmStdDev, 1)
		observationsMap["TIME_BELOW_RANGE_VERY_LOW_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryLow))
		observationsMap["TIME_BELOW_RANGE_LOW_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInLow))
		observationsMap["TIME_IN_RANGE_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInTarget))
		observationsMap["TIME_ABOVE_RANGE_HIGH_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInHigh))
		observationsMap["TIME_ABOVE_RANGE_VERY_HIGH_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryHigh))
	}

	return slices.DeleteFunc(observations, func(o Statistic) bool {
		return o.Value == missingValue
	})
}

func computeBGM(stats *patients.PatientBGMStats, dest GlucoseUnits, icode bool) []Statistic {
	var period *patients.PatientBGMPeriod
	reportingTime := formatTime(stats.Dates.LastUpdatedDate)
	var firstData, periodEnd, periodStart *time.Time

	if v, ok := stats.Periods[summaryStatsPeriod]; ok {
		period = &v
	}

	firstData = stats.Dates.FirstData
	periodEnd = stats.Dates.LastData
	if periodEnd != nil {
		start := periodEnd.Add(-days14)
		periodStart = &start
		if firstData != nil && firstData.Before(start) {
			firstData = periodStart
		}
	}

	unitsDay := day
	unitsPercentage := percentage
	sourceGlucoseUnits := string(MmolL)
	destGlucoseUnits := string(dest)

	var averageDailyRecords *float64
	var averageGlucose *float64
	var timeInVeryLowRecords *int
	var timeInVeryHighRecords *int
	var timeInVeryLowPercent *float64
	var timeInLowPercent *float64
	var timeInTargetPercent *float64
	var timeInHighPercent *float64
	var timeInVeryHighPercent *float64
	var bgmStdDev *float64
	var bgmCoeffVar *float64
	var bgmDaysWithData *int
	var bgmTotalRecords *int
	var minGlucose *float64
	var maxGlucose *float64

	if period != nil {
		if period.AverageGlucoseMmol != nil {
			// Convert blood glucose to preferred units, store unit result overriding preference if unit is not convertable
			averageGlucoseVal, units := bgInUnits(*period.AverageGlucoseMmol, sourceGlucoseUnits, destGlucoseUnits)
			averageGlucose = &averageGlucoseVal
			destGlucoseUnits = units
		}

		if period.StandardDeviation != nil {
			// Convert standard deviation to preferred units
			bgmStdDevVal, _ := bgInUnits(*period.StandardDeviation, sourceGlucoseUnits, destGlucoseUnits)
			bgmStdDev = &bgmStdDevVal
		}

		// Convert min glucose to preferred units
		minGlucoseVal, _ := bgInUnits(period.Min, sourceGlucoseUnits, destGlucoseUnits)
		minGlucose = &minGlucoseVal

		// Convert max glucose to preferred units
		maxGlucoseVal, _ := bgInUnits(period.Max, sourceGlucoseUnits, destGlucoseUnits)
		maxGlucose = &maxGlucoseVal

		averageDailyRecords = period.AverageDailyRecords
		timeInVeryLowRecords = period.TimeInVeryLowRecords
		timeInVeryHighRecords = period.TimeInVeryHighRecords
		bgmCoeffVar = period.CoefficientOfVariation
		bgmDaysWithData = &period.DaysWithData
		bgmTotalRecords = period.TotalRecords
		timeInVeryLowPercent = period.TimeInVeryLowPercent
		timeInLowPercent = period.TimeInLowPercent
		timeInTargetPercent = period.TimeInTargetPercent
		timeInHighPercent = period.TimeInHighPercent
		timeInVeryHighPercent = period.TimeInVeryHighPercent
	}

	observations := []Statistic{
		{"REPORTING_PERIOD_START_SMBG", "Reporting Period Start SMBG", "SMBG Reporting Period Start", formatTime(periodStart), "", KindDate, reportingTime},
		{"REPORTING_PERIOD_END_SMBG", "Reporting Period End SMBG", "SMBG Reporting Period End", formatTime(periodEnd), "", KindDate, reportingTime},
		{"REPORTING_PERIOD_START_SMBG_DATA", "Reporting Period Start SMBG Data", "SMBG Reporting Period Start Date of actual Data", formatTime(firstData), "", KindDate, reportingTime},
		{"TIME_ABOVE_RANGE_VERY_HIGH_SMBG", "Time Above Range Very High SMBG", "% of readings > 250 mg/dL (>13.9 mmol/L)", formatFloat(unitIntervalToPercent(timeInVeryHighPercent)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_ABOVE_RANGE_HIGH_SMBG", "Time Above Range High SMBG", "% of readings between 181–250 mg/dL (10.1–13.9 mmol/L)", formatFloat(unitIntervalToPercent(timeInHighPercent)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_IN_RANGE_SMBG", "Time In Range SMBG", "% of readings between 70–180 mg/dL (3.9–10.0 mmol/L)", formatFloat(unitIntervalToPercent(timeInTargetPercent)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_BELOW_RANGE_LOW_SMBG", "Time Below Range Low SMBG", "% of readings between 54–69 mg/dL (3.0–3.8 mmol/L)", formatFloat(unitIntervalToPercent(timeInLowPercent)), unitsPercentage, KindDecimal, reportingTime},
		{"TIME_BELOW_RANGE_VERY_LOW_SMBG", "Time Below Range Very Low SMBG", "% of readings < 54 mg/dL (<3.0 mmol/L)", formatFloat(unitIntervalToPercent(timeInVeryLowPercent)), unitsPercentage, KindDecimal, reportingTime},
		{"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "Readings Above Range Very High SMBG", "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period", formatInt(timeInVeryHighRecords), "", KindInteger, reportingTime},
		{"READINGS_BELOW_RANGE_VERY_LOW_SMBG", "Readings Below Range Very Low SMBG", "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period", formatInt(timeInVeryLowRecords), "", KindInteger, reportingTime},
		{"MAX_SMBG", "Max SMBG", "Maximum blood glucose reading over the time period", formatFloat(maxGlucose), destGlucoseUnits, KindDecimal, reportingTime},
		{"MIN_SMBG", "Min SMBG", "Minimum blood glucose reading over the time period", formatFloat(minGlucose), destGlucoseUnits, KindDecimal, reportingTime},
		{"AVERAGE_SMBG", "Average SMBG", "SMBG Average Glucose during reporting period", formatFloat(averageGlucose), destGlucoseUnits, KindDecimal, reportingTime},
		{"STANDARD_DEVIATION_SMBG", "Standard Deviation SMBG", "The standard deviation of SMBG measurements during the reporting period", formatFloat(bgmStdDev), destGlucoseUnits, KindDecimal, reportingTime},
		{"COEFFICIENT_OF_VARIATION_SMBG", "Coefficient Of Variation SMBG", "The coefficient of variation (standard deviation * 100 / mean) of SMBG measurements during the reporting period", formatFloat(bgmCoeffVar), "", KindDecimal, reportingTime},
		{"TOTAL_READING_COUNT_SMBG", "Total Reading Count SMBG", "The total number of SMBG readings taken during the SMBG Reporting Period", formatInt(bgmTotalRecords), "", KindInteger, reportingTime},
		{"CHECK_RATE_READINGS_DAY_SMBG", "Check Rate Readings Day SMBG", "Average Numeric of SMBG readings per day during reporting period", formatFloat(averageDailyRecords), "", KindDecimal, reportingTime},
		{"DAYS_WITH_DATA_SMBG", "Days With Data SMBG", "The total number of days with at least 1 SMBG reading over the reporting period", formatInt(bgmDaysWithData), unitsDay, KindInteger, reportingTime},
	}

	observationsMap := map[string]*Statistic{}
	for i := range observations {
		observationsMap[observations[i].Code] = &observations[i]
	}

	// For clinics flagged as icode, replace certain values with alternative formatting, as defined in BACK-3476
	if icode {
		observationsMap["COEFFICIENT_OF_VARIATION_SMBG"].Value = formatFloatWithPrecision(unitIntervalToPercent(bgmCoeffVar), 1)
		observationsMap["COEFFICIENT_OF_VARIATION_SMBG"].Unit = unitsPercentage

		// ICode2 defines whole-number precision for glucose, this is only accurate enough for mg/dl
		if strings.ToLower(string(dest)) == "mg/dl" {
			observationsMap["AVERAGE_SMBG"].Value = formatFloatConditionalPrecision(averageGlucose)
			observationsMap["MIN_SMBG"].Value = formatFloatConditionalPrecision(minGlucose)
			observationsMap["MAX_SMBG"].Value = formatFloatConditionalPrecision(maxGlucose)
		} else {
			observationsMap["AVERAGE_SMBG"].Value = formatFloatWithPrecision(averageGlucose, 1)
			observationsMap["MIN_SMBG"].Value = formatFloatWithPrecision(minGlucose, 1)
			observationsMap["MAX_SMBG"].Value = formatFloatWithPrecision(maxGlucose, 1)
		}

		observationsMap["STANDARD_DEVIATION_SMBG"].Value = formatFloatWithPrecision(bgmStdDev, 1)
		observationsMap["TIME_BELOW_RANGE_VERY_LOW_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryLowPercent))
		observationsMap["TIME_BELOW_RANGE_LOW_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInLowPercent))
		observationsMap["TIME_IN_RANGE_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInTargetPercent))
		observationsMap["TIME_ABOVE_RANGE_HIGH_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInHighPercent))
		observationsMap["TIME_ABOVE_RANGE_VERY_HIGH_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryHighPercent))
	}

	return slices.DeleteFunc(observations, func(o Statistic) bool {
		return o.Value == missingValue
	})
}

func formatTime(t *time.Time) string {
	if t == nil {
		return missingValue
	}
	return t.Format(time.RFC3339)
}

func formatInt(val *int) string {
	if val == nil {
		return missingValue
	}
	return fmt.Sprintf("%d", *val)
}

func formatFloat(val *float64) string {
	return formatFloatWithPrecision(val, 4)
}

func formatFloatWithPrecision(val *float64, decimalPlaces int) string {
	if val == nil {
		return missingValue
	}
	return fmt.Sprintf("%.*f", decimalPlaces, *val)
}

// formatFloatConditionalPrecision conditionally removes the decimal only if the number is <1.
func formatFloatConditionalPrecision(val *float64) string {
	if val == nil {
		return missingValue
	}
	if *val < 1 {
		return formatFloatWithPrecision(val, 1)
	}
	return formatFloatWithPrecision(val, 0)
}

// unitIntervalToPercent converts a unit interval (0.0 - 1.0) to a percentage (0.0 - 100.0)
func unitIntervalToPercent(val *float64) *float64 {
	if val == nil {
		return nil
	}
	res := *val * 100
	return &res
}

func bgInUnits(val float64, sourceUnits string, targetUnits string) (float64, string) {
	if strings.EqualFold(sourceUnits, string(MmolL)) && strings.EqualFold(targetUnits, string(MgdL)) {
		intValue := int(val*MmolLToMgdLConversionFactor*MmolLToMgdLPrecisionFactor + 0.5)
		floatValue := float64(intValue) / MmolLToMgdLPrecisionFactor
		return floatValue, targetUnits
	}

	return val, sourceUnits
}

// summaryDatesAvailable reports whether a summary block carries the non-zero
// last-updated and last-data dates required for EHR writeback.
func summaryDatesAvailable(d patients.PatientSummaryDates) bool {
	return d.LastUpdatedDate != nil && d.LastData != nil &&
		!d.LastUpdatedDate.IsZero() && !d.LastData.IsZero()
}
