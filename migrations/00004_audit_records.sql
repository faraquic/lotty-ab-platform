-- +goose Up
CREATE TABLE audit_records(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    action_type varchar(16) NOT NULL CHECK (action_type IN ('create', 'update', 'delete')),
    action varchar(255) NOT NULL,
    object_type varchar(16) NOT NULL CHECK (object_type IN ('user', 'flag', 'metric', 'experiment')),
    object_id uuid NOT NULL,
    before_json jsonb,
    after_json jsonb,
    request_id uuid,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fg_audit_records_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX idx_audit_records_object ON audit_records(object_type, object_id);
CREATE INDEX idx_audit_records_created_at ON audit_records(created_at);
CREATE INDEX idx_audit_records_request_id ON audit_records(request_id) WHERE request_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS audit_records;
