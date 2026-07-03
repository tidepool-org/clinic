-- name: UpsertMigration :exec
INSERT INTO migrations (user_id, clinic_id, status, created_time, updated_time, pg_synced_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (user_id) DO UPDATE
SET clinic_id = EXCLUDED.clinic_id,
    status = EXCLUDED.status,
    created_time = EXCLUDED.created_time,
    updated_time = EXCLUDED.updated_time,
    pg_synced_at = now();
