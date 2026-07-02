-- name: UpsertMergePlan :exec
INSERT INTO merge_plans (id, plan_id, type, payload, created_time, pg_synced_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (id) DO UPDATE
SET plan_id = EXCLUDED.plan_id,
    type = EXCLUDED.type,
    payload = EXCLUDED.payload,
    created_time = EXCLUDED.created_time,
    pg_synced_at = now();
