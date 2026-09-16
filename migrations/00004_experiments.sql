-- +goose Up
CREATE TABLE experiments(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    flag_id uuid NOT NULL,
    basic_experiment_id uuid NULL,
    name varchar(256) NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
    description varchar(4096) NULL,
    status varchar(16) NOT NULL CHECK (status IN ('draft', 'review', 'approved', 'running', 'paused', 'completed', 'archived')),
    current_version_id uuid NULL,
    owner_id uuid NOT NULL,
    version int NOT NULL DEFAULT 1 CHECK (version > 0),
    guardrail_paused boolean NOT NULL DEFAULT FALSE,
    completion_decision varchar(16) NULL CHECK (completion_decision IN ('rollout_winner', 'rollback', 'no_effect')),
    completion_reason varchar(4096) NULL,
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_experiments_flag FOREIGN KEY (flag_id) REFERENCES flags(id) ON DELETE RESTRICT,
    CONSTRAINT fk_experiments_basic_experiment FOREIGN KEY (basic_experiment_id) REFERENCES experiments(id) ON DELETE RESTRICT,
    CONSTRAINT fk_experiments_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_experiments_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_experiments_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX uniq_active_experiment_per_flag ON experiments(flag_id)
WHERE
    status IN ('running', 'paused');

CREATE TABLE experiment_versions(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    experiment_id uuid NOT NULL,
    version_num int NOT NULL,
    review_id uuid NULL,
    weights_total int NOT NULL DEFAULT 10000 CHECK (weights_total > 0 AND weights_total <= 10000),
    targeting_expr jsonb NULL,
    distribution_salt text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid NOT NULL,
    CONSTRAINT fk_versions_experiment FOREIGN KEY (experiment_id) REFERENCES experiments(id) ON DELETE CASCADE,
    -- Stub for Phase 8 (review workflow): FK to reviews(id) added when the reviews table lands.
    CONSTRAINT fk_versions_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT uq_versions_experiment_num UNIQUE (experiment_id, version_num)
);

CREATE INDEX idx_versions_experiment ON experiment_versions(experiment_id);

ALTER TABLE experiments
    ADD CONSTRAINT fk_experiments_current_version FOREIGN KEY (current_version_id) REFERENCES experiment_versions(id) ON DELETE SET NULL;

CREATE TABLE variants(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    version_id uuid NOT NULL,
    name varchar(128) NOT NULL,
    value jsonb NOT NULL,
    weight_bp int NOT NULL,
    is_control boolean NOT NULL DEFAULT FALSE,
    CONSTRAINT fk_variants_version FOREIGN KEY (version_id) REFERENCES experiment_versions(id) ON DELETE CASCADE,
    CONSTRAINT chk_variant_name CHECK (length(trim(name)) > 0),
    CONSTRAINT chk_variant_weight_positive CHECK (weight_bp > 0),
    CONSTRAINT uq_variants_version_name UNIQUE (version_id, name)
);

CREATE INDEX idx_variants_version ON variants(version_id);

CREATE UNIQUE INDEX uniq_variants_one_control_per_version ON variants(version_id)
WHERE
    is_control;

--    Инварианты «сумма весов = weights_total» и «≥ 2 варианта».
--    Отложенный constraint trigger: проверка сработает на COMMIT,
--    поэтому варианты можно вставлять по одному в одной транзакции.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_version_variants_invariants()
    RETURNS TRIGGER
    LANGUAGE plpgsql
    AS $$
DECLARE
    v_version_id uuid := COALESCE(NEW.version_id, OLD.version_id);
    v_total int;
    v_sum int;
    v_cnt int;
BEGIN
    SELECT
        weights_total
    INTO
        v_total
    FROM
        experiment_versions
    WHERE
        id = v_version_id;
    IF v_total IS NULL THEN
        RETURN NULL;
    END IF;
    SELECT
        COALESCE(SUM(weight_bp), 0),
        COUNT(*)
    INTO
        v_sum,
        v_cnt
    FROM
        variants
    WHERE
        version_id = v_version_id;
    IF v_cnt > 0 AND v_sum <> v_total THEN
        RAISE EXCEPTION 'sum(weight_bp)=% does not equal weights_total=% for version %', v_sum, v_total, v_version_id;
    END IF;
    IF v_cnt > 0 AND v_cnt < 2 THEN
        RAISE EXCEPTION 'version % requires at least 2 variants (got %)', v_version_id, v_cnt;
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER trg_variants_invariants
    AFTER INSERT OR UPDATE OR DELETE ON variants DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_version_variants_invariants();

CREATE TRIGGER experiments_updated_at
    BEFORE UPDATE ON experiments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS experiments_updated_at ON experiments;

DROP TRIGGER IF EXISTS trg_variants_invariants ON variants;

DROP FUNCTION IF EXISTS check_version_variants_invariants();

DROP TABLE variants;

ALTER TABLE experiments
    DROP CONSTRAINT IF EXISTS fk_experiments_current_version;

DROP TABLE experiment_versions;

DROP TABLE experiments;

