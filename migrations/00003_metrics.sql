-- +goose Up
CREATE TABLE metrics(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    "key" varchar(128) NOT NULL UNIQUE CHECK (length(trim(key)) > 0),
    name varchar(256) NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
    description varchar(4096) NULL,
    metric_type varchar(16) NOT NULL CHECK (metric_type IN ('count', 'sum', 'unique_count', 'ratio', 'percentile', 'average')),
    aggregation jsonb NOT NULL,
    attribution jsonb NOT NULL,
    is_builtin boolean NOT NULL DEFAULT FALSE,
    status varchar(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fg_metrics_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fg_metrics_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TRIGGER metrics_updated_at
    BEFORE UPDATE ON metrics
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TABLE metrics;

