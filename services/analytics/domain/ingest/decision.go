package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	pkgclickhouse "github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	decidedomain "github.com/faraquic/lotty-ab-platform/services/runtime/domain/decide"
	"github.com/segmentio/kafka-go"
)

const decisionValidationSkew = 24 * time.Hour

type DecisionValidator struct {
	now func() time.Time
}

func NewDecisionValidator() *DecisionValidator {
	return &DecisionValidator{now: time.Now}
}

func (v *DecisionValidator) Handle(_ context.Context, msg kafka.Message) error {
	env, err := outbox.ParseEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: envelope: %v", consumer.ErrNonRetryable, err)
	}

	if env.Type != decidedomain.DecisionEnvelopeType {
		return fmt.Errorf("%w: unexpected record type %q", consumer.ErrNonRetryable, env.Type)
	}

	var rec decidedomain.DecisionRecord
	if err := json.Unmarshal(env.Data, &rec); err != nil {
		return fmt.Errorf("%w: payload: %v", consumer.ErrNonRetryable, err)
	}

	if _, err := uuid.Parse(rec.DecisionID); err != nil {
		return fmt.Errorf("%w: decision_id: %v", consumer.ErrNonRetryable, err)
	}

	if rec.SubjectID == "" {
		return fmt.Errorf("%w: subject_id is required", consumer.ErrNonRetryable)
	}

	if rec.FlagKey == "" {
		return fmt.Errorf("%w: flag_key is required", consumer.ErrNonRetryable)
	}

	if rec.ResultSource == "" {
		return fmt.Errorf("%w: result_source is required", consumer.ErrNonRetryable)
	}

	now := v.now()
	if rec.CreatedAt.Before(now.Add(-decisionValidationSkew)) || rec.CreatedAt.After(now.Add(decisionValidationSkew)) {
		return fmt.Errorf("%w: created_at %s outside ±%s window", consumer.ErrNonRetryable, rec.CreatedAt, decisionValidationSkew)
	}

	return nil
}

func (v *DecisionValidator) IsRetryable(error) bool {
	return false
}

func DecisionRowFromEnvelope(env outbox.Envelope) (pkgclickhouse.DecisionRow, error) {
	var rec decidedomain.DecisionRecord
	if err := json.Unmarshal(env.Data, &rec); err != nil {
		return pkgclickhouse.DecisionRow{}, fmt.Errorf("unmarshal decision record: %w", err)
	}

	decisionID, err := uuid.Parse(rec.DecisionID)
	if err != nil {
		return pkgclickhouse.DecisionRow{}, fmt.Errorf("parse decision_id: %w", err)
	}

	requestID, err := uuid.Parse(rec.RequestID)
	if err != nil {
		return pkgclickhouse.DecisionRow{}, fmt.Errorf("parse request_id: %w", err)
	}

	row := pkgclickhouse.DecisionRow{
		DecisionID:   decisionID,
		RequestID:    requestID,
		SubjectID:    rec.SubjectID,
		FlagKey:      rec.FlagKey,
		ResultSource: rec.ResultSource,
		CreatedAt:    rec.CreatedAt.UTC(),
	}

	if rec.ExperimentID != "" {
		row.ExperimentID, _ = uuid.Parse(rec.ExperimentID)
	}

	if rec.ExperimentVersionID != "" {
		row.ExperimentVersionID, _ = uuid.Parse(rec.ExperimentVersionID)
	}

	if rec.VariantID != "" {
		row.VariantID, _ = uuid.Parse(rec.VariantID)
	}

	if rec.ConfigRevision != "" {
		var rev uint64
		if _, err := fmt.Sscanf(rec.ConfigRevision, "%d", &rev); err == nil {
			row.ConfigRevision = rev
		}
	}

	return row, nil
}
