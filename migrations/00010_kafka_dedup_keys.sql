-- +goose Up
CREATE TABLE kafka_dedup_keys(
    event_id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE kafka_dedup_keys;
