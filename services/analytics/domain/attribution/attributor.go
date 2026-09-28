package attribution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Catalog interface {
	RequiresExposure(eventType string) bool
}

type Attributor struct {
	store   PendingStore
	catalog Catalog
	writer  AttrWriter
	window  time.Duration
	grace   time.Duration
	now     func() time.Time
	log     *zap.Logger
}

type AttrWriter interface {
	WriteAttributed(ctx context.Context, row clickhouse.AttributedEventRow)
}

type CHWriterAdapter struct {
	w *clickhouse.Writer
}

func NewCHWriterAdapter(w *clickhouse.Writer) CHWriterAdapter {
	return CHWriterAdapter{w: w}
}

func (a CHWriterAdapter) WriteAttributed(ctx context.Context, row clickhouse.AttributedEventRow) {
	a.w.AppendAttributedEvent(row)
}

func NewAttributor(
	store PendingStore,
	catalog Catalog,
	writer AttrWriter,
	window time.Duration,
	grace time.Duration,
	now func() time.Time,
	log *zap.Logger,
) *Attributor {
	return &Attributor{
		store:   store,
		catalog: catalog,
		writer:  writer,
		window:  window,
		grace:   grace,
		now:     now,
		log:     log.Named("attributor"),
	}
}

func (a *Attributor) Handle(ctx context.Context, msg kafka.Message) error {
	env, err := outbox.ParseEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: envelope: %v", consumer.ErrNonRetryable, err)
	}

	if env.Type != events.EventEnvelopeType {
		return fmt.Errorf("%w: unexpected record type %q", consumer.ErrNonRetryable, env.Type)
	}

	var p events.KafkaEventPayload
	if err := json.Unmarshal(env.Data, &p); err != nil {
		return fmt.Errorf("%w: payload: %v", consumer.ErrNonRetryable, err)
	}

	if p.DecisionID == "" {
		return nil
	}

	decisionID, err := uuid.Parse(p.DecisionID)
	if err != nil {
		return fmt.Errorf("%w: decision_id: %v", consumer.ErrNonRetryable, err)
	}

	eventID, err := uuid.Parse(p.EventID)
	if err != nil {
		return fmt.Errorf("%w: event_id: %v", consumer.ErrNonRetryable, err)
	}

	exposure, err := a.store.GetExposure(ctx, p.DecisionID)
	if err != nil {
		return fmt.Errorf("get pending exposure: %w", err)
	}

	if exposure != nil {
		a.writeAttribution(ctx, eventID, decisionID, exposure.ExperimentID, exposure.VariantID, p)
		if err := a.store.DeleteExposure(ctx, p.DecisionID); err != nil {
			a.log.Warn("failed to delete pending exposure",
				zap.String(logger.FieldDecisionID, p.DecisionID),
				zap.Error(err),
			)
		}

		return nil
	}

	if !a.catalog.RequiresExposure(p.EventType) {
		a.writeAttribution(ctx, eventID, decisionID, "", "", p)

		return nil
	}

	ttlAt := a.now().Add(a.window + a.grace)
	err = a.store.RegisterConversion(ctx, p.DecisionID, PendingConversion{
		EventID:     p.EventID,
		EventType:   p.EventType,
		SubjectID:   p.SubjectID,
		SaltVersion: p.SaltVersion,
		OccurredAt:  p.OccurredAt,
		TTLAt:       ttlAt,
	}, a.window+a.grace)
	if err != nil {
		return fmt.Errorf("register pending conversion: %w", err)
	}

	return nil
}

func (a *Attributor) writeAttribution(ctx context.Context, eventID, decision uuid.UUID, experimentID, variantID string, p events.KafkaEventPayload) {
	var expID, varID uuid.UUID

	if experimentID != "" {
		expID, _ = uuid.Parse(experimentID)
	}

	if variantID != "" {
		varID, _ = uuid.Parse(variantID)
	}

	a.writer.WriteAttributed(ctx, clickhouse.AttributedEventRow{
		EventID:      eventID,
		DecisionID:   decision,
		ExperimentID: expID,
		VariantID:    varID,
		SubjectID:    p.SubjectID,
		SaltVersion:  p.SaltVersion,
		EventType:    p.EventType,
		OccurredAt:   p.OccurredAt.UTC(),
		ReceivedAt:   a.now().UTC(),
	})
}

func (a *Attributor) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, consumer.ErrNonRetryable) {
		return false
	}

	return true
}
