-- name: GetBackfillProgress :one
SELECT collection, last_id, updated_at
FROM pgsync_backfill_progress
WHERE collection = $1;

-- name: UpsertBackfillProgress :exec
INSERT INTO pgsync_backfill_progress (collection, last_id, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (collection) DO UPDATE
SET last_id = EXCLUDED.last_id, updated_at = now();

-- name: DeleteBackfillProgress :exec
DELETE FROM pgsync_backfill_progress
WHERE collection = $1;
