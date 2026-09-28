package reports

import (
	"time"
)

type ReportQuery struct {
	Start    time.Time
	End      time.Time
	Interval string
	Metrics  []string
	Format   string
}

type Report struct {
	ExperimentID   string
	ExperimentName string
	Start          time.Time
	End            time.Time
	Interval       string
	Metrics        []MetricReport
	SampleSize     []VariantSample
	AllocationSkew *AllocationSkew
}

type MetricReport struct {
	Key        string
	Type       string
	TimeSeries []TimeSeriesPoint
	ByVariant  []VariantMetric
}

type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}

type VariantMetric struct {
	VariantID   string
	VariantName string
	Value       float64
}

type VariantSample struct {
	VariantID   string
	VariantName string
	Count       int64
}

type AllocationSkew struct {
	Expected float64
	Actual   float64
	Skew     float64
}

type DataQuality struct {
	RejectedRate       float64
	DuplicateRate      float64
	PendingAttribution int64
	AttributionLagSec  float64
}
