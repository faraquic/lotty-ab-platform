package events

import (
	"context"
	"errors"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyRecord struct {
	EventID     string
	RequestHash []byte
	Response    json.RawMessage
	ExpiresAt   time.Time
}

func (r IdempotencyRecord) Expired(now time.Time) bool {
	return !r.ExpiresAt.After(now)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db}
}

func (r *Repository) ListActiveEventTypes(ctx context.Context) ([]EventType, error) {
	const q = `
SELECT
    key,
    name,
    description,
    schema_version,
    require_exposure,
    status
FROM
    event_types
WHERE
    status = 'active'
ORDER BY
    key`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EventType

	for rows.Next() {
		var t EventType
		if err := rows.Scan(&t.Key, &t.Name, &t.Description, &t.SchemaVersion, &t.RequireExposure, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}

	return out, rows.Err()
}

func (r *Repository) FindIdempotencyKey(ctx context.Context, eventID string) (IdempotencyRecord, error) {
	const q = `
SELECT
    event_id,
    request_hash,
    response,
    expires_at
FROM
    event_idempotency_keys
WHERE
    event_id = $1`

	var rec IdempotencyRecord

	err := r.db.QueryRow(ctx, q, eventID).Scan(&rec.EventID, &rec.RequestHash, &rec.Response, &rec.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IdempotencyRecord{}, ErrIdempotencyNotFound
		}

		return IdempotencyRecord{}, err
	}

	return rec, nil
}

func (r *Repository) InsertIdempotencyKey(ctx context.Context, rec IdempotencyRecord) error {
	const q = `
INSERT INTO event_idempotency_keys(event_id, request_hash, response, expires_at)
    VALUES ($1, $2, $3, $4)`

	_, err := r.db.Exec(ctx, q, rec.EventID, rec.RequestHash, rec.Response, rec.ExpiresAt)

	return err
}

func (r *Repository) ReplaceIdempotencyKey(ctx context.Context, rec IdempotencyRecord) error {
	const q = `
UPDATE
    event_idempotency_keys
SET
    request_hash = $2,
    response = $3,
    created_at = now(),
    expires_at = $4
WHERE
    event_id = $1`

	tag, err := r.db.Exec(ctx, q, rec.EventID, rec.RequestHash, rec.Response, rec.ExpiresAt)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrIdempotencyNotFound
	}

	return nil
}

func (r *Repository) DeleteIdempotencyKeys(ctx context.Context, eventIDs []string) error {
	const q = `DELETE FROM event_idempotency_keys WHERE event_id = ANY($1)`

	_, err := r.db.Exec(ctx, q, eventIDs)

	return err
}

func AcceptedResponse() json.RawMessage {
	return json.RawMessage(`{"status":"accepted"}`)
}
