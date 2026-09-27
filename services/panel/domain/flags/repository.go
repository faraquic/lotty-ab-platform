package flags

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound         = errors.New("flag not found")
	ErrActiveExperiment = errors.New("flag is used by an active experiment")
	ErrConflictKeys     = errors.New("flag with this key already exists")
	ErrConflictNames    = errors.New("flag with this name already exists")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db}
}

func (r *Repository) Create(ctx context.Context, f Flag) (string, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	id := uid.String()
	const q = `
INSERT INTO flags(id, key, name, type, default_value, description, created_by, updated_by)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    id`

	var outID string
	var desc string
	if f.Description != nil {
		desc = *f.Description
	}
	err = r.db.QueryRow(ctx, q, id, f.Key, f.Name, f.Type, f.DefaultValue, desc, f.CreatedBy, f.UpdatedBy).Scan(&outID)
	if err != nil {
		if database.IsUniqueViolation(err) {
			if strings.Contains(err.Error(), "flags_key_key") {
				return "", ErrConflictKeys
			}
			if strings.Contains(err.Error(), "flags_name_key") {
				return "", ErrConflictNames
			}
			return "", ErrConflictKeys
		}
		return "", err
	}
	return outID, nil
}

const selectFlagWithCreatorAndUpdater = `
SELECT
    f.id,
    f.key,
    f.name,
    f.type,
    f.default_value,
    f.description,
    f.created_by,
    f.updated_by,
    f.deleted_at,
    f.created_at,
    f.updated_at,
    cb.id,
    cb.full_name,
    cb.email,
    cb.role,
    cb.avatar_url,
    cb.created_at,
    cb.updated_at,
    ub.id,
    ub.full_name,
    ub.email,
    ub.role,
    ub.avatar_url,
    ub.created_at,
    ub.updated_at
FROM
    flags f
    JOIN users cb ON f.created_by = cb.id
    JOIN users ub ON f.updated_by = ub.id`

type flagWithCreatorAndUpdaterRow struct {
	FlagID          string
	Key             string
	Name            string
	Type            TypeFlag
	DefaultValue    ValueFlag
	Description     string
	CreatedByID     string
	UpdatedByID     string
	DeletedAt       *time.Time
	FlagCreated     time.Time
	FlagUpdated     time.Time
	CreatorID       string
	CreatorFullName string
	CreatorEmail    string
	CreatorRole     string
	CreatorAvatar   *string
	CreatorCreated  time.Time
	CreatorUpdated  time.Time
	UpdaterID       string
	UpdaterFullName string
	UpdaterEmail    string
	UpdaterRole     string
	UpdaterAvatar   *string
	UpdaterCreated  time.Time
	UpdaterUpdated  time.Time
}

func (r flagWithCreatorAndUpdaterRow) toFlagWithCreatorAndUpdater() FlagWithCreatorAndUpdater {
	var desc *string
	if r.Description != "" {
		desc = &r.Description
	}
	return FlagWithCreatorAndUpdater{
		Flag: Flag{
			ID:           r.FlagID,
			Key:          r.Key,
			Name:         r.Name,
			Type:         r.Type,
			DefaultValue: r.DefaultValue,
			Description:  desc,
			CreatedBy:    r.CreatedByID,
			UpdatedBy:    r.UpdatedByID,
			DeletedAt:    r.DeletedAt,
			CreatedAt:    r.FlagCreated,
			UpdatedAt:    r.FlagUpdated,
		},
		CreatedBy: &users.User{
			ID:        r.CreatorID,
			FullName:  r.CreatorFullName,
			Email:     r.CreatorEmail,
			Role:      users.Role(r.CreatorRole),
			CreatedAt: r.CreatorCreated,
			UpdatedAt: r.CreatorUpdated,
		},
		UpdatedBy: &users.User{
			ID:        r.UpdaterID,
			FullName:  r.UpdaterFullName,
			Email:     r.UpdaterEmail,
			Role:      users.Role(r.UpdaterRole),
			CreatedAt: r.UpdaterCreated,
			UpdatedAt: r.UpdaterUpdated,
		},
	}
}

func (r *Repository) GetByID(ctx context.Context, id string) (FlagWithCreatorAndUpdater, error) {
	const q = selectFlagWithCreatorAndUpdater + `
WHERE
    f.id = $1
    AND f.deleted_at IS NULL`

	var row flagWithCreatorAndUpdaterRow
	err := r.db.QueryRow(ctx, q, id).Scan(
		&row.FlagID,
		&row.Key,
		&row.Name,
		&row.Type,
		&row.DefaultValue,
		&row.Description,
		&row.CreatedByID,
		&row.UpdatedByID,
		&row.DeletedAt,
		&row.FlagCreated,
		&row.FlagUpdated,
		&row.CreatorID,
		&row.CreatorFullName,
		&row.CreatorEmail,
		&row.CreatorRole,
		&row.CreatorAvatar,
		&row.CreatorCreated,
		&row.CreatorUpdated,
		&row.UpdaterID,
		&row.UpdaterFullName,
		&row.UpdaterEmail,
		&row.UpdaterRole,
		&row.UpdaterAvatar,
		&row.UpdaterCreated,
		&row.UpdaterUpdated,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FlagWithCreatorAndUpdater{}, ErrNotFound
		}
		return FlagWithCreatorAndUpdater{}, err
	}

	fwo := row.toFlagWithCreatorAndUpdater()
	if row.CreatorAvatar != nil {
		fwo.CreatedBy.AvatarURL = *row.CreatorAvatar
	}
	if row.UpdaterAvatar != nil {
		fwo.UpdatedBy.AvatarURL = *row.UpdaterAvatar
	}
	return fwo, nil
}

