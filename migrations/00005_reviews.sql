-- +goose Up
CREATE TABLE approver_groups(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name varchar(256) NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
    description varchar(4096) NULL,
    min_approvals int NOT NULL CHECK (min_approvals >= 1),
    status varchar(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_groups_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_groups_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TABLE approver_group_members(
    group_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_group_members PRIMARY KEY (group_id, user_id),
    CONSTRAINT fk_members_group FOREIGN KEY (group_id) REFERENCES approver_groups(id) ON DELETE CASCADE,
    CONSTRAINT fk_members_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TABLE experimenter_groups(
    experimenter_id uuid PRIMARY KEY,
    group_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_expgroups_experimenter FOREIGN KEY (experimenter_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_expgroups_group FOREIGN KEY (group_id) REFERENCES approver_groups(id) ON DELETE RESTRICT
);

CREATE TABLE reviews(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    experiment_id uuid NOT NULL,
    version_id uuid NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'approved', 'changes_requested', 'rejected')),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_reviews_experiment FOREIGN KEY (experiment_id) REFERENCES experiments(id) ON DELETE CASCADE,
    CONSTRAINT fk_reviews_version FOREIGN KEY (version_id) REFERENCES experiment_versions(id) ON DELETE CASCADE,
    CONSTRAINT fk_reviews_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT uq_reviews_version UNIQUE (version_id)
);

CREATE TABLE review_approvals(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    review_id uuid NOT NULL,
    reviewer_id uuid NOT NULL,
    decision varchar(16) NOT NULL CHECK (decision IN ('approve', 'request_changes', 'reject')),
    comment varchar(4096) NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_approvals_review FOREIGN KEY (review_id) REFERENCES reviews(id) ON DELETE CASCADE,
    CONSTRAINT fk_approvals_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT uq_approvals_reviewer UNIQUE (review_id, reviewer_id)
);

CREATE INDEX idx_approvals_review ON review_approvals(review_id);

CREATE TABLE review_comments(
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    review_id uuid NOT NULL,
    author_id uuid NOT NULL,
    parent_id uuid NULL,
    body varchar(4096) NOT NULL CHECK (length(trim(body)) > 0),
    resolved boolean NOT NULL DEFAULT FALSE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz NULL,
    CONSTRAINT fk_comments_review FOREIGN KEY (review_id) REFERENCES reviews(id) ON DELETE CASCADE,
    CONSTRAINT fk_comments_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_comments_parent FOREIGN KEY (parent_id) REFERENCES review_comments(id) ON DELETE CASCADE,
    CONSTRAINT chk_comments_no_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX idx_comments_review ON review_comments(review_id);

ALTER TABLE experiments
    DROP CONSTRAINT experiments_status_check;

ALTER TABLE experiments
    ADD CONSTRAINT experiments_status_check CHECK (status IN ('draft', 'review', 'approved', 'running', 'paused', 'completed', 'archived', 'rejected'));

ALTER TABLE experiment_versions
    ADD CONSTRAINT fk_versions_review FOREIGN KEY (review_id) REFERENCES reviews(id) ON DELETE SET NULL;

CREATE TRIGGER reviews_updated_at
    BEFORE UPDATE ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER review_comments_updated_at
    BEFORE UPDATE ON review_comments
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER approver_groups_updated_at
    BEFORE UPDATE ON approver_groups
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS approver_groups_updated_at ON approver_groups;

DROP TRIGGER IF EXISTS review_comments_updated_at ON review_comments;

DROP TRIGGER IF EXISTS reviews_updated_at ON reviews;

ALTER TABLE experiment_versions
    DROP CONSTRAINT IF EXISTS fk_versions_review;

ALTER TABLE experiments
    DROP CONSTRAINT IF EXISTS experiments_status_check;

ALTER TABLE experiments
    ADD CONSTRAINT experiments_status_check CHECK (status IN ('draft', 'review', 'approved', 'running', 'paused', 'completed', 'archived'));

DROP TABLE review_comments;

DROP TABLE review_approvals;

DROP TABLE reviews;

DROP TABLE experimenter_groups;

DROP TABLE approver_group_members;

DROP TABLE approver_groups;

