package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/patients/postgres/sqlcgen"
	storepg "github.com/tidepool-org/clinic/store/postgres"
)

// upsertSummaries replaces the summary and period rows of a patient inside
// the snapshot transaction. Every mirror input is a complete Mongo document,
// so writing summaries as part of the patient snapshot cannot clobber state:
// a patient without a summary genuinely has none.
func upsertSummaries(ctx context.Context, q *sqlcgen.Queries, patientId string, summary *patients.Summary) error {
	if err := q.DeletePatientSummaries(ctx, patientId); err != nil {
		return err
	}
	if summary == nil {
		return nil
	}

	if cgm := summary.CGM; cgm != nil {
		if err := q.InsertPatientSummary(ctx, summaryParams(patientId, "cgm", cgm.Id, cgm.Config, cgm.Dates)); err != nil {
			return err
		}
		periods := make([]sqlcgen.InsertPatientSummaryPeriodParams, 0, len(cgm.Periods))
		for period, stats := range cgm.Periods {
			params := sqlcgen.InsertPatientSummaryPeriodParams{
				PatientID:   patientId,
				SummaryType: "cgm",
				Period:      period,
			}
			cgmPeriodParams(stats, &params)
			periods = append(periods, params)
		}
		if len(periods) > 0 {
			if err := storepg.ExecBatch(q.InsertPatientSummaryPeriod(ctx, periods)); err != nil {
				return err
			}
		}
	}

	if bgm := summary.BGM; bgm != nil {
		if err := q.InsertPatientSummary(ctx, summaryParams(patientId, "bgm", bgm.Id, bgm.Config, bgm.Dates)); err != nil {
			return err
		}
		periods := make([]sqlcgen.InsertPatientSummaryPeriodParams, 0, len(bgm.Periods))
		for period, stats := range bgm.Periods {
			params := sqlcgen.InsertPatientSummaryPeriodParams{
				PatientID:   patientId,
				SummaryType: "bgm",
				Period:      period,
			}
			bgmPeriodParams(stats, &params)
			periods = append(periods, params)
		}
		if len(periods) > 0 {
			if err := storepg.ExecBatch(q.InsertPatientSummaryPeriod(ctx, periods)); err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteSummariesBySummaryId removes the summary rows carrying the given
// platform summary id, mirroring DeleteSummaryInAllClinics.
func (w *Writer) DeleteSummariesBySummaryId(ctx context.Context, summaryId string) error {
	return w.queries.DeleteSummariesBySummaryId(ctx, summaryId)
}

func summaryParams(patientId, summaryType, summaryId string, config patients.PatientSummaryConfig, dates patients.PatientSummaryDates) sqlcgen.InsertPatientSummaryParams {
	params := sqlcgen.InsertPatientSummaryParams{
		PatientID:                      patientId,
		SummaryType:                    summaryType,
		SummaryID:                      summaryId,
		ConfigSchemaVersion:            int32(config.SchemaVersion),
		ConfigHighGlucoseThreshold:     config.HighGlucoseThreshold,
		ConfigLowGlucoseThreshold:      config.LowGlucoseThreshold,
		ConfigVeryHighGlucoseThreshold: config.VeryHighGlucoseThreshold,
		ConfigVeryLowGlucoseThreshold:  config.VeryLowGlucoseThreshold,
		DatesFirstData:                 storepg.TimestamptzValue(dates.FirstData),
		DatesHasFirstData:              dates.HasFirstData,
		DatesHasLastData:               dates.HasLastData,
		DatesHasLastUploadDate:         dates.HasLastUploadDate,
		DatesHasOutdatedSince:          dates.HasOutdatedSince,
		DatesLastData:                  storepg.TimestamptzValue(dates.LastData),
		DatesLastUpdatedDate:           storepg.TimestamptzValue(dates.LastUpdatedDate),
		DatesLastUploadDate:            storepg.TimestamptzValue(dates.LastUploadDate),
		DatesOutdatedSince:             storepg.TimestamptzValue(dates.OutdatedSince),
		DatesOutdatedSinceLimit:        storepg.TimestamptzValue(dates.OutdatedSinceLimit),
	}
	if dates.LastUpdatedReason != nil {
		params.DatesLastUpdatedReason = *dates.LastUpdatedReason
	}
	if dates.OutdatedReason != nil {
		params.DatesOutdatedReason = *dates.OutdatedReason
	}
	return params
}

func f64Value(v *float64) pgtype.Float8 {
	if v == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *v, Valid: true}
}

func intPtrValue(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func cgmPeriodParams(p patients.PatientCGMPeriod, params *sqlcgen.InsertPatientSummaryPeriodParams) {
	params.AverageDailyRecords = f64Value(p.AverageDailyRecords)
	params.AverageDailyRecordsDelta = f64Value(p.AverageDailyRecordsDelta)
	params.AverageGlucoseMmol = f64Value(p.AverageGlucoseMmol)
	params.AverageGlucoseMmolDelta = f64Value(p.AverageGlucoseMmolDelta)
	params.CoefficientOfVariation = pgtype.Float8{Float64: p.CoefficientOfVariation, Valid: true}
	params.CoefficientOfVariationDelta = pgtype.Float8{Float64: p.CoefficientOfVariationDelta, Valid: true}
	params.DaysWithData = pgtype.Int4{Int32: int32(p.DaysWithData), Valid: true}
	params.DaysWithDataDelta = pgtype.Int4{Int32: int32(p.DaysWithDataDelta), Valid: true}
	params.GlucoseManagementIndicator = f64Value(p.GlucoseManagementIndicator)
	params.GlucoseManagementIndicatorDelta = f64Value(p.GlucoseManagementIndicatorDelta)
	params.HasAverageDailyRecords = pgtype.Bool{Bool: p.HasAverageDailyRecords, Valid: true}
	params.HasAverageGlucoseMmol = pgtype.Bool{Bool: p.HasAverageGlucoseMmol, Valid: true}
	params.HasGlucoseManagementIndicator = pgtype.Bool{Bool: p.HasGlucoseManagementIndicator, Valid: true}
	params.HasTimeCgmUseMinutes = pgtype.Bool{Bool: p.HasTimeCGMUseMinutes, Valid: true}
	params.HasTimeCgmUsePercent = pgtype.Bool{Bool: p.HasTimeCGMUsePercent, Valid: true}
	params.HasTimeCgmUseRecords = pgtype.Bool{Bool: p.HasTimeCGMUseRecords, Valid: true}
	params.HasTimeInAnyHighMinutes = pgtype.Bool{Bool: p.HasTimeInAnyHighMinutes, Valid: true}
	params.HasTimeInAnyHighPercent = pgtype.Bool{Bool: p.HasTimeInAnyHighPercent, Valid: true}
	params.HasTimeInAnyHighRecords = pgtype.Bool{Bool: p.HasTimeInAnyHighRecords, Valid: true}
	params.HasTimeInAnyLowMinutes = pgtype.Bool{Bool: p.HasTimeInAnyLowMinutes, Valid: true}
	params.HasTimeInAnyLowPercent = pgtype.Bool{Bool: p.HasTimeInAnyLowPercent, Valid: true}
	params.HasTimeInAnyLowRecords = pgtype.Bool{Bool: p.HasTimeInAnyLowRecords, Valid: true}
	params.HasTimeInExtremeHighMinutes = pgtype.Bool{Bool: p.HasTimeInExtremeHighMinutes, Valid: true}
	params.HasTimeInExtremeHighPercent = pgtype.Bool{Bool: p.HasTimeInExtremeHighPercent, Valid: true}
	params.HasTimeInExtremeHighRecords = pgtype.Bool{Bool: p.HasTimeInExtremeHighRecords, Valid: true}
	params.HasTimeInHighMinutes = pgtype.Bool{Bool: p.HasTimeInHighMinutes, Valid: true}
	params.HasTimeInHighPercent = pgtype.Bool{Bool: p.HasTimeInHighPercent, Valid: true}
	params.HasTimeInHighRecords = pgtype.Bool{Bool: p.HasTimeInHighRecords, Valid: true}
	params.HasTimeInLowMinutes = pgtype.Bool{Bool: p.HasTimeInLowMinutes, Valid: true}
	params.HasTimeInLowPercent = pgtype.Bool{Bool: p.HasTimeInLowPercent, Valid: true}
	params.HasTimeInLowRecords = pgtype.Bool{Bool: p.HasTimeInLowRecords, Valid: true}
	params.HasTimeInTargetMinutes = pgtype.Bool{Bool: p.HasTimeInTargetMinutes, Valid: true}
	params.HasTimeInTargetPercent = pgtype.Bool{Bool: p.HasTimeInTargetPercent, Valid: true}
	params.HasTimeInTargetRecords = pgtype.Bool{Bool: p.HasTimeInTargetRecords, Valid: true}
	params.HasTimeInVeryHighMinutes = pgtype.Bool{Bool: p.HasTimeInVeryHighMinutes, Valid: true}
	params.HasTimeInVeryHighPercent = pgtype.Bool{Bool: p.HasTimeInVeryHighPercent, Valid: true}
	params.HasTimeInVeryHighRecords = pgtype.Bool{Bool: p.HasTimeInVeryHighRecords, Valid: true}
	params.HasTimeInVeryLowMinutes = pgtype.Bool{Bool: p.HasTimeInVeryLowMinutes, Valid: true}
	params.HasTimeInVeryLowPercent = pgtype.Bool{Bool: p.HasTimeInVeryLowPercent, Valid: true}
	params.HasTimeInVeryLowRecords = pgtype.Bool{Bool: p.HasTimeInVeryLowRecords, Valid: true}
	params.HasTotalRecords = pgtype.Bool{Bool: p.HasTotalRecords, Valid: true}
	params.HoursWithData = pgtype.Int4{Int32: int32(p.HoursWithData), Valid: true}
	params.HoursWithDataDelta = pgtype.Int4{Int32: int32(p.HoursWithDataDelta), Valid: true}
	params.Max = pgtype.Float8{Float64: p.Max, Valid: true}
	params.MaxDelta = pgtype.Float8{Float64: p.MaxDelta, Valid: true}
	params.Min = pgtype.Float8{Float64: p.Min, Valid: true}
	params.MinDelta = pgtype.Float8{Float64: p.MinDelta, Valid: true}
	params.StandardDeviation = pgtype.Float8{Float64: p.StandardDeviation, Valid: true}
	params.StandardDeviationDelta = pgtype.Float8{Float64: p.StandardDeviationDelta, Valid: true}
	params.TimeCgmUseMinutes = intPtrValue(p.TimeCGMUseMinutes)
	params.TimeCgmUseMinutesDelta = intPtrValue(p.TimeCGMUseMinutesDelta)
	params.TimeCgmUsePercent = f64Value(p.TimeCGMUsePercent)
	params.TimeCgmUsePercentDelta = f64Value(p.TimeCGMUsePercentDelta)
	params.TimeCgmUseRecords = intPtrValue(p.TimeCGMUseRecords)
	params.TimeCgmUseRecordsDelta = intPtrValue(p.TimeCGMUseRecordsDelta)
	params.TimeInAnyHighMinutes = intPtrValue(p.TimeInAnyHighMinutes)
	params.TimeInAnyHighMinutesDelta = intPtrValue(p.TimeInAnyHighMinutesDelta)
	params.TimeInAnyHighPercent = f64Value(p.TimeInAnyHighPercent)
	params.TimeInAnyHighPercentDelta = f64Value(p.TimeInAnyHighPercentDelta)
	params.TimeInAnyHighRecords = intPtrValue(p.TimeInAnyHighRecords)
	params.TimeInAnyHighRecordsDelta = intPtrValue(p.TimeInAnyHighRecordsDelta)
	params.TimeInAnyLowMinutes = intPtrValue(p.TimeInAnyLowMinutes)
	params.TimeInAnyLowMinutesDelta = intPtrValue(p.TimeInAnyLowMinutesDelta)
	params.TimeInAnyLowPercent = f64Value(p.TimeInAnyLowPercent)
	params.TimeInAnyLowPercentDelta = f64Value(p.TimeInAnyLowPercentDelta)
	params.TimeInAnyLowRecords = intPtrValue(p.TimeInAnyLowRecords)
	params.TimeInAnyLowRecordsDelta = intPtrValue(p.TimeInAnyLowRecordsDelta)
	params.TimeInExtremeHighMinutes = intPtrValue(p.TimeInExtremeHighMinutes)
	params.TimeInExtremeHighMinutesDelta = intPtrValue(p.TimeInExtremeHighMinutesDelta)
	params.TimeInExtremeHighPercent = f64Value(p.TimeInExtremeHighPercent)
	params.TimeInExtremeHighPercentDelta = f64Value(p.TimeInExtremeHighPercentDelta)
	params.TimeInExtremeHighRecords = intPtrValue(p.TimeInExtremeHighRecords)
	params.TimeInExtremeHighRecordsDelta = intPtrValue(p.TimeInExtremeHighRecordsDelta)
	params.TimeInHighMinutes = intPtrValue(p.TimeInHighMinutes)
	params.TimeInHighMinutesDelta = intPtrValue(p.TimeInHighMinutesDelta)
	params.TimeInHighPercent = f64Value(p.TimeInHighPercent)
	params.TimeInHighPercentDelta = f64Value(p.TimeInHighPercentDelta)
	params.TimeInHighRecords = intPtrValue(p.TimeInHighRecords)
	params.TimeInHighRecordsDelta = intPtrValue(p.TimeInHighRecordsDelta)
	params.TimeInLowMinutes = intPtrValue(p.TimeInLowMinutes)
	params.TimeInLowMinutesDelta = intPtrValue(p.TimeInLowMinutesDelta)
	params.TimeInLowPercent = f64Value(p.TimeInLowPercent)
	params.TimeInLowPercentDelta = f64Value(p.TimeInLowPercentDelta)
	params.TimeInLowRecords = intPtrValue(p.TimeInLowRecords)
	params.TimeInLowRecordsDelta = intPtrValue(p.TimeInLowRecordsDelta)
	params.TimeInTargetMinutes = intPtrValue(p.TimeInTargetMinutes)
	params.TimeInTargetMinutesDelta = intPtrValue(p.TimeInTargetMinutesDelta)
	params.TimeInTargetPercent = f64Value(p.TimeInTargetPercent)
	params.TimeInTargetPercentDelta = f64Value(p.TimeInTargetPercentDelta)
	params.TimeInTargetRecords = intPtrValue(p.TimeInTargetRecords)
	params.TimeInTargetRecordsDelta = intPtrValue(p.TimeInTargetRecordsDelta)
	params.TimeInVeryHighMinutes = intPtrValue(p.TimeInVeryHighMinutes)
	params.TimeInVeryHighMinutesDelta = intPtrValue(p.TimeInVeryHighMinutesDelta)
	params.TimeInVeryHighPercent = f64Value(p.TimeInVeryHighPercent)
	params.TimeInVeryHighPercentDelta = f64Value(p.TimeInVeryHighPercentDelta)
	params.TimeInVeryHighRecords = intPtrValue(p.TimeInVeryHighRecords)
	params.TimeInVeryHighRecordsDelta = intPtrValue(p.TimeInVeryHighRecordsDelta)
	params.TimeInVeryLowMinutes = intPtrValue(p.TimeInVeryLowMinutes)
	params.TimeInVeryLowMinutesDelta = intPtrValue(p.TimeInVeryLowMinutesDelta)
	params.TimeInVeryLowPercent = f64Value(p.TimeInVeryLowPercent)
	params.TimeInVeryLowPercentDelta = f64Value(p.TimeInVeryLowPercentDelta)
	params.TimeInVeryLowRecords = intPtrValue(p.TimeInVeryLowRecords)
	params.TimeInVeryLowRecordsDelta = intPtrValue(p.TimeInVeryLowRecordsDelta)
	params.TotalRecords = intPtrValue(p.TotalRecords)
	params.TotalRecordsDelta = intPtrValue(p.TotalRecordsDelta)
}

func bgmPeriodParams(p patients.PatientBGMPeriod, params *sqlcgen.InsertPatientSummaryPeriodParams) {
	params.AverageDailyRecords = f64Value(p.AverageDailyRecords)
	params.AverageDailyRecordsDelta = f64Value(p.AverageDailyRecordsDelta)
	params.AverageGlucoseMmol = f64Value(p.AverageGlucoseMmol)
	params.AverageGlucoseMmolDelta = f64Value(p.AverageGlucoseMmolDelta)
	params.CoefficientOfVariation = f64Value(p.CoefficientOfVariation)
	params.CoefficientOfVariationDelta = f64Value(p.CoefficientOfVariationDelta)
	params.DaysWithData = pgtype.Int4{Int32: int32(p.DaysWithData), Valid: true}
	params.DaysWithDataDelta = pgtype.Int4{Int32: int32(p.DaysWithDataDelta), Valid: true}
	params.HasAverageDailyRecords = pgtype.Bool{Bool: p.HasAverageDailyRecords, Valid: true}
	params.HasAverageGlucoseMmol = pgtype.Bool{Bool: p.HasAverageGlucoseMmol, Valid: true}
	params.HasTimeInAnyHighPercent = pgtype.Bool{Bool: p.HasTimeInAnyHighPercent, Valid: true}
	params.HasTimeInAnyHighRecords = pgtype.Bool{Bool: p.HasTimeInAnyHighRecords, Valid: true}
	params.HasTimeInAnyLowPercent = pgtype.Bool{Bool: p.HasTimeInAnyLowPercent, Valid: true}
	params.HasTimeInAnyLowRecords = pgtype.Bool{Bool: p.HasTimeInAnyLowRecords, Valid: true}
	params.HasTimeInExtremeHighPercent = pgtype.Bool{Bool: p.HasTimeInExtremeHighPercent, Valid: true}
	params.HasTimeInExtremeHighRecords = pgtype.Bool{Bool: p.HasTimeInExtremeHighRecords, Valid: true}
	params.HasTimeInHighPercent = pgtype.Bool{Bool: p.HasTimeInHighPercent, Valid: true}
	params.HasTimeInHighRecords = pgtype.Bool{Bool: p.HasTimeInHighRecords, Valid: true}
	params.HasTimeInLowPercent = pgtype.Bool{Bool: p.HasTimeInLowPercent, Valid: true}
	params.HasTimeInLowRecords = pgtype.Bool{Bool: p.HasTimeInLowRecords, Valid: true}
	params.HasTimeInTargetPercent = pgtype.Bool{Bool: p.HasTimeInTargetPercent, Valid: true}
	params.HasTimeInTargetRecords = pgtype.Bool{Bool: p.HasTimeInTargetRecords, Valid: true}
	params.HasTimeInVeryHighPercent = pgtype.Bool{Bool: p.HasTimeInVeryHighPercent, Valid: true}
	params.HasTimeInVeryHighRecords = pgtype.Bool{Bool: p.HasTimeInVeryHighRecords, Valid: true}
	params.HasTimeInVeryLowPercent = pgtype.Bool{Bool: p.HasTimeInVeryLowPercent, Valid: true}
	params.HasTimeInVeryLowRecords = pgtype.Bool{Bool: p.HasTimeInVeryLowRecords, Valid: true}
	params.HasTotalRecords = pgtype.Bool{Bool: p.HasTotalRecords, Valid: true}
	params.Max = pgtype.Float8{Float64: p.Max, Valid: true}
	params.MaxDelta = pgtype.Float8{Float64: p.MaxDelta, Valid: true}
	params.Min = pgtype.Float8{Float64: p.Min, Valid: true}
	params.MinDelta = pgtype.Float8{Float64: p.MinDelta, Valid: true}
	params.StandardDeviation = f64Value(p.StandardDeviation)
	params.StandardDeviationDelta = f64Value(p.StandardDeviationDelta)
	params.TimeInAnyHighPercent = f64Value(p.TimeInAnyHighPercent)
	params.TimeInAnyHighPercentDelta = f64Value(p.TimeInAnyHighPercentDelta)
	params.TimeInAnyHighRecords = intPtrValue(p.TimeInAnyHighRecords)
	params.TimeInAnyHighRecordsDelta = intPtrValue(p.TimeInAnyHighRecordsDelta)
	params.TimeInAnyLowPercent = f64Value(p.TimeInAnyLowPercent)
	params.TimeInAnyLowPercentDelta = f64Value(p.TimeInAnyLowPercentDelta)
	params.TimeInAnyLowRecords = intPtrValue(p.TimeInAnyLowRecords)
	params.TimeInAnyLowRecordsDelta = intPtrValue(p.TimeInAnyLowRecordsDelta)
	params.TimeInExtremeHighPercent = f64Value(p.TimeInExtremeHighPercent)
	params.TimeInExtremeHighPercentDelta = f64Value(p.TimeInExtremeHighPercentDelta)
	params.TimeInExtremeHighRecords = intPtrValue(p.TimeInExtremeHighRecords)
	params.TimeInExtremeHighRecordsDelta = intPtrValue(p.TimeInExtremeHighRecordsDelta)
	params.TimeInHighPercent = f64Value(p.TimeInHighPercent)
	params.TimeInHighPercentDelta = f64Value(p.TimeInHighPercentDelta)
	params.TimeInHighRecords = intPtrValue(p.TimeInHighRecords)
	params.TimeInHighRecordsDelta = intPtrValue(p.TimeInHighRecordsDelta)
	params.TimeInLowPercent = f64Value(p.TimeInLowPercent)
	params.TimeInLowPercentDelta = f64Value(p.TimeInLowPercentDelta)
	params.TimeInLowRecords = intPtrValue(p.TimeInLowRecords)
	params.TimeInLowRecordsDelta = intPtrValue(p.TimeInLowRecordsDelta)
	params.TimeInTargetPercent = f64Value(p.TimeInTargetPercent)
	params.TimeInTargetPercentDelta = f64Value(p.TimeInTargetPercentDelta)
	params.TimeInTargetRecords = intPtrValue(p.TimeInTargetRecords)
	params.TimeInTargetRecordsDelta = intPtrValue(p.TimeInTargetRecordsDelta)
	params.TimeInVeryHighPercent = f64Value(p.TimeInVeryHighPercent)
	params.TimeInVeryHighPercentDelta = f64Value(p.TimeInVeryHighPercentDelta)
	params.TimeInVeryHighRecords = intPtrValue(p.TimeInVeryHighRecords)
	params.TimeInVeryHighRecordsDelta = intPtrValue(p.TimeInVeryHighRecordsDelta)
	params.TimeInVeryLowPercent = f64Value(p.TimeInVeryLowPercent)
	params.TimeInVeryLowPercentDelta = f64Value(p.TimeInVeryLowPercentDelta)
	params.TimeInVeryLowRecords = intPtrValue(p.TimeInVeryLowRecords)
	params.TimeInVeryLowRecordsDelta = intPtrValue(p.TimeInVeryLowRecordsDelta)
	params.TotalRecords = intPtrValue(p.TotalRecords)
	params.TotalRecordsDelta = intPtrValue(p.TotalRecordsDelta)
}
