-- +goose Up
CREATE TABLE users (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username    TEXT        NOT NULL UNIQUE,
    password_hash TEXT      NOT NULL,
    role        TEXT        NOT NULL CHECK (role IN ('admin','experimenter','approver','viewer')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
