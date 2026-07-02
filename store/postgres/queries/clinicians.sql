-- name: UpsertClinician :exec
INSERT INTO clinicians (
    id, clinic_id, user_id, email, full_name, invite_id, roles,
    is_service_account, created_time, updated_time, pg_synced_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now()
)
ON CONFLICT (id) DO UPDATE SET
    clinic_id = EXCLUDED.clinic_id,
    user_id = EXCLUDED.user_id,
    email = EXCLUDED.email,
    full_name = EXCLUDED.full_name,
    invite_id = EXCLUDED.invite_id,
    roles = EXCLUDED.roles,
    is_service_account = EXCLUDED.is_service_account,
    created_time = EXCLUDED.created_time,
    updated_time = EXCLUDED.updated_time,
    pg_synced_at = now();

-- name: DeleteClinicianRolesUpdates :exec
DELETE FROM clinician_roles_updates WHERE clinician_id = $1;

-- name: InsertClinicianRolesUpdate :exec
INSERT INTO clinician_roles_updates (clinician_id, ordinal, roles, updated_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (clinician_id, ordinal) DO UPDATE SET
    roles = EXCLUDED.roles,
    updated_by = EXCLUDED.updated_by;

-- name: DeleteClinician :exec
DELETE FROM clinicians WHERE clinic_id = $1 AND user_id = $2;

-- name: DeleteAllClinicians :exec
DELETE FROM clinicians WHERE clinic_id = $1;

-- name: DeleteClinicianInvite :exec
DELETE FROM clinicians WHERE clinic_id = $1 AND invite_id = $2;
