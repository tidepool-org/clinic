-- name: UpsertRedoxMessage :exec
INSERT INTO redox_messages (
    id, meta_data_model, meta_event_type, meta_source_id, meta_source_name,
    meta_facility_code, meta_log_ids, payload, pg_synced_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (id) DO UPDATE SET
    meta_data_model = EXCLUDED.meta_data_model,
    meta_event_type = EXCLUDED.meta_event_type,
    meta_source_id = EXCLUDED.meta_source_id,
    meta_source_name = EXCLUDED.meta_source_name,
    meta_facility_code = EXCLUDED.meta_facility_code,
    meta_log_ids = EXCLUDED.meta_log_ids,
    payload = EXCLUDED.payload,
    pg_synced_at = now();

-- name: UpsertScheduledSummaryReportsOrder :exec
INSERT INTO scheduled_summary_reports_orders (
    id, clinic_id, user_id, last_matched_order_id, created_time, payload, pg_synced_at
) VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (id) DO UPDATE SET
    clinic_id = EXCLUDED.clinic_id,
    user_id = EXCLUDED.user_id,
    last_matched_order_id = EXCLUDED.last_matched_order_id,
    created_time = EXCLUDED.created_time,
    payload = EXCLUDED.payload,
    pg_synced_at = now();

-- name: PruneScheduledSummaryReportsOrders :execrows
DELETE FROM scheduled_summary_reports_orders
WHERE created_time < $1;
