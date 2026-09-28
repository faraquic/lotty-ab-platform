package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type dedupQuerier interface {
	Exists(ctx context.Context, id string) (bool, error)
	Insert(ctx context.Context, id string) (bool, error)
}

type pgDedupStore struct {
	db *pgxpool.Pool
}

func (p pgDedupStore) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool

	err := p.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kafka_dedup_keys WHERE event_id = $1)`, id).Scan(&exists)

	return exists, err
}

func (p pgDedupStore) Insert(ctx context.Context, id string) (bool, error) {
	tag, err := p.db.Exec(ctx, `INSERT INTO kafka_dedup_keys(event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, id)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() > 0, nil
}

type DedupStore struct {
	seen *outbox.Deduper
	q    dedupQuerier
	log  *zap.Logger
}

func NewDedupStore(db *pgxpool.Pool, log *zap.Logger) *DedupStore {
	return &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    pgDedupStore{db: db},
		log:  log,
	}
}

func (d *DedupStore) Handle(ctx context.Context, msg kafka.Message) error {
	env, err := outbox.ParseEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: envelope: %v", consumer.ErrNonRetryable, err)
	}

	if env.Type != events.EventEnvelopeType {
		return fmt.Errorf("%w: unexpected record type %q", consumer.ErrNonRetryable, env.Type)
	}

	if d.seen.SeenOrMark(env.ID) {
		return fmt.Errorf("%w: %s", consumer.ErrDrop, env.ID)
	}

	exists, err := d.q.Exists(ctx, env.ID)
	if err != nil {
		return fmt.Errorf("dedup lookup: %w", err)
	}

	if exists {
		return fmt.Errorf("%w: %s", consumer.ErrDrop, env.ID)
	}

	inserted, err := d.q.Insert(ctx, env.ID)
	if err != nil {
		return fmt.Errorf("dedup insert: %w", err)
	}

	if !inserted {
		return fmt.Errorf("%w: %s", consumer.ErrDrop, env.ID)
	}

	return nil
}

func (d *DedupStore) IsRetryable(err error) bool {
	if errors.Is(err, consumer.ErrDrop) || errors.Is(err, consumer.ErrNonRetryable) {
		return false
	}

	return true
}
