-- name: UpsertXealthPreorder :exec
INSERT INTO xealth_preorders (id, data_tracking_id, payload, pg_synced_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (id) DO UPDATE
SET data_tracking_id = EXCLUDED.data_tracking_id,
    payload = EXCLUDED.payload,
    pg_synced_at = now();

-- name: UpsertXealthOrder :exec
INSERT INTO xealth_orders (id, payload, pg_synced_at)
VALUES ($1, $2, now())
ON CONFLICT (id) DO UPDATE
SET payload = EXCLUDED.payload,
    pg_synced_at = now();

-- name: UpsertXealthReportView :exec
INSERT INTO xealth_report_views (id, user_id, deployment_id, system_login, patient_user_id, program_id, clinic_id, created_time, pg_synced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (id) DO UPDATE
SET user_id = EXCLUDED.user_id,
    deployment_id = EXCLUDED.deployment_id,
    system_login = EXCLUDED.system_login,
    patient_user_id = EXCLUDED.patient_user_id,
    program_id = EXCLUDED.program_id,
    clinic_id = EXCLUDED.clinic_id,
    created_time = EXCLUDED.created_time,
    pg_synced_at = now();

-- name: DeleteConflictingPreorders :exec
-- Removes stale rows that would collide with the unique data_tracking_id;
-- Mongo enforces the same uniqueness.
DELETE FROM xealth_preorders
WHERE data_tracking_id = $1 AND id <> $2;
