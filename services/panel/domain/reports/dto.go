package reports

import (
	"time"
)

type ReportRequest struct {
	Start    time.Time `form:"start" binding:"required"`
	End      time.Time `form:"end" binding:"required"`
	Interval string    `form:"interval" binding:"required,oneof=hour day week month"`
	Metrics  []string  `form:"metrics" binding:"required,min=1,max=20,dive,min=1,max=128"`
	Format   string    `form:"format" binding:"omitempty,oneof=json csv"`
}

type ReportResponse struct {
	ExperimentID   string          `json:"experiment_id"`
	ExperimentName string          `json:"experiment_name"`
	Start          time.Time       `json:"start"`
	End            time.Time       `json:"end"`
	Interval       string          `json:"interval"`
	Metrics        []MetricReport  `json:"metrics"`
	SampleSize     []VariantSample `json:"sample_size"`
	AllocationSkew *AllocationSkew `json:"allocation_skew,omitempty"`
}

type DataQualityResponse struct {
	RejectedRate       float64 `json:"rejected_rate"`
	DuplicateRate      float64 `json:"duplicate_rate"`
	PendingAttribution int64   `json:"pending_attribution"`
	AttributionLagSec  float64 `json:"attribution_lag_seconds"`
}
