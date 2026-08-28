package flags

import (
	"context"
	"errors"

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
    id
`

	var id int64
	err := r.db.QueryRow(ctx, q, f.Key, f.Type, f.DefaultValue, f.Description, f.Owner).Scan(&id)
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
    f.id AS flag_id,
    f.key,
    f.type,
    f.default_value,
    f.description,
    f.owner,
    f.deleted_at,
    f.created_at AS flag_created_at,
    f.updated_at AS flag_updated_at,
    u.id AS user_id,
    u.username,
    u.email,
    u.role,
    u.avatar_url,
    u.created_at AS user_created_at,
    u.updated_at AS user_updated_at
FROM
    %s f
    JOIN users u ON f.owner = u.id`

func scanFlagWithOwner(row pgx.Row) (FlagWithOwner, error) {
	var result FlagWithOwner
	err := row.Scan(
		&result.Flag.ID,
		&result.Flag.Key,
		&result.Flag.Type,
		&result.Flag.DefaultValue,
		&result.Flag.Description,
		&result.Flag.Owner,
		&result.Flag.DeletedAt,
		&result.Flag.CreatedAt,
		&result.Flag.UpdatedAt,
		&result.Owner.ID,
		&result.Owner.Username,
		&result.Owner.Email,
		&result.Owner.Role,
		&result.Owner.AvatarURL,
		&result.Owner.CreatedAt,
		&result.Owner.UpdatedAt,
	)
	return result, err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (FlagWithOwner, error) {
	q := selectFlagWithOwner + `
WHERE
    f.id = $1
    AND f.deleted_at IS NULL`

	result, err := scanFlagWithOwner(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FlagWithOwner{}, ErrNotFound
		}
		return FlagWithOwner{}, err
	}

	return result, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]FlagWithOwner, error) {
	q := selectFlagWithOwner + `
WHERE f.deleted_at IS NULL
ORDER BY f.id
LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]FlagWithOwner, 0, limit)

	for rows.Next() {
		var item FlagWithOwner
		err := rows.Scan(
			&item.Flag.ID,
			&item.Flag.Key,
			&item.Flag.Type,
			&item.Flag.DefaultValue,
			&item.Flag.Description,
			&item.Flag.Owner,
			&item.Flag.DeletedAt,
			&item.Flag.CreatedAt,
			&item.Flag.UpdatedAt,
			&item.Owner.ID,
			&item.Owner.Username,
			&item.Owner.Email,
			&item.Owner.Role,
			&item.Owner.AvatarURL,
			&item.Owner.CreatedAt,
			&item.Owner.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repository) Update(ctx context.Context, id int64, key string, description string) (FlagWithOwner, error) {
	q := `WITH updated AS (
    UPDATE flags
    SET
        key = COALESCE($2, key),
        description = COALESCE($3, description)
    WHERE
        id = $1
        AND deleted_at IS NULL
    RETURNING *
)` + selectFlagWithOwner + `
WHERE f.id = updated.id`

	result, err := scanFlagWithOwner(r.db.QueryRow(ctx, q, id, key, description))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FlagWithOwner{}, ErrNotFound
		}
		if pgconnutils.IsUniqueViolation(err) {
			return FlagWithOwner{}, ErrConflictKeys
		}
		return FlagWithOwner{}, err
	}
	return result, nil
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
