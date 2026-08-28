-- +goose Up
CREATE TABLE users(
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role TEXT NOT NULL CHECK (ROLE IN ('admin', 'experimenter', 'approver', 'viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;

