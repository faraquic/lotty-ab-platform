-- +goose Up
ALTER TABLE users
    ADD COLUMN email text(96) NOT NULL,
    ADD COLUMN deleted_at timestamptz;

-- +goose Down
ALTER TABLE users
    DROP COLUMN email, deleted_at;

