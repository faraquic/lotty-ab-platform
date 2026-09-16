package reviews

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db}
}

func mapGroupUnique(err error) error {
	if database.IsUniqueViolation(err) && strings.Contains(err.Error(), "approver_groups_name_key") {
		return ErrConflictName
	}
	return err
}

func (r *Repository) CreateGroup(ctx context.Context, name string, description *string, minApprovals int, callerID string) (string, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	id := uid.String()
	const q = `
INSERT INTO approver_groups(id, name, description, min_approvals, created_by, updated_by)
    VALUES ($1, $2, $3, $4, $5, $5)
RETURNING
    id`
	var out string
	if err := r.db.QueryRow(ctx, q, id, name, description, minApprovals, callerID).Scan(&out); err != nil {
		return "", mapGroupUnique(err)
	}
	return out, nil
}

func scanGroupMembers(rows pgx.Rows) ([]users.User, error) {
	var out []users.User
	for rows.Next() {
		var u users.User
		var role users.Role
		var avatar *string
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &role, &avatar, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Role = role
		if avatar != nil {
			u.AvatarURL = *avatar
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

const membersByGroupQ = `
SELECT
    u.id, u.full_name, u.email, u.role, u.avatar_url, u.created_at, u.updated_at
FROM
    approver_group_members m
    JOIN users u ON m.user_id = u.id
WHERE
    m.group_id = $1
ORDER BY
    u.full_name`

func (r *Repository) listMembers(ctx context.Context, groupID string) ([]users.User, error) {
	rows, err := r.db.Query(ctx, membersByGroupQ, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroupMembers(rows)
}

const groupColumns = `
    g.id,
    g.name,
    g.description,
    g.min_approvals,
    g.status,
    g.created_by::text,
    g.updated_by::text,
    g.created_at,
    g.updated_at`

func scanGroup(row pgx.Row) (ApproverGroup, error) {
	var g ApproverGroup
	var status GroupStatus
	err := row.Scan(
		&g.ID,
		&g.Name,
		&g.Description,
		&g.MinApprovals,
		&status,
		&g.CreatedBy,
		&g.UpdatedBy,
		&g.CreatedAt,
		&g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ApproverGroup{}, ErrGroupNotFound
		}
		return ApproverGroup{}, err
	}
	g.Status = status
	return g, nil
}

func (r *Repository) GetGroup(ctx context.Context, id string, includeArchived bool) (ApproverGroup, error) {
	q := `SELECT ` + groupColumns + ` FROM approver_groups g WHERE g.id = $1`
	if !includeArchived {
		q += ` AND g.status = 'active'`
	}
	g, err := scanGroup(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return ApproverGroup{}, err
	}
	members, err := r.listMembers(ctx, id)
	if err != nil {
		return ApproverGroup{}, err
	}
	g.Members = members
	return g, nil
}

func (r *Repository) ListGroups(ctx context.Context, limit, offset int, includeArchived bool) ([]ApproverGroup, error) {
	q := `SELECT ` + groupColumns + ` FROM approver_groups g`
	if !includeArchived {
		q += ` WHERE g.status = 'active'`
	}
	q += ` ORDER BY g.name LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ApproverGroup
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		members, err := r.listMembers(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.Members = members
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repository) CountGroups(ctx context.Context, includeArchived bool) (int64, error) {
	q := `SELECT count(*) FROM approver_groups`
	if !includeArchived {
		q += ` WHERE status = 'active'`
	}
	var n int64
	err := r.db.QueryRow(ctx, q).Scan(&n)
	return n, err
}

func (r *Repository) UpdateGroup(ctx context.Context, id, name string, description *string, minApprovals *int, status GroupStatus, updatedBy string) (ApproverGroup, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ApproverGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var desc any
	if description != nil {
		desc = *description
	}
	var minAny any
	if minApprovals != nil {
		minAny = *minApprovals
	}
	var statusAny any
	if status != "" {
		statusAny = string(status)
	}

	const q = `
UPDATE
    approver_groups
SET
    name = COALESCE(NULLIF($1, ''), name),
    description = COALESCE($2, description),
    min_approvals = COALESCE($3, min_approvals),
    status = COALESCE($4, status),
    updated_by = $5
WHERE
    id = $6
RETURNING
    min_approvals`
	var newMin int
	if err := tx.QueryRow(ctx, q, name, desc, minAny, statusAny, updatedBy, id).Scan(&newMin); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ApproverGroup{}, ErrGroupNotFound
		}
		return ApproverGroup{}, mapGroupUnique(err)
	}

	var memberCount int
	const cntQ = `SELECT count(*) FROM approver_group_members WHERE group_id = $1`
	if err := tx.QueryRow(ctx, cntQ, id).Scan(&memberCount); err != nil {
		return ApproverGroup{}, err
	}
	if newMin > memberCount {
		return ApproverGroup{}, ErrInvalidThreshold
	}

	if err := tx.Commit(ctx); err != nil {
		return ApproverGroup{}, err
	}
	return r.GetGroup(ctx, id, true)
}

func (r *Repository) AddMember(ctx context.Context, groupID, userID string) error {
	const q = `INSERT INTO approver_group_members(group_id, user_id) VALUES ($1, $2)`
	if _, err := r.db.Exec(ctx, q, groupID, userID); err != nil {
		if database.IsUniqueViolation(err) {
			return ErrDuplicateMember
		}
		if database.IsForeignKeyViolation(err) {
			if strings.Contains(err.Error(), "fk_members_user") {
				return ErrUserNotFound
			}
			return ErrGroupNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) RemoveMember(ctx context.Context, groupID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM approver_group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}

	var minApprovals, memberCount int
	const q = `SELECT min_approvals FROM approver_groups WHERE id = $1`
	if err := tx.QueryRow(ctx, q, groupID).Scan(&minApprovals); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrGroupNotFound
		}
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM approver_group_members WHERE group_id = $1`, groupID).Scan(&memberCount); err != nil {
		return err
	}
	if minApprovals > memberCount {
		return ErrInvalidThreshold
	}

	return tx.Commit(ctx)
}

type GroupRule struct {
	GroupID      *string
	MinApprovals int
	MemberIDs    []string
}

func (r *Repository) GetRule(ctx context.Context, experimenterID string) (GroupRule, error) {
	const q = `
SELECT
    g.id, g.min_approvals
FROM
    experimenter_groups eg
    JOIN approver_groups g ON eg.group_id = g.id
WHERE
    eg.experimenter_id = $1 AND g.status = 'active'`
	var groupID string
	var minApprovals int
	err := r.db.QueryRow(ctx, q, experimenterID).Scan(&groupID, &minApprovals)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GroupRule{}, nil
		}
		return GroupRule{}, err
	}

	const mq = `SELECT user_id::text FROM approver_group_members WHERE group_id = $1`
	rows, err := r.db.Query(ctx, mq, groupID)
	if err != nil {
		return GroupRule{}, err
	}
	defer rows.Close()

	rule := GroupRule{GroupID: &groupID, MinApprovals: minApprovals}
	for rows.Next() {
		var memberID string
		if err := rows.Scan(&memberID); err != nil {
			return GroupRule{}, err
		}
		rule.MemberIDs = append(rule.MemberIDs, memberID)
	}
	return rule, rows.Err()
}

