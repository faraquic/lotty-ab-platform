-- +goose Up
CREATE TABLE audit_records(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    actor_type varchar(32) NOT NULL,
    actor_id uuid,
    action varchar(128) NOT NULL,
    resource_type varchar(64) NOT NULL,
    resource_id uuid NOT NULL,
    reason varchar(4096),
    before_state jsonb,
    after_state jsonb,
    request_id uuid,
    trace_id varchar(256),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT audit_records_actor_type_check CHECK ((actor_type = 'user' AND actor_id IS NOT NULL) OR (actor_type = 'system' AND actor_id IS NULL))
);

CREATE INDEX idx_audit_records_resource_created ON audit_records(resource_type, resource_id, created_at DESC);

-- +goose StatementBegin
CREATE FUNCTION prevent_audit_record_mutation()
    RETURNS TRIGGER
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'audit_records is append-only';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_records_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_records
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_record_mutation();

CREATE TRIGGER audit_records_no_truncate
    BEFORE TRUNCATE ON audit_records
    FOR EACH STATEMENT
    EXECUTE FUNCTION prevent_audit_record_mutation();

-- +goose Down
DROP TRIGGER audit_records_no_truncate ON audit_records;

DROP TRIGGER audit_records_no_update_delete ON audit_records;

DROP FUNCTION prevent_audit_record_mutation();

DROP TABLE audit_records;

