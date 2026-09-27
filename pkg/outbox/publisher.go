package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	defaultBatchSize     = 100
	defaultPollInterval  = 500 * time.Millisecond
	defaultMaxAttempts   = 10
	defaultBaseDelay     = time.Second
	defaultMaxDelay      = 5 * time.Minute
	defaultPublishTimout = 10 * time.Second
)

// Publisher relays outbox rows to Kafka. Delivery is at-least-once: rows are
// marked published only after the broker acknowledges them, and the envelope
// carries the outbox row id so consumers can deduplicate.
type Publisher struct {
	db                     *pgxpool.Pool
	brokers                []string
	writer                 *kafka.Writer
	log                    *zap.Logger
	batchSize              int
	pollInterval           time.Duration
	maxAttempts            int
	baseRetryDelay         time.Duration
	maxRetryDelay          time.Duration
	publishTimeout         time.Duration
	topicPartitions        int
	topicReplicationFactor int
}

// PublisherOption customizes a Publisher.
type PublisherOption func(*Publisher)

// WithBatchSize sets how many rows a single PublishOnce relays at most.
func WithBatchSize(n int) PublisherOption {
	return func(p *Publisher) {
		if n > 0 {
			p.batchSize = n
		}
	}
}

// WithPollInterval sets how often Run polls for pending rows.
func WithPollInterval(d time.Duration) PublisherOption {
	return func(p *Publisher) {
		if d > 0 {
			p.pollInterval = d
		}
	}
}

// WithMaxAttempts sets how many publish attempts a row gets before it is
// marked dead.
func WithMaxAttempts(n int) PublisherOption {
	return func(p *Publisher) {
		if n > 0 {
			p.maxAttempts = n
		}
	}
}

// WithRetryDelays sets the exponential backoff bounds between attempts.
func WithRetryDelays(base, max time.Duration) PublisherOption {
	return func(p *Publisher) {
		if base > 0 {
			p.baseRetryDelay = base
		}
		if max > 0 {
			p.maxRetryDelay = max
		}
	}
}

// WithPublishTimeout sets the per-message publish deadline.
func WithPublishTimeout(d time.Duration) PublisherOption {
	return func(p *Publisher) {
		if d > 0 {
			p.publishTimeout = d
		}
	}
}

// WithTopicConfig sets the partitions and replication factor used when the
// publisher auto-creates a missing topic. Defaults suit a single-broker
// dev cluster; production topics should be pre-created with proper settings.
func WithTopicConfig(partitions, replicationFactor int) PublisherOption {
	return func(p *Publisher) {
		if partitions > 0 {
			p.topicPartitions = partitions
		}
		if replicationFactor > 0 {
			p.topicReplicationFactor = replicationFactor
		}
	}
}

// NewPublisher returns a Publisher relaying rows from db through writer.
// writer must accept per-message topics (see database.NewBroadcastWriter).
// brokers are used for topic auto-creation.
func NewPublisher(db *pgxpool.Pool, brokers []string, writer *kafka.Writer, log *zap.Logger, opts ...PublisherOption) *Publisher {
	p := &Publisher{
		db:                     db,
		brokers:                brokers,
		writer:                 writer,
		log:                    log,
		batchSize:              defaultBatchSize,
		pollInterval:           defaultPollInterval,
		maxAttempts:            defaultMaxAttempts,
		baseRetryDelay:         defaultBaseDelay,
		maxRetryDelay:          defaultMaxDelay,
		publishTimeout:         defaultPublishTimout,
		topicPartitions:        1,
		topicReplicationFactor: 1,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Run polls and relays until ctx is cancelled. It returns nil on graceful
// shutdown; the row in flight, if any, is finished or rolled back.
func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := p.PublishOnce(ctx); err != nil {
				p.log.Error("outbox publish batch failed", zap.Error(err))
			}
		}
	}
}

type pendingRow struct {
	id       string
	topic    string
	key      *string
	payload  []byte
	headers  map[string]string
	attempts int
}

// PublishOnce relays up to batchSize pending rows and returns how many were
// published. Each row is claimed and marked in its own transaction, so a
// failed row never blocks the rows behind it.
func (p *Publisher) PublishOnce(ctx context.Context) (int, error) {
	published := 0

	for i := 0; i < p.batchSize; i++ {
		if err := ctx.Err(); err != nil {
			return published, err
		}

		done, ok, err := p.processOne(ctx)
		if err != nil {
			return published, err
		}

		if done {
			return published, nil
		}

		if ok {
			published++
		}
	}

	return published, nil
}

