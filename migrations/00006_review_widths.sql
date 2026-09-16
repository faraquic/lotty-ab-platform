-- +goose Up
ALTER TABLE reviews
    ALTER COLUMN status TYPE varchar(32);

ALTER TABLE review_approvals
    ALTER COLUMN decision TYPE varchar(32);

-- +goose Down
ALTER TABLE review_approvals
    ALTER COLUMN decision TYPE varchar(16);

ALTER TABLE reviews
    ALTER COLUMN status TYPE varchar(16);
