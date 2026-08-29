-- +goose Up
CREATE TABLE users(
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username text(64) NOT NULL UNIQUE,
    password_hash text(64) NOT NULL,
    role TEXT(16) NOT NULL CHECK (ROLE IN ('admin', 'experimenter', 'approver', 'viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;

