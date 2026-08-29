package flags

import (
	"context"
	"errors"
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
	pgconnutils "github.com/faraquic/lotty-ab-platform/services/panel/internal/lib/pgconn-utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("flag not found")
	ErrConflictKeys = errors.New("flag with this key already exists")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db}
}

func (r *Repository) Create(ctx context.Context, f Flag) (int64, error) {
	const q = `
INSERT INTO flags(key, type, default_value, description, owner)
    VALUES ($1, $2, $3, $4, $5)
RETURNING
    id`

	var id int64
	var desc string
	if f.Description != nil {
		desc = *f.Description
	}
	err := r.db.QueryRow(ctx, q, f.Key, f.Type, f.DefaultValue, desc, f.Owner).Scan(&id)
	if err != nil {
		if pgconnutils.IsUniqueViolation(err) {
			return 0, ErrConflictKeys
		}
		return 0, err
	}
	return id, nil
}

const selectFlagWithOwner = `
SELECT
    f.id, f.key, f.type, f.default_value, f.description,
    f.owner, f.deleted_at, f.created_at, f.updated_at,
    u.id, u.username, u.email, u.role, u.avatar_url,
    u.created_at, u.updated_at
FROM flags f JOIN users u ON f.owner = u.id`

type flagWithOwnerRow struct {
	FlagID       int64
	Key          string
	Type         TypeFlag
	DefaultValue ValueFlag
	Description  string
	OwnerID      int64
	DeletedAt    *time.Time
	FlagCreated  time.Time
	FlagUpdated  time.Time
	UserID       int64
	Username     string
	Email        string
	Role         string
	AvatarURL    *string
	UserCreated  time.Time
	UserUpdated  time.Time
}

func (r flagWithOwnerRow) toFlagWithOwner() FlagWithOwner {
	var desc *string
	if r.Description != "" {
		desc = &r.Description
	}
	return FlagWithOwner{
		Flag: Flag{
			ID:           r.FlagID,
			Key:          r.Key,
			Type:         r.Type,
			DefaultValue: r.DefaultValue,
			Description:  desc,
			Owner:        r.OwnerID,
			DeletedAt:    r.DeletedAt,
			CreatedAt:    r.FlagCreated,
			UpdatedAt:    r.FlagUpdated,
		},
		Owner: &users.User{
			ID:        r.UserID,
			Username:  r.Username,
			Email:     r.Email,
			Role:      users.Role(r.Role),
			CreatedAt: r.UserCreated,
			UpdatedAt: r.UserUpdated,
		},
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (FlagWithOwner, error) {
	const q = selectFlagWithOwner + `
WHERE
    f.id = $1
    AND f.deleted_at IS NULL`

	var row flagWithOwnerRow
	err := r.db.QueryRow(ctx, q, id).Scan(
		&row.FlagID,
		&row.Key,
		&row.Type,
		&row.DefaultValue,
		&row.Description,
		&row.OwnerID,
		&row.DeletedAt,
		&row.FlagCreated,
		&row.FlagUpdated,
		&row.UserID,
		&row.Username,
		&row.Email,
		&row.Role,
		&row.AvatarURL,
		&row.UserCreated,
		&row.UserUpdated,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FlagWithOwner{}, ErrNotFound
		}
		return FlagWithOwner{}, err
	}

	fwo := row.toFlagWithOwner()
	if row.AvatarURL != nil {
		fwo.Owner.AvatarURL = *row.AvatarURL
	}
	return fwo, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]Flag, error) {
	const q = `
SELECT
    *
FROM
    flags
WHERE
    deleted_at IS NULL
ORDER BY
    id
LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[Flag])
}

func (r *Repository) Update(ctx context.Context, id int64, key string, defaultValue ValueFlag, description string) (FlagWithOwner, error) {
	var dv any
	if len(defaultValue) > 0 {
		dv = string(defaultValue)
	}
	const q = `
UPDATE
    flags
SET
    key = COALESCE(NULLIF($2, ''), key),
    default_value = COALESCE($3, default_value),
    description = COALESCE(NULLIF($4, ''), description)
WHERE
    id = $1
    AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, id, key, dv, description)
	if err != nil {
		if pgconnutils.IsUniqueViolation(err) {
			return FlagWithOwner{}, ErrConflictKeys
		}
		return FlagWithOwner{}, err
	}
	if tag.RowsAffected() == 0 {
		return FlagWithOwner{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const q = `
UPDATE
    flags
SET
    deleted_at = now()
WHERE
    id = $1
    AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	const q = `
SELECT
    count(*)
FROM
    flags
WHERE
    deleted_at IS NULL`

	var n int64
	err := r.db.QueryRow(ctx, q).Scan(&n)
	return n, err
}
