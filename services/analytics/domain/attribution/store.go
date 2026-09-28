package attribution

import (
	"context"
	"time"
)

const (
	pendingExposureKeyPrefix   = "attribution:pending:exposure:"
	pendingConversionKeyPrefix = "attribution:pending:conversion:"
	pendingConversionsZSet     = "attribution:pending:conversions"
)

type PendingExposure struct {
	ExperimentID        string    `json:"experiment_id"`
	ExperimentVersionID string    `json:"experiment_version_id"`
	VariantID           string    `json:"variant_id"`
	FlagID              string    `json:"flag_id"`
	SubjectID           string    `json:"subject_id"`
	SaltVersion         string    `json:"salt_version"`
	OccurredAt          time.Time `json:"occurred_at"`
	TTLAt               time.Time `json:"ttl_at"`
}

type PendingConversion struct {
	EventID     string    `json:"event_id"`
	EventType   string    `json:"event_type"`
	SubjectID   string    `json:"subject_id"`
	SaltVersion string    `json:"salt_version"`
	OccurredAt  time.Time `json:"occurred_at"`
	TTLAt       time.Time `json:"ttl_at"`
}

type PendingStore interface {
	RegisterExposure(ctx context.Context, decisionID string, pe PendingExposure, ttl time.Duration) error
	GetExposure(ctx context.Context, decisionID string) (*PendingExposure, error)
	DeleteExposure(ctx context.Context, decisionID string) error
	RegisterConversion(ctx context.Context, decisionID string, pc PendingConversion, ttl time.Duration) error
	GetConversion(ctx context.Context, decisionID string) (*PendingConversion, error)
	DeleteConversion(ctx context.Context, decisionID string) error
	ExpiredConversions(ctx context.Context, now time.Time) ([]string, error)
}
