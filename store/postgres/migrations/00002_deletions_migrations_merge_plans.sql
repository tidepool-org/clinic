-- +goose Up
CREATE TABLE patient_deletions (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    deleted_time timestamptz NOT NULL,
    deleted_by_user_id text,
    clinic_id text,
    user_id text,
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX patient_deletions_clinic_user ON patient_deletions (clinic_id, user_id);
CREATE INDEX patient_deletions_deleted_time ON patient_deletions (deleted_time);

CREATE TABLE clinician_deletions (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    deleted_time timestamptz NOT NULL,
    deleted_by_user_id text,
    clinic_id text,
    user_id text,
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX clinician_deletions_clinic_user ON clinician_deletions (clinic_id, user_id);
CREATE INDEX clinician_deletions_deleted_time ON clinician_deletions (deleted_time);

CREATE TABLE clinic_deletions (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    deleted_time timestamptz NOT NULL,
    deleted_by_user_id text,
    clinic_id text,
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX clinic_deletions_clinic ON clinic_deletions (clinic_id);
CREATE INDEX clinic_deletions_deleted_time ON clinic_deletions (deleted_time);

-- The migrations domain model does not expose the Mongo document id, and the
-- user id is unique in Mongo, so it serves as the natural primary key.
CREATE TABLE migrations (
    user_id text PRIMARY KEY,
    clinic_id text NOT NULL,
    status text NOT NULL,
    created_time timestamptz,
    updated_time timestamptz,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX migrations_clinic ON migrations (clinic_id);

CREATE TABLE merge_plans (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    plan_id text NOT NULL,
    type text NOT NULL,
    payload jsonb NOT NULL,
    created_time timestamptz,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX merge_plans_plan ON merge_plans (plan_id);

-- +goose Down
DROP TABLE merge_plans;
DROP TABLE migrations;
DROP TABLE clinic_deletions;
DROP TABLE clinician_deletions;
DROP TABLE patient_deletions;
