-- +goose Up
CREATE TABLE event_types(
    key varchar(128) PRIMARY KEY,
    name varchar(256) NOT NULL CHECK (length(trim(name)) > 0),
    description text NULL,
    schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    schema_version int NOT NULL DEFAULT 1 CHECK (schema_version > 0),
    require_exposure boolean NOT NULL DEFAULT TRUE,
    status varchar(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE event_idempotency_keys(
    event_id uuid PRIMARY KEY,
    request_hash bytea NOT NULL,
    response jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL DEFAULT now() + interval '24 hours'
);

CREATE INDEX idx_event_idempotency_expires ON event_idempotency_keys(expires_at);

INSERT INTO event_types(key, name, description, require_exposure) VALUES
    ('exposure', 'Exposure', 'Subject was exposed to an experiment variant', FALSE),
    ('conversion', 'Conversion', 'Example conversion event attributed to an exposure', TRUE),
    ('error', 'Error', 'Technical error event, attributed without exposure', FALSE),
    ('latency', 'Latency', 'Technical latency event, attributed without exposure', FALSE);

-- +goose Down
DROP TABLE event_idempotency_keys;
DROP TABLE event_types;
