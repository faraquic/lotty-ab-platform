package users

import (
	"context"
	"errors"
	"strconv"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ListFilter narrows user listings by role and/or free-text search.
type ListFilter struct {
	Role   *Role
	Search *string
}

var (
	ErrNotFound = errors.New("user not found")
	ErrConflict = errors.New("user already exists")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, u User) (string, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	id := uid.String()
	const q = `
INSERT INTO users(id, full_name, email, password_hash, role)
    VALUES ($1, $2, $3, $4, $5)
RETURNING
    id`

	var outID string
	err = r.db.QueryRow(ctx, q, id, u.FullName, u.Email, u.PasswordHash, u.Role).Scan(&outID)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return "", ErrConflict
		}
		return "", err
	}

	return outID, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (User, error) {
	const q = `
SELECT
    id,
    full_name,
    email,
    password_hash,
    ROLE,
    COALESCE(avatar_url, ''),
    deleted_at,
    created_at,
    updated_at
FROM
    users
WHERE
    id = $1
    AND deleted_at IS NULL`

	var u User
	err := r.db.QueryRow(ctx, q, id).
		Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.Role, &u.AvatarURL, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	return u, nil
}

// List returns users matching the optional filters. The search term matches
// full_name or email (case-insensitive); role filters exactly.
func (r *Repository) List(ctx context.Context, limit, offset int, filter ListFilter) ([]User, error) {
	q := `
SELECT
    id,
    full_name,
    email,
    password_hash,
    ROLE,
    COALESCE(avatar_url, ''),
    deleted_at,
    created_at,
    updated_at
FROM
    users
WHERE
    deleted_at IS NULL`
	args := []any{}
	if filter.Role != nil {
		args = append(args, string(*filter.Role))
		q += ` AND role = $` + strconv.Itoa(len(args))
	}
	if filter.Search != nil {
		args = append(args, "%"+*filter.Search+"%")
		q += ` AND (full_name ILIKE $` + strconv.Itoa(len(args)) + ` OR email ILIKE $` + strconv.Itoa(len(args)) + `)`
	}
	q += ` ORDER BY id LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usersList := make([]User, 0, limit)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.Role, &u.AvatarURL, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		usersList = append(usersList, u)
	}

	return usersList, rows.Err()
}

func (r *Repository) Update(ctx context.Context, id string, email *string, role *Role) (User, error) {
	const q = `
UPDATE
    users
SET
    email = COALESCE($2, email),
    role = COALESCE($3, role)
WHERE
    id = $1
    AND deleted_at IS NULL
RETURNING
    id,
    full_name,
    email,
    password_hash,
    role,
    COALESCE(avatar_url, ''),
    deleted_at,
    created_at,
    updated_at`

	var u User
	err := r.db.QueryRow(ctx, q, id, email, role).
		Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.Role, &u.AvatarURL, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		if database.IsUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}

	return u, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `
UPDATE
    users
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

func (r *Repository) UpdateAvatarURL(ctx context.Context, id string, avatarURL string) error {
	const q = `
UPDATE
    users
SET
    avatar_url = $2
WHERE
    id = $1
    AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, id, avatarURL)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) Count(ctx context.Context, filter ListFilter) (int64, error) {
	q := `
SELECT
  count(*)
FROM
  users
WHERE
  deleted_at IS NULL`
	args := []any{}
	if filter.Role != nil {
		args = append(args, string(*filter.Role))
		q += ` AND role = $` + strconv.Itoa(len(args))
	}
	if filter.Search != nil {
		args = append(args, "%"+*filter.Search+"%")
		q += ` AND (full_name ILIKE $` + strconv.Itoa(len(args)) + ` OR email ILIKE $` + strconv.Itoa(len(args)) + `)`
	}

	var n int64
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)

	return n, err
}

func (r *Repository) CountAdmins(ctx context.Context) (int64, error) {
	const q = `
SELECT
    count(*)
FROM
    users
WHERE
    ROLE = 'admin'
    AND deleted_at IS NULL`

	var n int64
	err := r.db.QueryRow(ctx, q).Scan(&n)

	return n, err
}
