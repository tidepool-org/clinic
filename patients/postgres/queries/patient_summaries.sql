-- name: DeletePatientSummaries :exec
DELETE FROM patient_summaries WHERE patient_id = $1;

-- name: DeleteSummariesBySummaryId :exec
DELETE FROM patient_summaries WHERE summary_id = $1;

-- name: InsertPatientSummary :exec
INSERT INTO patient_summaries (
    patient_id, summary_type, summary_id,
    config_schema_version, config_high_glucose_threshold,
    config_low_glucose_threshold, config_very_high_glucose_threshold,
    config_very_low_glucose_threshold,
    dates_first_data, dates_has_first_data, dates_has_last_data,
    dates_has_last_upload_date, dates_has_outdated_since, dates_last_data,
    dates_last_updated_date, dates_last_updated_reason,
    dates_last_upload_date, dates_outdated_reason, dates_outdated_since,
    dates_outdated_since_limit
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
);

-- name: InsertPatientSummaryPeriod :exec
INSERT INTO patient_summary_periods (
    patient_id, summary_type, period, average_daily_records,
    average_daily_records_delta, average_glucose_mmol, average_glucose_mmol_delta, coefficient_of_variation,
    coefficient_of_variation_delta, days_with_data, days_with_data_delta, glucose_management_indicator,
    glucose_management_indicator_delta, has_average_daily_records, has_average_glucose_mmol, has_glucose_management_indicator,
    has_time_cgm_use_minutes, has_time_cgm_use_percent, has_time_cgm_use_records, has_time_in_any_high_minutes,
    has_time_in_any_high_percent, has_time_in_any_high_records, has_time_in_any_low_minutes, has_time_in_any_low_percent,
    has_time_in_any_low_records, has_time_in_extreme_high_minutes, has_time_in_extreme_high_percent, has_time_in_extreme_high_records,
    has_time_in_high_minutes, has_time_in_high_percent, has_time_in_high_records, has_time_in_low_minutes,
    has_time_in_low_percent, has_time_in_low_records, has_time_in_target_minutes, has_time_in_target_percent,
    has_time_in_target_records, has_time_in_very_high_minutes, has_time_in_very_high_percent, has_time_in_very_high_records,
    has_time_in_very_low_minutes, has_time_in_very_low_percent, has_time_in_very_low_records, has_total_records,
    hours_with_data, hours_with_data_delta, max, max_delta,
    min, min_delta, standard_deviation, standard_deviation_delta,
    time_cgm_use_minutes, time_cgm_use_minutes_delta, time_cgm_use_percent, time_cgm_use_percent_delta,
    time_cgm_use_records, time_cgm_use_records_delta, time_in_any_high_minutes, time_in_any_high_minutes_delta,
    time_in_any_high_percent, time_in_any_high_percent_delta, time_in_any_high_records, time_in_any_high_records_delta,
    time_in_any_low_minutes, time_in_any_low_minutes_delta, time_in_any_low_percent, time_in_any_low_percent_delta,
    time_in_any_low_records, time_in_any_low_records_delta, time_in_extreme_high_minutes, time_in_extreme_high_minutes_delta,
    time_in_extreme_high_percent, time_in_extreme_high_percent_delta, time_in_extreme_high_records, time_in_extreme_high_records_delta,
    time_in_high_minutes, time_in_high_minutes_delta, time_in_high_percent, time_in_high_percent_delta,
    time_in_high_records, time_in_high_records_delta, time_in_low_minutes, time_in_low_minutes_delta,
    time_in_low_percent, time_in_low_percent_delta, time_in_low_records, time_in_low_records_delta,
    time_in_target_minutes, time_in_target_minutes_delta, time_in_target_percent, time_in_target_percent_delta,
    time_in_target_records, time_in_target_records_delta, time_in_very_high_minutes, time_in_very_high_minutes_delta,
    time_in_very_high_percent, time_in_very_high_percent_delta, time_in_very_high_records, time_in_very_high_records_delta,
    time_in_very_low_minutes, time_in_very_low_minutes_delta, time_in_very_low_percent, time_in_very_low_percent_delta,
    time_in_very_low_records, time_in_very_low_records_delta, total_records, total_records_delta
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
    $31, $32, $33, $34, $35, $36, $37, $38, $39, $40,
    $41, $42, $43, $44, $45, $46, $47, $48, $49, $50,
    $51, $52, $53, $54, $55, $56, $57, $58, $59, $60,
    $61, $62, $63, $64, $65, $66, $67, $68, $69, $70,
    $71, $72, $73, $74, $75, $76, $77, $78, $79, $80,
    $81, $82, $83, $84, $85, $86, $87, $88, $89, $90,
    $91, $92, $93, $94, $95, $96, $97, $98, $99, $100,
    $101, $102, $103, $104, $105, $106, $107, $108
);
