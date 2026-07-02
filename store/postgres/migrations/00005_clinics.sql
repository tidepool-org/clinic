-- +goose Up
CREATE TABLE clinics (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    name text,
    address text,
    city text,
    state text,
    postal_code text,
    country text,
    clinic_type text,
    clinic_size text,
    website text,
    timezone text,
    preferred_bg_units text,
    canonical_share_code text UNIQUE,
    tier text,
    is_migrated boolean NOT NULL DEFAULT false,
    created_time timestamptz,
    updated_time timestamptz,
    suppress_patient_clinic_invitation boolean,
    mrn_required boolean,
    mrn_unique boolean,
    ehr_enabled boolean,
    ehr_provider text,
    ehr_source_id text,
    ehr_mrn_id_type text,
    ehr_destination_flowsheet text,
    ehr_destination_notes text,
    ehr_destination_results text,
    ehr_procedure_enable_summary_reports text,
    ehr_procedure_disable_summary_reports text,
    ehr_procedure_create_account text,
    ehr_procedure_create_account_and_enable_reports text,
    ehr_scheduled_reports_cadence text,
    ehr_scheduled_reports_on_upload_enabled boolean,
    ehr_scheduled_reports_on_upload_note_event_type text,
    ehr_tags_codes text[],
    ehr_tags_separator text,
    ehr_flowsheets_icode boolean,
    ehr_notes_include_gmi boolean,
    pcs_hard_limit_plan integer,
    pcs_hard_limit_start_date timestamptz,
    pcs_hard_limit_end_date timestamptz,
    pcs_hard_limit_legacy_patient_count integer,
    pcs_soft_limit_plan integer,
    pcs_soft_limit_start_date timestamptz,
    pcs_soft_limit_end_date timestamptz,
    pcs_soft_limit_legacy_patient_count integer,
    patient_count_total integer,
    patient_count_demo integer,
    patient_count_plan integer,
    patient_count_legacy integer,
    -- Cached derived aggregate, rewritten wholesale and never queried by
    -- field, which qualifies it for the jsonb exception
    patient_count_providers jsonb,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX clinics_ehr_provider_idx ON clinics (ehr_provider);
CREATE INDEX clinics_ehr_source_id_idx ON clinics (ehr_source_id);

CREATE TABLE clinic_share_codes (
    share_code text PRIMARY KEY,
    clinic_id text NOT NULL REFERENCES clinics (id) ON DELETE CASCADE
);

CREATE INDEX clinic_share_codes_clinic_id_idx ON clinic_share_codes (clinic_id);

CREATE TABLE clinic_admins (
    clinic_id text NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    user_id text NOT NULL,
    PRIMARY KEY (clinic_id, user_id)
);

CREATE TABLE clinic_phone_numbers (
    clinic_id text NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    ordinal integer NOT NULL,
    type text,
    number text NOT NULL,
    PRIMARY KEY (clinic_id, ordinal)
);

CREATE TABLE clinic_membership_restrictions (
    clinic_id text NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    email_domain text NOT NULL,
    required_idp text,
    PRIMARY KEY (clinic_id, email_domain)
);

CREATE TABLE clinic_patient_tags (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    clinic_id text NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    name text NOT NULL,
    UNIQUE (clinic_id, name)
);

CREATE TABLE clinic_sites (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    clinic_id text NOT NULL REFERENCES clinics (id) ON DELETE CASCADE,
    name text NOT NULL,
    UNIQUE (clinic_id, name)
);

-- +goose Down
DROP TABLE clinic_sites;
DROP TABLE clinic_patient_tags;
DROP TABLE clinic_membership_restrictions;
DROP TABLE clinic_phone_numbers;
DROP TABLE clinic_admins;
DROP TABLE clinic_share_codes;
DROP TABLE clinics;