var validFlagSortFields = map[string]string{
	"id":         "f.id",
	"key":        "f.key",
	"name":       "f.name",
	"type":       "f.type",
	"created_at": "f.created_at",
	"updated_at": "f.updated_at",
}

func (r *Repository) List(ctx context.Context, limit, offset int, filter ListFilter) ([]Flag, error) {
	q := `
SELECT
    f.id,
    f.key,
    f.name,
    f.type,
    f.default_value,
    f.description,
    f.created_by,
    f.updated_by,
    f.deleted_at,
    f.created_at,
    f.updated_at
FROM
    flags f
WHERE
    f.deleted_at IS NULL`
	args := []any{}
	if filter.Type != nil {
		args = append(args, *filter.Type)
		q += ` AND f.type = $` + strconv.Itoa(len(args))
	}
	if filter.CreatedBy != nil {
		args = append(args, *filter.CreatedBy)
		q += ` AND f.created_by = $` + strconv.Itoa(len(args))
	}
	if filter.Search != nil {
		args = append(args, "%"+*filter.Search+"%")
		q += ` AND (f.key ILIKE $` + strconv.Itoa(len(args)) + ` OR f.name ILIKE $` + strconv.Itoa(len(args)) + `)`
	}
	sortCol := validFlagSortFields[filter.Sort]
	if sortCol == "" {
		sortCol = "f.id"
	}
	dir := "ASC"
	if filter.Order == "desc" {
		dir = "DESC"
	}
	q += ` ORDER BY ` + sortCol + ` ` + dir + ` LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Flag
	for rows.Next() {
		var f Flag
		if err := rows.Scan(
			&f.ID,
			&f.Key,
			&f.Name,
			&f.Type,
			&f.DefaultValue,
			&f.Description,
			&f.CreatedBy,
			&f.UpdatedBy,
			&f.DeletedAt,
			&f.CreatedAt,
			&f.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func (r *Repository) ListAll(ctx context.Context) ([]Flag, error) {
	q := `
SELECT
    f.id,
    f.key,
    f.name,
    f.type,
    f.default_value,
    f.description,
    f.created_by,
    f.updated_by,
    f.deleted_at,
    f.created_at,
    f.updated_at
FROM
    flags f
WHERE
    f.deleted_at IS NULL
ORDER BY f.id`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Flag
	for rows.Next() {
		var f Flag
		if err := rows.Scan(
			&f.ID,
			&f.Key,
			&f.Name,
			&f.Type,
			&f.DefaultValue,
			&f.Description,
			&f.CreatedBy,
			&f.UpdatedBy,
			&f.DeletedAt,
			&f.CreatedAt,
			&f.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func (r *Repository) Update(ctx context.Context, id string, key string, name string, defaultValue ValueFlag, description string, updatedBy string) (FlagWithCreatorAndUpdater, error) {
	var dv any
	if len(defaultValue) > 0 {
		dv = string(defaultValue)
	}
	const q = `
UPDATE
    flags
SET
    key = COALESCE(NULLIF($2, ''), key),
    name = COALESCE(NULLIF($3, ''), name),
    default_value = COALESCE($4, default_value),
    description = COALESCE(NULLIF($5, ''), description),
    updated_by = $6
WHERE
    id = $1
    AND deleted_at IS NULL`

	tag, err := r.db.Exec(ctx, q, id, key, name, dv, description, updatedBy)
	if err != nil {
		if database.IsUniqueViolation(err) {
			if strings.Contains(err.Error(), "flags_key_key") {
				return FlagWithCreatorAndUpdater{}, ErrConflictKeys
			}
			if strings.Contains(err.Error(), "flags_name_key") {
				return FlagWithCreatorAndUpdater{}, ErrConflictNames
			}
			return FlagWithCreatorAndUpdater{}, ErrConflictKeys
		}
		return FlagWithCreatorAndUpdater{}, err
	}
	if tag.RowsAffected() == 0 {
		return FlagWithCreatorAndUpdater{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `
UPDATE
	flags AS f
SET
    deleted_at = now()
WHERE
	f.id = $1
	AND f.deleted_at IS NULL
	AND NOT EXISTS (
		SELECT 1
		FROM experiments e
		WHERE e.flag_id = f.id
			AND e.status IN ('running', 'paused')
	)`

	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		const existsQuery = `SELECT EXISTS (SELECT 1 FROM flags WHERE id = $1 AND deleted_at IS NULL)`
		if err := r.db.QueryRow(ctx, existsQuery, id).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrActiveExperiment
		}
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Count(ctx context.Context, filter ListFilter) (int64, error) {
	q := `
SELECT
    count(*)
FROM
    flags
WHERE
    deleted_at IS NULL`
	args := []any{}
	if filter.Type != nil {
		args = append(args, *filter.Type)
		q += ` AND type = $` + strconv.Itoa(len(args))
	}
	if filter.CreatedBy != nil {
		args = append(args, *filter.CreatedBy)
		q += ` AND created_by = $` + strconv.Itoa(len(args))
	}
	if filter.Search != nil {
		args = append(args, "%"+*filter.Search+"%")
		q += ` AND (key ILIKE $` + strconv.Itoa(len(args)) + ` OR name ILIKE $` + strconv.Itoa(len(args)) + `)`
	}

	var n int64
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}
