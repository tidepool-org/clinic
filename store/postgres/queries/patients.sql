-- name: UpsertPatient :exec
INSERT INTO patients (
    id, clinic_id, user_id, full_name, full_name_normalized, birth_date,
    email, mrn, require_unique_mrn, is_migrated, invited_by, diagnosis_type,
    target_devices, legacy_clinician_ids,
    perm_custodian, perm_view, perm_upload, perm_note,
    glycemic_ranges_type, glycemic_ranges_preset, glycemic_ranges_custom,
    created_time, updated_time, last_upload_reminder_time,
    last_requested_dexcom_connect_time, pg_synced_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
    $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, now()
)
ON CONFLICT (id) DO UPDATE SET
    clinic_id = EXCLUDED.clinic_id,
    user_id = EXCLUDED.user_id,
    full_name = EXCLUDED.full_name,
    full_name_normalized = EXCLUDED.full_name_normalized,
    birth_date = EXCLUDED.birth_date,
    email = EXCLUDED.email,
    mrn = EXCLUDED.mrn,
    require_unique_mrn = EXCLUDED.require_unique_mrn,
    is_migrated = EXCLUDED.is_migrated,
    invited_by = EXCLUDED.invited_by,
    diagnosis_type = EXCLUDED.diagnosis_type,
    target_devices = EXCLUDED.target_devices,
    legacy_clinician_ids = EXCLUDED.legacy_clinician_ids,
    perm_custodian = EXCLUDED.perm_custodian,
    perm_view = EXCLUDED.perm_view,
    perm_upload = EXCLUDED.perm_upload,
    perm_note = EXCLUDED.perm_note,
    glycemic_ranges_type = EXCLUDED.glycemic_ranges_type,
    glycemic_ranges_preset = EXCLUDED.glycemic_ranges_preset,
    glycemic_ranges_custom = EXCLUDED.glycemic_ranges_custom,
    created_time = EXCLUDED.created_time,
    updated_time = EXCLUDED.updated_time,
    last_upload_reminder_time = EXCLUDED.last_upload_reminder_time,
    last_requested_dexcom_connect_time = EXCLUDED.last_requested_dexcom_connect_time,
    pg_synced_at = now();

-- name: DeletePatient :exec
DELETE FROM patients WHERE clinic_id = $1 AND user_id = $2;

-- name: DeletePatientsByUserId :exec
DELETE FROM patients WHERE user_id = $1;

-- name: DeleteNonCustodialPatientsOfClinic :exec
DELETE FROM patients WHERE clinic_id = $1 AND NOT perm_custodian;

-- name: DeletePatientTags :exec
DELETE FROM patient_tags WHERE patient_id = $1;

-- name: InsertPatientTag :exec
INSERT INTO patient_tags (patient_id, tag_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeletePatientSites :exec
DELETE FROM patient_sites WHERE patient_id = $1;

-- name: InsertPatientSite :exec
INSERT INTO patient_sites (patient_id, site_id, site_name)
VALUES ($1, $2, $3)
ON CONFLICT (patient_id, site_id) DO UPDATE SET site_name = EXCLUDED.site_name;

-- name: DeletePatientDataSources :exec
DELETE FROM patient_data_sources WHERE patient_id = $1;

-- name: InsertPatientDataSource :exec
INSERT INTO patient_data_sources (
    patient_id, ordinal, provider_name, state, data_source_id,
    modified_time, expiration_time, latest_data_time
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: DeletePatientReviews :exec
DELETE FROM patient_reviews WHERE patient_id = $1;

-- name: InsertPatientReview :exec
INSERT INTO patient_reviews (patient_id, ordinal, clinician_id, review_time)
VALUES ($1, $2, $3, $4);

-- name: DeletePatientConnectionRequests :exec
DELETE FROM patient_provider_connection_requests WHERE patient_id = $1;

-- name: InsertPatientConnectionRequest :exec
INSERT INTO patient_provider_connection_requests (patient_id, provider_name, created_time)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: DeletePatientEHRSubscriptions :exec
DELETE FROM patient_ehr_subscriptions WHERE patient_id = $1;

-- name: InsertPatientEHRSubscription :exec
INSERT INTO patient_ehr_subscriptions (patient_id, name, provider, active, created_time, updated_time)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertPatientEHRSubscriptionMatchedMessage :exec
INSERT INTO patient_ehr_subscription_matched_messages (
    patient_id, subscription_name, ordinal, message_id, data_model, event_type
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: AssignTagToClinicPatients :exec
INSERT INTO patient_tags (patient_id, tag_id)
SELECT id, $2 FROM patients WHERE clinic_id = $1
ON CONFLICT DO NOTHING;

-- name: AssignTagToPatients :exec
INSERT INTO patient_tags (patient_id, tag_id)
SELECT id, sqlc.arg(tag_id) FROM patients
WHERE clinic_id = sqlc.arg(clinic_id) AND user_id = ANY(sqlc.arg(user_ids)::text[])
ON CONFLICT DO NOTHING;

-- name: DeleteTagFromClinicPatients :exec
DELETE FROM patient_tags
USING patients
WHERE patient_tags.patient_id = patients.id
  AND patients.clinic_id = $1
  AND patient_tags.tag_id = $2;

-- name: DeleteTagFromPatients :exec
DELETE FROM patient_tags
USING patients
WHERE patient_tags.patient_id = patients.id
  AND patients.clinic_id = sqlc.arg(clinic_id)
  AND patient_tags.tag_id = sqlc.arg(tag_id)
  AND patients.user_id = ANY(sqlc.arg(user_ids)::text[]);

-- name: DeleteSiteFromClinicPatients :exec
DELETE FROM patient_sites
USING patients
WHERE patient_sites.patient_id = patients.id
  AND patients.clinic_id = $1
  AND patient_sites.site_id = $2;

-- name: RenameSiteForClinicPatients :exec
UPDATE patient_sites SET site_name = $3
FROM patients
WHERE patient_sites.patient_id = patients.id
  AND patients.clinic_id = $1
  AND patient_sites.site_id = $2;

-- name: AddSiteToPatientsWithSite :exec
INSERT INTO patient_sites (patient_id, site_id, site_name)
SELECT ps.patient_id, sqlc.arg(target_site_id), sqlc.arg(target_site_name)
FROM patient_sites ps
JOIN patients p ON p.id = ps.patient_id
WHERE p.clinic_id = sqlc.arg(clinic_id) AND ps.site_id = sqlc.arg(source_site_id)
ON CONFLICT (patient_id, site_id) DO NOTHING;

-- name: AddSiteToPatientsWithTag :exec
INSERT INTO patient_sites (patient_id, site_id, site_name)
SELECT pt.patient_id, sqlc.arg(site_id), sqlc.arg(site_name)
FROM patient_tags pt
JOIN patients p ON p.id = pt.patient_id
WHERE p.clinic_id = sqlc.arg(clinic_id) AND pt.tag_id = sqlc.arg(tag_id)
ON CONFLICT (patient_id, site_id) DO NOTHING;
