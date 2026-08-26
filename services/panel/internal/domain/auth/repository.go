package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")

type AuthRepo interface {
	GetCredentialsByEmail(ctx context.Context, email string) (id int64, role string, passwordHash string, err error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const selectCredentialsByEmail = `
SELECT id, role, password_hash
FROM users
WHERE email = $1 AND deleted_at IS NULL`

func (r *Repository) GetCredentialsByEmail(ctx context.Context, email string) (int64, string, string, error) {
	var (
		id   int64
		role string
		hash string
	)

	err := r.db.QueryRow(ctx, selectCredentialsByEmail, email).Scan(&id, &role, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", "", ErrNotFound
		}
		return 0, "", "", err
	}

	return id, role, hash, nil
}
