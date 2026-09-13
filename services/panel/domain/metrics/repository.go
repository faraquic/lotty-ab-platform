package metrics

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("metric not found")
	ErrConflictKeys  = errors.New("metric with this key already exists")
	ErrConflictNames = errors.New("metric with this name already exists")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db}
}

func (r *Repository) Create(ctx context.Context, m Metric) (string, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	id := uid.String()
	const q = `
INSERT INTO metrics(id, key, name, description, metric_type, aggregation, attribution, is_builtin, status, created_by, updated_by)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING
    id`

	var outID string
	err = r.db.QueryRow(
		ctx, q,
		id, m.Key, m.Name, m.Description, string(m.MetricType),
		m.Aggregation, m.Attribution, m.IsBuiltin, string(m.Status),
		m.CreatedBy, m.UpdatedBy,
	).Scan(&outID)
	if err != nil {
		if database.IsUniqueViolation(err) {
			if strings.Contains(err.Error(), "metrics_key_key") {
				return "", ErrConflictKeys
			}
			if strings.Contains(err.Error(), "metrics_name_key") {
				return "", ErrConflictNames
			}
			return "", ErrConflictKeys
		}
		return "", err
	}
	return outID, nil
}

const selectMetricWithCreatorAndUpdater = `
SELECT
    m.id,
    m.key,
    m.name,
    m.description,
    m.metric_type,
    m.aggregation,
    m.attribution,
    m.is_builtin,
    m.status,
    m.created_by,
    m.updated_by,
    m.created_at,
    m.updated_at,
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
    metrics m
    JOIN users cb ON m.created_by = cb.id
    JOIN users ub ON m.updated_by = ub.id`

type metricWithCreatorAndUpdaterRow struct {
	MetricID        string
	Key             string
	Name            string
	Description     *string
	MetricType      string
	Aggregation     Aggregation
	Attribution     Attribution
	IsBuiltin       bool
	Status          string
	CreatedByID     string
	UpdatedByID     string
	MetricCreated   time.Time
	MetricUpdated   time.Time
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

func (r metricWithCreatorAndUpdaterRow) toMetricWithCreatorAndUpdater() MetricWithCreatorAndUpdater {
	return MetricWithCreatorAndUpdater{
		Metric: Metric{
			ID:          r.MetricID,
			Key:         r.Key,
			Name:        r.Name,
			Description: r.Description,
			MetricType:  MetricType(r.MetricType),
			Aggregation: r.Aggregation,
			Attribution: r.Attribution,
			IsBuiltin:   r.IsBuiltin,
			Status:      MetricStatus(r.Status),
			CreatedBy:   r.CreatedByID,
			UpdatedBy:   r.UpdatedByID,
			CreatedAt:   r.MetricCreated,
			UpdatedAt:   r.MetricUpdated,
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

func (r *Repository) GetByID(ctx context.Context, id string, includeArchived bool) (MetricWithCreatorAndUpdater, error) {
	q := selectMetricWithCreatorAndUpdater + `
WHERE
    m.id = $1`
	if !includeArchived {
		q += `
    AND m.status = 'active'`
	}

	var row metricWithCreatorAndUpdaterRow
	err := r.db.QueryRow(ctx, q, id).Scan(
		&row.MetricID,
		&row.Key,
		&row.Name,
		&row.Description,
		&row.MetricType,
		&row.Aggregation,
		&row.Attribution,
		&row.IsBuiltin,
		&row.Status,
		&row.CreatedByID,
		&row.UpdatedByID,
		&row.MetricCreated,
		&row.MetricUpdated,
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
			return MetricWithCreatorAndUpdater{}, ErrNotFound
		}
		return MetricWithCreatorAndUpdater{}, err
	}

	return row.toMetricWithCreatorAndUpdater(), nil
}

func (r *Repository) List(ctx context.Context, limit, offset int, includeArchived bool) ([]Metric, error) {
	q := `
SELECT
    m.id,
    m.key,
    m.name,
    m.description,
    m.metric_type,
    m.aggregation,
    m.attribution,
    m.is_builtin,
    m.status,
    m.created_by,
    m.updated_by,
    m.created_at,
    m.updated_at
FROM
    metrics m`
	if !includeArchived {
		q += `
WHERE
    m.status = 'active'`
	}
	q += `
ORDER BY
    m.id
LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(
			&m.ID,
			&m.Key,
			&m.Name,
			&m.Description,
			&m.MetricType,
			&m.Aggregation,
			&m.Attribution,
			&m.IsBuiltin,
			&m.Status,
			&m.CreatedBy,
			&m.UpdatedBy,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *Repository) Update(ctx context.Context, id string, key, name, description string, aggregation *Aggregation, attribution *Attribution, status MetricStatus, updatedBy string) (MetricWithCreatorAndUpdater, error) {
	var agg any
	if aggregation != nil {
		agg = *aggregation
	}
	var attr any
	if attribution != nil {
		attr = *attribution
	}
	var stat any
	if status != "" {
		stat = string(status)
	}

	const q = `
UPDATE
    metrics
SET
    key = COALESCE(NULLIF($2, ''), key),
    name = COALESCE(NULLIF($3, ''), name),
    description = COALESCE(NULLIF($4, ''), description),
    aggregation = COALESCE($5, aggregation),
    attribution = COALESCE($6, attribution),
    status = COALESCE($7, status),
    updated_by = $8
WHERE
    id = $1`

	tag, err := r.db.Exec(ctx, q, id, key, name, description, agg, attr, stat, updatedBy)
	if err != nil {
		if database.IsUniqueViolation(err) {
			if strings.Contains(err.Error(), "metrics_key_key") {
				return MetricWithCreatorAndUpdater{}, ErrConflictKeys
			}
			if strings.Contains(err.Error(), "metrics_name_key") {
				return MetricWithCreatorAndUpdater{}, ErrConflictNames
			}
			return MetricWithCreatorAndUpdater{}, ErrConflictKeys
		}
		return MetricWithCreatorAndUpdater{}, err
	}
	if tag.RowsAffected() == 0 {
		return MetricWithCreatorAndUpdater{}, ErrNotFound
	}
	return r.GetByID(ctx, id, true)
}

func (r *Repository) Count(ctx context.Context, includeArchived bool) (int64, error) {
	q := `
SELECT
    count(*)
FROM
    metrics`
	if !includeArchived {
		q += `
WHERE
    status = 'active'`
	}

	var n int64
	err := r.db.QueryRow(ctx, q).Scan(&n)
	return n, err
}
