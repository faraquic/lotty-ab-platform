package attribution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"

	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type ExposureRegistry struct {
	store  PendingStore
	window time.Duration
	grace  time.Duration
	now    func() time.Time
	log    *zap.Logger
}

func NewExposureRegistry(
	store PendingStore,
	window time.Duration,
	grace time.Duration,
	now func() time.Time,
	log *zap.Logger,
) *ExposureRegistry {
	return &ExposureRegistry{
		store:  store,
		window: window,
		grace:  grace,
		now:    now,
		log:    log.Named("exposure_registry"),
	}
}

func (r *ExposureRegistry) Handle(ctx context.Context, msg kafka.Message) error {
	env, err := outbox.ParseEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: envelope: %v", consumer.ErrNonRetryable, err)
	}

	if env.Type != events.ExposureEnvelopeType {
		return fmt.Errorf("%w: unexpected record type %q", consumer.ErrNonRetryable, env.Type)
	}

	var p events.KafkaExposurePayload
	if err := json.Unmarshal(env.Data, &p); err != nil {
		return fmt.Errorf("%w: payload: %v", consumer.ErrNonRetryable, err)
	}

	if p.DecisionID == "" {
		return fmt.Errorf("%w: decision_id is required", consumer.ErrNonRetryable)
	}

	ttl := r.window + r.grace
	ttlAt := r.now().Add(ttl)

	err = r.store.RegisterExposure(ctx, p.DecisionID, PendingExposure{
		ExperimentID:        p.ExperimentID,
		ExperimentVersionID: p.ExperimentVersionID,
		VariantID:           p.VariantID,
		FlagID:              p.FlagID,
		SubjectID:           p.SubjectID,
		SaltVersion:         p.SaltVersion,
		OccurredAt:          p.OccurredAt,
		TTLAt:               ttlAt,
	}, ttl)
	if err != nil {
		return fmt.Errorf("register pending exposure: %w", err)
	}

	return nil
}

func (r *ExposureRegistry) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, consumer.ErrNonRetryable) {
		return false
	}

	return true
}
