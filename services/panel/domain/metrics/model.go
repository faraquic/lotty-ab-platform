package metrics

import (
	"database/sql/driver"
	"errors"
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/goccy/go-json"
)

var (
	ErrInvalidMetricConfig = errors.New("aggregation and attribution must be valid non-empty JSON objects")
	ErrBuiltinProtected    = errors.New("built-in metric fields cannot be modified")
	ErrBuiltinDelete       = errors.New("built-in metrics cannot be deleted")
)

type MetricConfig []byte

func (c MetricConfig) Valid() bool {
	if len(c) == 0 || !json.Valid(c) {
		return false
	}

	var m map[string]any
	if err := json.Unmarshal(c, &m); err != nil {
		return false
	}

	return len(m) > 0
}

func (c MetricConfig) MarshalJSON() ([]byte, error) {
	return c, nil
}

func (c *MetricConfig) UnmarshalJSON(data []byte) error {
	out := make([]byte, len(data))
	copy(out, data)
	*c = out
	return nil
}

func (c *MetricConfig) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		return c.UnmarshalJSON(v)
	case string:
		return c.UnmarshalJSON([]byte(v))
	}
	return ErrInvalidMetricConfig
}

func (c MetricConfig) Value() (driver.Value, error) {
	return string(c), nil
}

type MetricType string

const (
	MetricTypeCount       MetricType = "count"
	MetricTypeSum         MetricType = "sum"
	MetricTypeUniqueCount MetricType = "unique_count"
	MetricTypeRatio       MetricType = "ratio"
	MetricTypePercentile  MetricType = "percentile"
	MetricTypeAverage     MetricType = "average"
)

func (t MetricType) Valid() bool {
	switch t {
	case MetricTypeCount, MetricTypeSum, MetricTypeUniqueCount,
		MetricTypeRatio, MetricTypePercentile, MetricTypeAverage:
		return true
	default:
		return false
	}
}

type MetricStatus string

const (
	MetricStatusActive   MetricStatus = "active"
	MetricStatusArchived MetricStatus = "archived"
)

func (s MetricStatus) Valid() bool {
	switch s {
	case MetricStatusActive, MetricStatusArchived:
		return true
	default:
		return false
	}
}

type Metric struct {
	ID          string
	Key         string
	Name        string
	Description *string
	MetricType  MetricType
	Aggregation MetricConfig
	Attribution MetricConfig
	IsBuiltin   bool
	Status      MetricStatus
	CreatedBy   string
	UpdatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MetricWithCreatorAndUpdater struct {
	Metric    Metric
	CreatedBy *users.User
	UpdatedBy *users.User
}
