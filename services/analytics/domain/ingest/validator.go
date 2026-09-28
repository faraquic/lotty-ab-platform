package ingest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const validationSkew = 24 * time.Hour

type eventCatalogQuerier interface {
	ActiveEventTypes(ctx context.Context) ([]string, error)
}

type pgEventCatalog struct {
	db *pgxpool.Pool
}

func (p pgEventCatalog) ActiveEventTypes(ctx context.Context) ([]string, error) {
	rows, err := p.db.Query(ctx, `SELECT key FROM event_types WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}

		out = append(out, key)
	}

	return out, rows.Err()
}

type ValidationCatalog struct {
	querier         eventCatalogQuerier
	refreshInterval time.Duration

	mu   sync.RWMutex
	keys map[string]struct{}

	log *zap.Logger
}

func NewValidationCatalog(querier eventCatalogQuerier, refreshInterval time.Duration, log *zap.Logger) (*ValidationCatalog, error) {
	c := &ValidationCatalog{
		querier:         querier,
		refreshInterval: refreshInterval,
		keys:            map[string]struct{}{},
		log:             log,
	}

	if err := c.Reload(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ValidationCatalog) Reload(ctx context.Context) error {
	keys, err := c.querier.ActiveEventTypes(ctx)
	if err != nil {
		return err
	}

	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}

	c.mu.Lock()
	c.keys = set
	c.mu.Unlock()

	return nil
}

func (c *ValidationCatalog) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.keys[key]

	return ok
}

func (c *ValidationCatalog) Start(ctx context.Context) {
	if c.refreshInterval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(c.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.Reload(ctx); err != nil {
					c.log.Warn("event type catalog reload failed", zap.Error(err))
				}
			}
		}
	}()
}

type Validator struct {
	catalog *ValidationCatalog
	now     func() time.Time
}

func (v *Validator) Handle(_ context.Context, msg kafka.Message) error {
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

	if !v.catalog.Contains(p.EventType) {
		return fmt.Errorf("%w: unknown event type %q", consumer.ErrNonRetryable, p.EventType)
	}

	if !json.Valid(p.Payload) {
		return fmt.Errorf("%w: invalid payload json", consumer.ErrNonRetryable)
	}

	now := v.now()
	if p.OccurredAt.Before(now.Add(-validationSkew)) || p.OccurredAt.After(now.Add(validationSkew)) {
		return fmt.Errorf("%w: occurred_at %s outside ±%s window", consumer.ErrNonRetryable, p.OccurredAt, validationSkew)
	}

	return nil
}

func (v *Validator) IsRetryable(error) bool {
	return false
}
