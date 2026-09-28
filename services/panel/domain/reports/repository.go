package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

type Repository struct {
	conn clickhouse.Conn
}

func NewRepository(conn clickhouse.Conn) *Repository {
	return &Repository{conn: conn}
}

func (r *Repository) GetTimeSeries(ctx context.Context, experimentID uuid.UUID, variantID uuid.UUID, eventType string, start, end time.Time, interval string) ([]TimeSeriesPoint, error) {
	intervalExpr := intervalExpr(interval)

	q := fmt.Sprintf(`
SELECT
    toStartOfInterval(occurred_at, %s) AS ts,
    count() AS value
FROM labp.attributed_events FINAL
WHERE experiment_id = {experimentID:UUID}
  AND variant_id = {variantID:UUID}
  AND event_type = {eventType:String}
  AND occurred_at BETWEEN {start:DateTime64(3)} AND {end:DateTime64(3)}
GROUP BY ts
ORDER BY ts`, intervalExpr)

	rows, err := r.conn.Query(ctx, q,
		clickhouse.Named("experimentID", experimentID),
		clickhouse.Named("variantID", variantID),
		clickhouse.Named("eventType", eventType),
		clickhouse.Named("start", start),
		clickhouse.Named("end", end),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (r *Repository) GetVariantMetrics(ctx context.Context, experimentID uuid.UUID, eventType string, start, end time.Time) ([]VariantMetric, error) {
	q := `
SELECT
    variant_id,
    count() AS value
FROM labp.attributed_events FINAL
WHERE experiment_id = {experimentID:UUID}
  AND event_type = {eventType:String}
  AND occurred_at BETWEEN {start:DateTime64(3)} AND {end:DateTime64(3)}
GROUP BY variant_id
ORDER BY variant_id`

	rows, err := r.conn.Query(ctx, q,
		clickhouse.Named("experimentID", experimentID),
		clickhouse.Named("eventType", eventType),
		clickhouse.Named("start", start),
		clickhouse.Named("end", end),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []VariantMetric
	for rows.Next() {
		var m VariantMetric
		if err := rows.Scan(&m.VariantID, &m.Value); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

func (r *Repository) GetSampleSize(ctx context.Context, experimentID uuid.UUID, start, end time.Time) ([]VariantSample, error) {
	q := `
SELECT
    variant_id,
    uniqExact(subject_id) AS sample_size
FROM labp.exposures FINAL
WHERE experiment_id = {experimentID:UUID}
  AND occurred_at BETWEEN {start:DateTime64(3)} AND {end:DateTime64(3)}
GROUP BY variant_id
ORDER BY variant_id`

	rows, err := r.conn.Query(ctx, q,
		clickhouse.Named("experimentID", experimentID),
		clickhouse.Named("start", start),
		clickhouse.Named("end", end),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []VariantSample
	for rows.Next() {
		var s VariantSample
		var variantID string
		if err := rows.Scan(&variantID, &s.Count); err != nil {
			return nil, err
		}
		s.VariantID = variantID
		samples = append(samples, s)
	}
	return samples, rows.Err()
}

func (r *Repository) GetAttributionLag(ctx context.Context, experimentID uuid.UUID, start, end time.Time) (float64, error) {
	q := `
SELECT
    avg(dateDiff('second', occurred_at, received_at)) AS lag_sec
FROM labp.attributed_events FINAL
WHERE experiment_id = {experimentID:UUID}
  AND occurred_at BETWEEN {start:DateTime64(3)} AND {end:DateTime64(3)}`

	var lag float64
	err := r.conn.QueryRow(ctx, q,
		clickhouse.Named("experimentID", experimentID),
		clickhouse.Named("start", start),
		clickhouse.Named("end", end),
	).Scan(&lag)
	if err != nil {
		return 0, err
	}
	return lag, nil
}

func (r *Repository) GetEventCounts(ctx context.Context, experimentID uuid.UUID, start, end time.Time) (map[string]int64, error) {
	q := `
SELECT
    event_type,
    count() AS cnt
FROM labp.attributed_events FINAL
WHERE experiment_id = {experimentID:UUID}
  AND occurred_at BETWEEN {start:DateTime64(3)} AND {end:DateTime64(3)}
GROUP BY event_type`

	rows, err := r.conn.Query(ctx, q,
		clickhouse.Named("experimentID", experimentID),
		clickhouse.Named("start", start),
		clickhouse.Named("end", end),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var eventType string
		var cnt int64
		if err := rows.Scan(&eventType, &cnt); err != nil {
			return nil, err
		}
		counts[eventType] = cnt
	}
	return counts, rows.Err()
}

func intervalExpr(interval string) string {
	switch interval {
	case "hour":
		return "INTERVAL 1 HOUR"
	case "day":
		return "INTERVAL 1 DAY"
	case "week":
		return "INTERVAL 1 WEEK"
	case "month":
		return "INTERVAL 1 MONTH"
	default:
		return "INTERVAL 1 DAY"
	}
}
