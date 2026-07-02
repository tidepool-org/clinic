-- +goose Up
CREATE TABLE xealth_preorders (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    data_tracking_id text UNIQUE NOT NULL,
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE xealth_orders (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE xealth_report_views (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    user_id text NOT NULL,
    deployment_id text NOT NULL,
    system_login text,
    patient_user_id text NOT NULL,
    program_id text NOT NULL,
    clinic_id text NOT NULL CHECK (clinic_id ~ '^[0-9a-f]{24}$'),
    created_time timestamptz NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX xealth_report_views_last_view
    ON xealth_report_views (clinic_id, deployment_id, patient_user_id, program_id, user_id, created_time DESC);

-- +goose Down
DROP TABLE xealth_report_views;
DROP TABLE xealth_orders;
DROP TABLE xealth_preorders;
