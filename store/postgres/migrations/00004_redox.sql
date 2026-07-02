-- +goose Up
CREATE TABLE redox_messages (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    meta_data_model text NOT NULL,
    meta_event_type text NOT NULL,
    meta_source_id text,
    meta_source_name text,
    meta_facility_code text,
    meta_log_ids text[] NOT NULL DEFAULT '{}',
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX redox_messages_data_model_event_type ON redox_messages (meta_data_model, meta_event_type);
CREATE INDEX redox_messages_source ON redox_messages (meta_source_id, meta_facility_code);
CREATE INDEX redox_messages_log_ids ON redox_messages USING GIN (meta_log_ids);

CREATE TABLE scheduled_summary_reports_orders (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    clinic_id text,
    user_id text,
    last_matched_order_id text,
    created_time timestamptz NOT NULL,
    payload jsonb NOT NULL,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX scheduled_summary_reports_orders_last_matched_order
    ON scheduled_summary_reports_orders (last_matched_order_id, created_time DESC);
-- Replaces the Mongo TTL index; rows are removed by `pgsync prune`
CREATE INDEX scheduled_summary_reports_orders_created_time
    ON scheduled_summary_reports_orders (created_time);

-- +goose Down
DROP TABLE scheduled_summary_reports_orders;
DROP TABLE redox_messages;
