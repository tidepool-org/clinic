-- +goose Up
CREATE TABLE clinicians (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{24}$'),
    clinic_id text NOT NULL,
    user_id text,
    email text,
    -- The Mongo document stores the clinician name under "name"; note that
    -- the Mongo text index is declared on "fullName" and therefore never
    -- matches names, only emails
    full_name text,
    invite_id text,
    roles text[] NOT NULL DEFAULT '{}',
    is_service_account boolean NOT NULL DEFAULT false,
    created_time timestamptz,
    updated_time timestamptz,
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(email, '') || ' ' || coalesce(full_name, ''))
    ) STORED,
    pg_synced_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX clinicians_unique_user_idx ON clinicians (clinic_id, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX clinicians_unique_invite_idx ON clinicians (clinic_id, invite_id) WHERE invite_id IS NOT NULL;
CREATE UNIQUE INDEX clinicians_unique_email_idx ON clinicians (clinic_id, email) WHERE email IS NOT NULL;
CREATE INDEX clinicians_user_id_idx ON clinicians (user_id);
CREATE INDEX clinicians_created_time_idx ON clinicians (clinic_id, created_time);
CREATE INDEX clinicians_roles_idx ON clinicians USING GIN (roles);
CREATE INDEX clinicians_search_idx ON clinicians USING GIN (search_vector);

CREATE TABLE clinician_roles_updates (
    clinician_id text NOT NULL REFERENCES clinicians (id) ON DELETE CASCADE,
    ordinal integer NOT NULL,
    roles text[] NOT NULL DEFAULT '{}',
    updated_by text,
    PRIMARY KEY (clinician_id, ordinal)
);

-- +goose Down
DROP TABLE clinician_roles_updates;
DROP TABLE clinicians;
