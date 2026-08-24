-- +goose Up
ALTER TABLE users ADD COLUMN email TEXT;

UPDATE users SET email = username || '@lotty.local' WHERE email IS NULL;

ALTER TABLE users ALTER COLUMN email SET NOT NULL;
CREATE UNIQUE INDEX users_email_uniq ON users (email);

ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;

-- +goose Down
DROP INDEX IF EXISTS users_email_uniq;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE users DROP COLUMN IF EXISTS email;
