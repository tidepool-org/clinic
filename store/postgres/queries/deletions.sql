-- name: UpsertPatientDeletion :exec
INSERT INTO patient_deletions (id, deleted_time, deleted_by_user_id, clinic_id, user_id, payload, pg_synced_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (id) DO UPDATE
SET deleted_time = EXCLUDED.deleted_time,
    deleted_by_user_id = EXCLUDED.deleted_by_user_id,
    clinic_id = EXCLUDED.clinic_id,
    user_id = EXCLUDED.user_id,
    payload = EXCLUDED.payload,
    pg_synced_at = now();

-- name: UpsertClinicianDeletion :exec
INSERT INTO clinician_deletions (id, deleted_time, deleted_by_user_id, clinic_id, user_id, payload, pg_synced_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (id) DO UPDATE
SET deleted_time = EXCLUDED.deleted_time,
    deleted_by_user_id = EXCLUDED.deleted_by_user_id,
    clinic_id = EXCLUDED.clinic_id,
    user_id = EXCLUDED.user_id,
    payload = EXCLUDED.payload,
    pg_synced_at = now();

-- name: UpsertClinicDeletion :exec
INSERT INTO clinic_deletions (id, deleted_time, deleted_by_user_id, clinic_id, payload, pg_synced_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (id) DO UPDATE
SET deleted_time = EXCLUDED.deleted_time,
    deleted_by_user_id = EXCLUDED.deleted_by_user_id,
    clinic_id = EXCLUDED.clinic_id,
    payload = EXCLUDED.payload,
    pg_synced_at = now();
