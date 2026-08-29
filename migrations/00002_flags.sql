-- +goose Up
CREATE TABLE flags(
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "key" varchar(128) NOT NULL UNIQUE CHECK (length(trim(key)) > 0),
    name varchar(256) NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
    description varchar(4096) NULL,
    "type" varchar(8) NOT NULL CHECK (type IN ('string', 'number', 'bool')),
    default_value jsonb NOT NULL,
    created_by bigint NOT NULL,
    updated_by bigint NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT flags_default_value_type CHECK ((type = 'string' AND jsonb_typeof(default_value) = 'string') OR (type = 'number' AND jsonb_typeof(default_value) = 'number') OR (type = 'bool' AND jsonb_typeof(default_value) = 'boolean')),
    CONSTRAINT fg_flags_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fg_flags_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TRIGGER flags_updated_at
    BEFORE UPDATE ON flags
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TABLE flags;