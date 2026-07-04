-- name: UpsertClinic :exec
INSERT INTO clinics (
    id, name, address, city, state, postal_code, country, clinic_type,
    clinic_size, website, timezone, preferred_bg_units, canonical_share_code,
    tier, is_migrated, created_time, updated_time,
    suppress_patient_clinic_invitation, mrn_required, mrn_unique,
    ehr_enabled, ehr_provider, ehr_source_id, ehr_mrn_id_type,
    ehr_destination_flowsheet, ehr_destination_notes, ehr_destination_results,
    ehr_procedure_enable_summary_reports, ehr_procedure_disable_summary_reports,
    ehr_procedure_create_account, ehr_procedure_create_account_and_enable_reports,
    ehr_scheduled_reports_cadence, ehr_scheduled_reports_on_upload_enabled,
    ehr_scheduled_reports_on_upload_note_event_type,
    ehr_tags_codes, ehr_tags_separator, ehr_flowsheets_icode, ehr_notes_include_gmi,
    pcs_hard_limit_plan, pcs_hard_limit_start_date, pcs_hard_limit_end_date,
    pcs_hard_limit_legacy_patient_count,
    pcs_soft_limit_plan, pcs_soft_limit_start_date, pcs_soft_limit_end_date,
    pcs_soft_limit_legacy_patient_count,
    patient_count_total, patient_count_demo, patient_count_plan,
    patient_count_legacy, patient_count_providers, pg_synced_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
    $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44,
    $45, $46, $47, $48, $49, $50, $51, now()
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    address = EXCLUDED.address,
    city = EXCLUDED.city,
    state = EXCLUDED.state,
    postal_code = EXCLUDED.postal_code,
    country = EXCLUDED.country,
    clinic_type = EXCLUDED.clinic_type,
    clinic_size = EXCLUDED.clinic_size,
    website = EXCLUDED.website,
    timezone = EXCLUDED.timezone,
    preferred_bg_units = EXCLUDED.preferred_bg_units,
    canonical_share_code = EXCLUDED.canonical_share_code,
    tier = EXCLUDED.tier,
    is_migrated = EXCLUDED.is_migrated,
    created_time = EXCLUDED.created_time,
    updated_time = EXCLUDED.updated_time,
    suppress_patient_clinic_invitation = EXCLUDED.suppress_patient_clinic_invitation,
    mrn_required = EXCLUDED.mrn_required,
    mrn_unique = EXCLUDED.mrn_unique,
    ehr_enabled = EXCLUDED.ehr_enabled,
    ehr_provider = EXCLUDED.ehr_provider,
    ehr_source_id = EXCLUDED.ehr_source_id,
    ehr_mrn_id_type = EXCLUDED.ehr_mrn_id_type,
    ehr_destination_flowsheet = EXCLUDED.ehr_destination_flowsheet,
    ehr_destination_notes = EXCLUDED.ehr_destination_notes,
    ehr_destination_results = EXCLUDED.ehr_destination_results,
    ehr_procedure_enable_summary_reports = EXCLUDED.ehr_procedure_enable_summary_reports,
    ehr_procedure_disable_summary_reports = EXCLUDED.ehr_procedure_disable_summary_reports,
    ehr_procedure_create_account = EXCLUDED.ehr_procedure_create_account,
    ehr_procedure_create_account_and_enable_reports = EXCLUDED.ehr_procedure_create_account_and_enable_reports,
    ehr_scheduled_reports_cadence = EXCLUDED.ehr_scheduled_reports_cadence,
    ehr_scheduled_reports_on_upload_enabled = EXCLUDED.ehr_scheduled_reports_on_upload_enabled,
    ehr_scheduled_reports_on_upload_note_event_type = EXCLUDED.ehr_scheduled_reports_on_upload_note_event_type,
    ehr_tags_codes = EXCLUDED.ehr_tags_codes,
    ehr_tags_separator = EXCLUDED.ehr_tags_separator,
    ehr_flowsheets_icode = EXCLUDED.ehr_flowsheets_icode,
    ehr_notes_include_gmi = EXCLUDED.ehr_notes_include_gmi,
    pcs_hard_limit_plan = EXCLUDED.pcs_hard_limit_plan,
    pcs_hard_limit_start_date = EXCLUDED.pcs_hard_limit_start_date,
    pcs_hard_limit_end_date = EXCLUDED.pcs_hard_limit_end_date,
    pcs_hard_limit_legacy_patient_count = EXCLUDED.pcs_hard_limit_legacy_patient_count,
    pcs_soft_limit_plan = EXCLUDED.pcs_soft_limit_plan,
    pcs_soft_limit_start_date = EXCLUDED.pcs_soft_limit_start_date,
    pcs_soft_limit_end_date = EXCLUDED.pcs_soft_limit_end_date,
    pcs_soft_limit_legacy_patient_count = EXCLUDED.pcs_soft_limit_legacy_patient_count,
    patient_count_total = EXCLUDED.patient_count_total,
    patient_count_demo = EXCLUDED.patient_count_demo,
    patient_count_plan = EXCLUDED.patient_count_plan,
    patient_count_legacy = EXCLUDED.patient_count_legacy,
    patient_count_providers = EXCLUDED.patient_count_providers,
    pg_synced_at = now();

-- name: DeleteClinic :exec
DELETE FROM clinics WHERE id = $1;

-- name: DeleteClinicShareCodes :exec
DELETE FROM clinic_share_codes WHERE clinic_id = $1;

-- name: InsertClinicShareCode :batchexec
INSERT INTO clinic_share_codes (share_code, clinic_id)
VALUES ($1, $2)
ON CONFLICT (share_code) DO UPDATE SET clinic_id = EXCLUDED.clinic_id;

-- name: DeleteClinicAdmins :exec
DELETE FROM clinic_admins WHERE clinic_id = $1;

-- name: InsertClinicAdmin :batchexec
INSERT INTO clinic_admins (clinic_id, user_id)
VALUES ($1, $2)
ON CONFLICT (clinic_id, user_id) DO NOTHING;

-- name: DeleteClinicPhoneNumbers :exec
DELETE FROM clinic_phone_numbers WHERE clinic_id = $1;

-- name: InsertClinicPhoneNumber :batchexec
INSERT INTO clinic_phone_numbers (clinic_id, ordinal, type, number)
VALUES ($1, $2, $3, $4)
ON CONFLICT (clinic_id, ordinal) DO UPDATE SET
    type = EXCLUDED.type,
    number = EXCLUDED.number;

-- name: DeleteClinicMembershipRestrictions :exec
DELETE FROM clinic_membership_restrictions WHERE clinic_id = $1;

-- name: InsertClinicMembershipRestriction :batchexec
INSERT INTO clinic_membership_restrictions (clinic_id, email_domain, required_idp)
VALUES ($1, $2, $3)
ON CONFLICT (clinic_id, email_domain) DO UPDATE SET
    required_idp = EXCLUDED.required_idp;

-- name: DeleteClinicPatientTags :exec
DELETE FROM clinic_patient_tags WHERE clinic_id = $1;

-- name: InsertClinicPatientTag :batchexec
INSERT INTO clinic_patient_tags (id, clinic_id, name)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET
    clinic_id = EXCLUDED.clinic_id,
    name = EXCLUDED.name;

-- name: DeleteClinicSites :exec
DELETE FROM clinic_sites WHERE clinic_id = $1;

-- name: InsertClinicSite :batchexec
INSERT INTO clinic_sites (id, clinic_id, name)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET
    clinic_id = EXCLUDED.clinic_id,
    name = EXCLUDED.name;
