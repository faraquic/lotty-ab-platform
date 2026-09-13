package metrics

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/goccy/go-json"
)

var (
	ErrInvalidMetricConfig = errors.New("invalid metric aggregation or attribution")
	ErrBuiltinProtected    = errors.New("built-in metric fields cannot be modified")
	ErrBuiltinDelete       = errors.New("built-in metrics cannot be deleted")
)

const (
	maxEventTypeLen = 128
	maxFieldLen     = 128
	minWindowDays   = 1
	maxWindowDays   = 30
)

const (
	AttributionFallbackNone    = "none"
	AttributionFallbackSubject = "subject"
)

type EventRef struct {
	EventType string  `json:"event_type"`
	Field     *string `json:"field,omitempty"`
}

func (r EventRef) Validate() error {
	if strings.TrimSpace(r.EventType) == "" || len(r.EventType) > maxEventTypeLen {
		return fmt.Errorf("%w: event_type must be 1..%d chars", ErrInvalidMetricConfig, maxEventTypeLen)
	}
	if r.Field != nil && (strings.TrimSpace(*r.Field) == "" || len(*r.Field) > maxFieldLen) {
		return fmt.Errorf("%w: field must be 1..%d chars", ErrInvalidMetricConfig, maxFieldLen)
	}
	return nil
}

type Aggregation struct {
	EventType   *string   `json:"event_type,omitempty"`
	Field       *string   `json:"field,omitempty"`
	Level       *float64  `json:"level,omitempty"`
	Numerator   *EventRef `json:"numerator,omitempty"`
	Denominator *EventRef `json:"denominator,omitempty"`
}

func (a Aggregation) Validate(t MetricType) error {
	switch t {
	case MetricTypeCount, MetricTypeUniqueCount:
		if a.EventType == nil || strings.TrimSpace(*a.EventType) == "" || len(*a.EventType) > maxEventTypeLen {
			return fmt.Errorf("%w: aggregation.event_type must be 1..%d chars for %s", ErrInvalidMetricConfig, maxEventTypeLen, t)
		}
		if a.Field != nil || a.Level != nil || a.Numerator != nil || a.Denominator != nil {
			return fmt.Errorf("%w: only aggregation.event_type is allowed for %s", ErrInvalidMetricConfig, t)
		}
		return nil
	case MetricTypeSum, MetricTypeAverage:
		if a.EventType == nil || strings.TrimSpace(*a.EventType) == "" || len(*a.EventType) > maxEventTypeLen {
			return fmt.Errorf("%w: aggregation.event_type must be 1..%d chars for %s", ErrInvalidMetricConfig, maxEventTypeLen, t)
		}
		if a.Field == nil || strings.TrimSpace(*a.Field) == "" || len(*a.Field) > maxFieldLen {
			return fmt.Errorf("%w: aggregation.field must be 1..%d chars for %s", ErrInvalidMetricConfig, maxFieldLen, t)
		}
		if a.Level != nil || a.Numerator != nil || a.Denominator != nil {
			return fmt.Errorf("%w: only aggregation.event_type and aggregation.field are allowed for %s", ErrInvalidMetricConfig, t)
		}
		return nil
	case MetricTypePercentile:
		if a.EventType == nil || strings.TrimSpace(*a.EventType) == "" || len(*a.EventType) > maxEventTypeLen {
			return fmt.Errorf("%w: aggregation.event_type must be 1..%d chars for percentile", ErrInvalidMetricConfig, maxEventTypeLen)
		}
		if a.Field == nil || strings.TrimSpace(*a.Field) == "" || len(*a.Field) > maxFieldLen {
			return fmt.Errorf("%w: aggregation.field must be 1..%d chars for percentile", ErrInvalidMetricConfig, maxFieldLen)
		}
		if a.Level == nil || *a.Level <= 0 || *a.Level >= 1 {
			return fmt.Errorf("%w: aggregation.level must be in (0, 1) for percentile", ErrInvalidMetricConfig)
		}
		if a.Numerator != nil || a.Denominator != nil {
			return fmt.Errorf("%w: numerator/denominator are not allowed for percentile", ErrInvalidMetricConfig)
		}
		return nil
	case MetricTypeRatio:
		if a.EventType != nil || a.Field != nil || a.Level != nil {
			return fmt.Errorf("%w: only numerator/denominator are allowed for ratio", ErrInvalidMetricConfig)
		}
		if a.Numerator == nil || a.Denominator == nil {
			return fmt.Errorf("%w: aggregation.numerator and aggregation.denominator are required for ratio", ErrInvalidMetricConfig)
		}
		if err := a.Numerator.Validate(); err != nil {
			return fmt.Errorf("aggregation.numerator: %w", err)
		}
		if err := a.Denominator.Validate(); err != nil {
			return fmt.Errorf("aggregation.denominator: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown metric type", ErrInvalidMetricConfig)
	}
}

func (a Aggregation) Value() (driver.Value, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func (a *Aggregation) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return ErrInvalidMetricConfig
	}
	if err := json.Unmarshal(data, a); err != nil {
		return err
	}
	return nil
}

type Attribution struct {
	RequireExposure bool   `json:"require_exposure"`
	WindowDays      int    `json:"window_days"`
	Fallback        string `json:"fallback"`
}

func (a Attribution) Validate() error {
	if a.WindowDays < minWindowDays || a.WindowDays > maxWindowDays {
		return fmt.Errorf("%w: attribution.window_days must be %d..%d", ErrInvalidMetricConfig, minWindowDays, maxWindowDays)
	}
	if a.Fallback != AttributionFallbackNone && a.Fallback != AttributionFallbackSubject {
		return fmt.Errorf("%w: attribution.fallback must be one of (none, subject)", ErrInvalidMetricConfig)
	}
	return nil
}

func (a Attribution) Value() (driver.Value, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func (a *Attribution) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return ErrInvalidMetricConfig
	}
	if err := json.Unmarshal(data, a); err != nil {
		return err
	}
	return nil
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
	Aggregation Aggregation
	Attribution Attribution
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
