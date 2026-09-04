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
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    email varchar(96) UNIQUE NOT NULL,
    full_name varchar(255) UNIQUE NOT NULL,
    password_hash varchar(255) NOT NULL,
    "role" varchar(16) NOT NULL CHECK (ROLE IN ('admin', 'experimenter', 'approver', 'viewer')),
    avatar_url varchar(512) NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users(email);

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER users_updated_at ON users;

DROP FUNCTION update_updated_at();

DROP TABLE users;

