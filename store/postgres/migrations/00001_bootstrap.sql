-- +goose Up
CREATE TABLE pgsync_backfill_progress (
    collection text PRIMARY KEY,
    last_id text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE pgsync_backfill_progress;