// processOne claims the oldest due row and relays it. done reports that no
// row is due; ok reports that a row was published. A failed publish is
// recorded in the row (retry or dead) and reported as (!done, false, nil).
func (p *Publisher) processOne(ctx context.Context) (done bool, ok bool, err error) {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return false, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var row pendingRow

	var rawHeaders []byte

	err = tx.QueryRow(ctx, `
SELECT id::text, topic, key, payload, headers, attempts
FROM outbox
WHERE status = 'pending' AND next_attempt_at <= now()
ORDER BY created_at
LIMIT 1
FOR UPDATE SKIP LOCKED`).Scan(&row.id, &row.topic, &row.key, &row.payload, &rawHeaders, &row.attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, false, nil
	}

	if err != nil {
		return false, false, err
	}

	if len(rawHeaders) > 0 {
		if err := json.Unmarshal(rawHeaders, &row.headers); err != nil {
			return false, false, err
		}
	}

	if err := p.publishMessage(ctx, row); err != nil {
		publishErr := err
		if database.IsUnknownTopic(err) {
			publishErr = p.publishWithEnsure(ctx, row)
		}

		if publishErr != nil {
			if markErr := p.markAttempt(ctx, tx, row, publishErr); markErr != nil {
				return false, false, markErr
			}

			if err := tx.Commit(ctx); err != nil {
				return false, false, err
			}

			return false, false, nil
		}
	}

	const markQ = `UPDATE outbox SET status = 'published', published_at = now() WHERE id = $1`

	if _, err := tx.Exec(ctx, markQ, row.id); err != nil {
		return false, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, false, err
	}

	p.log.Info(
		"outbox message published",
		zap.String(logger.FieldOutboxID, row.id),
		zap.String(logger.FieldKafkaTopic, row.topic),
	)

	return false, true, nil
}

func (p *Publisher) publishMessage(ctx context.Context, row pendingRow) error {
	publishCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	msg := kafka.Message{
		Topic: row.topic,
		Value: row.payload,
	}

	if row.key != nil {
		msg.Key = []byte(*row.key)
	}

	for k, v := range row.headers {
		msg.Headers = append(msg.Headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	return p.writer.WriteMessages(publishCtx, msg)
}

// markAttempt records a failed publish: the row is rescheduled with
// exponential backoff, or marked dead once it exhausts maxAttempts.
func (p *Publisher) markAttempt(ctx context.Context, tx pgx.Tx, row pendingRow, publishErr error) error {
	attempts := row.attempts + 1

	if attempts >= p.maxAttempts {
		const deadQ = `UPDATE outbox SET status = 'dead', attempts = $2, last_error = $3 WHERE id = $1`

		if _, err := tx.Exec(ctx, deadQ, row.id, attempts, publishErr.Error()); err != nil {
			return err
		}

		p.log.Error(
			"outbox message dead",
			zap.String(logger.FieldOutboxID, row.id),
			zap.String(logger.FieldKafkaTopic, row.topic),
			zap.Int(logger.FieldOutboxAttempts, attempts),
			zap.Error(publishErr),
		)

		return nil
	}

	const retryQ = `UPDATE outbox SET attempts = $2, last_error = $3, next_attempt_at = now() + $4::interval WHERE id = $1`

	delay := p.retryDelay(attempts)

	if _, err := tx.Exec(ctx, retryQ, row.id, attempts, publishErr.Error(), delay.String()); err != nil {
		return err
	}

	p.log.Warn(
		"outbox publish attempt failed",
		zap.String(logger.FieldOutboxID, row.id),
		zap.String(logger.FieldKafkaTopic, row.topic),
		zap.Int(logger.FieldOutboxAttempts, attempts),
		zap.Error(publishErr),
	)

	return nil
}

// retryDelay returns the backoff before the given attempt count,
// exponentially growing from base and capped at max.
func (p *Publisher) retryDelay(attempts int) time.Duration {
	base := p.baseRetryDelay
	max := p.maxRetryDelay

	if base <= 0 {
		base = defaultBaseDelay
	}

	if max <= 0 {
		max = defaultMaxDelay
	}

	if attempts < 0 {
		attempts = 0
	}

	delay := base

	for i := 0; i < attempts; i++ {
		delay *= 2

		if delay <= 0 || delay >= max {
			return max
		}
	}

	if delay > max {
		return max
	}

	return delay
}

// publishWithEnsure publishes row, creating its topic first when the broker
// reports it as unknown. kafka-go metadata requests do not trigger
// server-side auto-creation, so the relay creates missing topics itself.
// Only one creation attempt is made; a repeated failure is returned for the
// regular backoff path.
func (p *Publisher) publishWithEnsure(ctx context.Context, row pendingRow) error {
	ensureCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	if err := database.EnsureTopic(ensureCtx, p.brokers, row.topic, p.topicPartitions, p.topicReplicationFactor); err != nil {
		p.log.Warn(
			"outbox topic ensure failed",
			zap.String(logger.FieldKafkaTopic, row.topic),
			zap.Error(err),
		)

		return err
	}

	p.log.Info(
		"outbox topic ensured",
		zap.String(logger.FieldKafkaTopic, row.topic),
	)

	return p.publishMessage(ctx, row)
}
