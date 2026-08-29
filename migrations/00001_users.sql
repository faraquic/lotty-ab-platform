-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at()
    RETURNS TRIGGER
    AS $body$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$body$
LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE users(
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email varchar(96) UNIQUE NOT NULL,
    username varchar(64) UNIQUE NOT NULL,
    password_hash varchar(64) NOT NULL,
    "role" varchar(16) NOT NULL CHECK (ROLE IN ('admin', 'experimenter', 'approver', 'viewer')),
    avatar_url varchar(512) NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER users_updated_at ON users;

DROP FUNCTION update_updated_at();

DROP TABLE users;