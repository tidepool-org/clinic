-- +goose Up
CREATE TABLE patients (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    clinic_id text NOT NULL,
    user_id text NOT NULL,
    full_name text,
    -- Lowercased, diacritic-stripped mirror of full_name, maintained by the
    -- writer. Replaces Mongo's strength-1 collation for name search.
    full_name_normalized text,
    -- Mongo stores birth dates as strings; ISO 8601 sorts correctly
    -- lexicographically and legacy documents may hold empty strings, so the
    -- value is kept as text
    birth_date text,
    email text,
    mrn text,
    require_unique_mrn boolean NOT NULL DEFAULT false,
    is_migrated boolean NOT NULL DEFAULT false,
    invited_by text,
    diagnosis_type text,
    target_devices text[],
    legacy_clinician_ids text[],
    -- Mongo stores permissions as empty sub-documents; a flag is true when
    -- the corresponding sub-document exists
    perm_custodian boolean NOT NULL DEFAULT false,
    perm_view boolean NOT NULL DEFAULT false,
    perm_upload boolean NOT NULL DEFAULT false,
    perm_note boolean NOT NULL DEFAULT false,
    glycemic_ranges_type text,
    glycemic_ranges_preset text,
    -- Custom glycemic ranges hold a variable-length threshold list which is
    -- never filtered or sorted by content
    glycemic_ranges_custom jsonb,
    created_time timestamptz,
    updated_time timestamptz,
    last_upload_reminder_time timestamptz,
    -- DEPRECATED in the domain model, kept for backfill fidelity
    last_requested_dexcom_connect_time timestamptz,
    pg_synced_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, user_id)
);

CREATE INDEX patients_user_id_idx ON patients (user_id);
CREATE INDEX patients_clinic_birth_date_idx ON patients (clinic_id, birth_date);
CREATE INDEX patients_clinic_email_idx ON patients (clinic_id, email);
CREATE INDEX patients_clinic_mrn_idx ON patients (clinic_id, mrn);
CREATE INDEX patients_clinic_full_name_normalized_idx ON patients (clinic_id, full_name_normalized text_pattern_ops);
-- Mirrors Mongo's conditional unique MRN index. Enforcement is best-effort
-- during dual writes: violations are logged and dropped, Mongo remains the
-- enforcer until reads move over.
CREATE UNIQUE INDEX patients_unique_mrn_idx ON patients (clinic_id, mrn)
    WHERE require_unique_mrn AND mrn IS NOT NULL;

CREATE TABLE patient_tags (
    patient_id text NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    tag_id text NOT NULL,
    PRIMARY KEY (patient_id, tag_id)
);
CREATE INDEX patient_tags_tag_id_idx ON patient_tags (tag_id);

CREATE TABLE patient_sites (
    patient_id text NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    site_id text NOT NULL,
    -- The site name is denormalized onto patient documents in Mongo
    site_name text NOT NULL,
    PRIMARY KEY (patient_id, site_id)
);
CREATE INDEX patient_sites_site_id_idx ON patient_sites (site_id);

CREATE TABLE patient_data_sources (
    patient_id text NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    ordinal integer NOT NULL,
    provider_name text NOT NULL,
    state text NOT NULL,
    data_source_id text,
    modified_time timestamptz,
    expiration_time timestamptz,
    latest_data_time timestamptz,
    PRIMARY KEY (patient_id, ordinal)
);
CREATE INDEX patient_data_sources_provider_state_idx ON patient_data_sources (provider_name, state);

CREATE TABLE patient_reviews (
    patient_id text NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    -- Preserves the Mongo array order; ordinal 0 is the most recent review
    ordinal integer NOT NULL,
    clinician_id text NOT NULL,
    review_time timestamptz NOT NULL,
    PRIMARY KEY (patient_id, ordinal)
);
CREATE INDEX patient_reviews_time_idx ON patient_reviews (patient_id, review_time DESC);

CREATE TABLE patient_provider_connection_requests (
    patient_id text NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    provider_name text NOT NULL,
    created_time timestamptz NOT NULL,
    PRIMARY KEY (patient_id, provider_name, created_time)
);

CREATE TABLE patient_ehr_subscriptions (
    patient_id text NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    name text NOT NULL,
    provider text NOT NULL,
    active boolean NOT NULL,
    created_time timestamptz,
    updated_time timestamptz,
    PRIMARY KEY (patient_id, name)
);

CREATE TABLE patient_ehr_subscription_matched_messages (
    patient_id text NOT NULL,
    subscription_name text NOT NULL,
    -- Preserves the Mongo array order; the last element is the most recent
    -- matched message
    ordinal integer NOT NULL,
    message_id text NOT NULL,
    data_model text NOT NULL,
    event_type text NOT NULL,
    PRIMARY KEY (patient_id, subscription_name, ordinal),
    FOREIGN KEY (patient_id, subscription_name)
        REFERENCES patient_ehr_subscriptions (patient_id, name) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE patient_ehr_subscription_matched_messages;
DROP TABLE patient_ehr_subscriptions;
DROP TABLE patient_provider_connection_requests;
DROP TABLE patient_reviews;
DROP TABLE patient_data_sources;
DROP TABLE patient_sites;
DROP TABLE patient_tags;
DROP TABLE patients;
