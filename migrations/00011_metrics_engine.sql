-- +goose Up
ALTER TABLE metrics ADD COLUMN formula jsonb NULL;

-- +goose Down
ALTER TABLE metrics DROP COLUMN formula;
