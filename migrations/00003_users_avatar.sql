-- +goose Up
ALTER TABLE users
    ADD COLUMN avatar_url text(512);

-- +goose Down
ALTER TABLE users
    DROP COLUMN avatar_url;