func (r *Repository) SetExperimenterGroup(ctx context.Context, experimenterID, groupID string) error {
	const q = `
INSERT INTO experimenter_groups(experimenter_id, group_id)
    VALUES ($1, $2)
ON CONFLICT (experimenter_id)
    DO UPDATE SET group_id = EXCLUDED.group_id`
	if _, err := r.db.Exec(ctx, q, experimenterID, groupID); err != nil {
		if database.IsForeignKeyViolation(err) {
			if strings.Contains(err.Error(), "fk_expgroups_group") {
				return ErrGroupNotFound
			}
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) UnsetExperimenterGroup(ctx context.Context, experimenterID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM experimenter_groups WHERE experimenter_id = $1`, experimenterID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *Repository) CreateReview(ctx context.Context, experimentID, versionID, callerID string) (string, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	id := uid.String()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insQ = `
INSERT INTO reviews(id, experiment_id, version_id, created_by)
    VALUES ($1, $2, $3, $4)`
	if _, err = tx.Exec(ctx, insQ, id, experimentID, versionID, callerID); err != nil {
		if database.IsUniqueViolation(err) {
			return "", ErrReviewExists
		}
		return "", err
	}

	const linkQ = `UPDATE experiment_versions SET review_id = $1 WHERE id = $2`
	if _, err = tx.Exec(ctx, linkQ, id, versionID); err != nil {
		return "", err
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func scanReview(row pgx.Row) (Review, error) {
	var r Review
	err := row.Scan(
		&r.ID,
		&r.ExperimentID,
		&r.VersionID,
		&r.VersionNum,
		&r.Status,
		&r.CreatedBy,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Review{}, ErrNotFound
		}
		return Review{}, err
	}
	return r, nil
}

const reviewColumns = `
    r.id,
    r.experiment_id,
    r.version_id,
    v.version_num,
    r.status,
    r.created_by::text,
    r.created_at,
    r.updated_at`

func (r *Repository) listApprovals(ctx context.Context, reviewID string) ([]Approval, error) {
	const q = `
SELECT
    id, review_id, reviewer_id::text, decision, comment, created_at
FROM
    review_approvals
WHERE
    review_id = $1
ORDER BY
    created_at`
	rows, err := r.db.Query(ctx, q, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Approval
	for rows.Next() {
		var a Approval
		var decision ApprovalDecision
		if err := rows.Scan(&a.ID, &a.ReviewID, &a.ReviewerID, &decision, &a.Comment, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Decision = decision
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) listComments(ctx context.Context, reviewID string, includeDeleted bool) ([]Comment, error) {
	q := `
SELECT
    id, review_id, author_id::text, parent_id::text, body, resolved, created_at, updated_at
FROM
    review_comments
WHERE
    review_id = $1`
	if !includeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	q += ` ORDER BY created_at`
	rows, err := r.db.Query(ctx, q, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ReviewID, &c.AuthorID, &c.ParentID, &c.Body, &c.Resolved, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) GetReview(ctx context.Context, id string) (Review, error) {
	q := `SELECT ` + reviewColumns + `
FROM
    reviews r
    JOIN experiment_versions v ON r.version_id = v.id
WHERE
    r.id = $1`
	rev, err := scanReview(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return Review{}, err
	}
	approvals, err := r.listApprovals(ctx, id)
	if err != nil {
		return Review{}, err
	}
	rev.Approvals = approvals
	comments, err := r.listComments(ctx, id, false)
	if err != nil {
		return Review{}, err
	}
	rev.Comments = comments
	return rev, nil
}

func (r *Repository) GetVersionReview(ctx context.Context, versionID string) (Review, error) {
	q := `SELECT ` + reviewColumns + `
FROM
    reviews r
    JOIN experiment_versions v ON r.version_id = v.id
WHERE
    r.version_id = $1`
	rev, err := scanReview(r.db.QueryRow(ctx, q, versionID))
	if err != nil {
		return Review{}, err
	}
	approvals, err := r.listApprovals(ctx, rev.ID)
	if err != nil {
		return Review{}, err
	}
	rev.Approvals = approvals
	return rev, nil
}

func (r *Repository) ListReviews(ctx context.Context, limit, offset int, status ReviewStatus) ([]Review, error) {
	q := `SELECT ` + reviewColumns + `
FROM
    reviews r
    JOIN experiment_versions v ON r.version_id = v.id`
	var args []any
	if status != "" {
		q += ` WHERE r.status = $1`
		args = append(args, string(status))
	}
	q += ` ORDER BY r.created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Review
	for rows.Next() {
		rev, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		approvals, err := r.listApprovals(ctx, rev.ID)
		if err != nil {
			return nil, err
		}
		rev.Approvals = approvals
		out = append(out, rev)
	}
	return out, rows.Err()
}

func (r *Repository) CountReviews(ctx context.Context, status ReviewStatus) (int64, error) {
	q := `SELECT count(*) FROM reviews`
	var args []any
	if status != "" {
		q += ` WHERE status = $1`
		args = append(args, string(status))
	}
	var n int64
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

func (r *Repository) ActOnReview(ctx context.Context, reviewID, reviewerID string, decision ApprovalDecision, comment *string, threshold int) (ReviewStatus, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status ReviewStatus
	const lockQ = `SELECT status FROM reviews WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, lockQ, reviewID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if status != ReviewOpen {
		return "", ErrReviewClosed
	}

	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	const insQ = `
INSERT INTO review_approvals(id, review_id, reviewer_id, decision, comment)
    VALUES ($1, $2, $3, $4, $5)`
	if _, err = tx.Exec(ctx, insQ, uid.String(), reviewID, reviewerID, string(decision), comment); err != nil {
		if database.IsUniqueViolation(err) {
			return "", ErrDuplicateAction
		}
		return "", err
	}

	final := status
	switch decision {
	case DecisionReject:
		final = ReviewRejected
	case DecisionRequestChanges:
		final = ReviewChangesRequested
	case DecisionApprove:
		var approves int
		const cntQ = `SELECT count(*) FROM review_approvals WHERE review_id = $1 AND decision = 'approve'`
		if err := tx.QueryRow(ctx, cntQ, reviewID).Scan(&approves); err != nil {
			return "", err
		}
		if approves >= threshold {
			final = ReviewApproved
		}
	}

	if final != ReviewOpen {
		const finQ = `UPDATE reviews SET status = $1 WHERE id = $2 AND status = 'open'`
		tag, err := tx.Exec(ctx, finQ, string(final), reviewID)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			var current ReviewStatus
			if err := tx.QueryRow(ctx, `SELECT status FROM reviews WHERE id = $1`, reviewID).Scan(&current); err != nil {
				return "", err
			}
			final = current
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return final, nil
}

func (r *Repository) AddComment(ctx context.Context, reviewID, authorID string, parentID *string, body string) (string, error) {
	if parentID != nil {
		var parentReview string
		var parentParent *string
		const pq = `SELECT review_id, parent_id::text FROM review_comments WHERE id = $1 AND deleted_at IS NULL`
		if err := r.db.QueryRow(ctx, pq, *parentID).Scan(&parentReview, &parentParent); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", ErrInvalidComment
			}
			return "", err
		}
		if parentReview != reviewID || parentParent != nil {
			return "", ErrInvalidComment
		}
	}

	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	id := uid.String()
	const q = `
INSERT INTO review_comments(id, review_id, author_id, parent_id, body)
    VALUES ($1, $2, $3, $4, $5)`
	if _, err := r.db.Exec(ctx, q, id, reviewID, authorID, parentID, body); err != nil {
		if database.IsForeignKeyViolation(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	return id, nil
}

func (r *Repository) GetComment(ctx context.Context, id string) (Comment, error) {
	const q = `
SELECT
    id, review_id, author_id::text, parent_id::text, body, resolved, created_at, updated_at
FROM
    review_comments
WHERE
    id = $1 AND deleted_at IS NULL`
	var c Comment
	err := r.db.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.ReviewID, &c.AuthorID, &c.ParentID, &c.Body, &c.Resolved, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}
	return c, nil
}

func (r *Repository) ResolveComment(ctx context.Context, reviewID, commentID string, resolved bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE review_comments SET resolved = $1 WHERE id = $2 AND review_id = $3 AND deleted_at IS NULL`,
		resolved, commentID, reviewID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func (r *Repository) DeleteComment(ctx context.Context, reviewID, commentID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE review_comments SET deleted_at = now() WHERE id = $1 AND review_id = $2 AND deleted_at IS NULL`,
		commentID, reviewID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCommentNotFound
	}
	return nil
}
