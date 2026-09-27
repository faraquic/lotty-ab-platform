-- +goose Up
CREATE TABLE outbox(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    topic varchar(255) NOT NULL,
    key varchar(255),
    type varchar(255) NOT NULL,
    payload jsonb NOT NULL,
    headers jsonb,
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempts int NOT NULL DEFAULT 0,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    CONSTRAINT outbox_status_check CHECK (status IN ('pending', 'published', 'dead'))
);

CREATE INDEX idx_outbox_pending ON outbox (next_attempt_at, created_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE outbox;
