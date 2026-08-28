-- +goose Up
CREATE
OR REPLACE FUNCTION update_updated_at () RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at BEFORE
UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at ();

-- +goose Down
DROP TRIGGER IF EXISTS users_updated_at ON users;

DROP FUNCTION IF EXISTS update_updated_at ();