package ingest

import (
	"context"
	"time"

	pkgclickhouse "github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Fetcher interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

type RowKind string

const (
	RowKindEvent    RowKind = "event"
	RowKindExposure RowKind = "exposure"
	RowKindDecision RowKind = "decision"
)

type TopicConsumer struct {
	fetcher       Fetcher
	writer        *pkgclickhouse.Writer
	kind          RowKind
	topic         string
	flushInterval time.Duration
	retryInterval time.Duration
	log           *zap.Logger
}

func NewTopicConsumer(fetcher Fetcher, writer *pkgclickhouse.Writer, kind RowKind, topic string, flushInterval time.Duration, log *zap.Logger) *TopicConsumer {
	return &TopicConsumer{
		fetcher:       fetcher,
		writer:        writer,
		kind:          kind,
		topic:         topic,
		flushInterval: flushInterval,
		retryInterval: time.Second,
		log:           log.Named("ingest"),
	}
}

func (c *TopicConsumer) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	msgs := make(chan kafka.Message)

	go c.fetchLoop(ctx, msgs)

	var uncommitted []kafka.Message

	commit := func() {
		if len(uncommitted) == 0 {
			return
		}

		if err := c.fetcher.CommitMessages(ctx, uncommitted...); err != nil {
			c.log.Warn(
				"ingest commit failed",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.Error(err),
			)

			return
		}

		uncommitted = nil
	}

	flush := func() bool {
		for {
			if err := c.writer.Flush(ctx); err != nil {
				c.log.Warn(
					"ingest flush failed; retrying",
					zap.String(logger.FieldKafkaTopic, c.topic),
					zap.Error(err),
				)

				select {
				case <-ctx.Done():
					return false
				case <-time.After(c.retryInterval):
					continue
				}
			}

			commit()

			return true
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return nil
		case <-ticker.C:
			if c.writer.Pending() > 0 && !flush() {
				return nil
			}
		case msg, ok := <-msgs:
			if !ok {
				c.log.Warn(
					"ingest fetch loop exited",
					zap.String(logger.FieldKafkaTopic, c.topic),
				)
				flush()
				return nil
			}

			if c.append(msg) {
				uncommitted = append(uncommitted, msg)
			} else if err := c.fetcher.CommitMessages(ctx, msg); err != nil {
				c.log.Warn(
					"ingest poison commit failed",
					zap.String(logger.FieldKafkaTopic, c.topic),
					zap.Error(err),
				)
			}

			if c.writer.Full() && !flush() {
				return nil
			}
		}
	}
}

func (c *TopicConsumer) fetchLoop(ctx context.Context, msgs chan<- kafka.Message) {
	defer close(msgs)

	for {
		msg, err := c.fetcher.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			c.log.Warn(
				"ingest fetch failed",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.Error(err),
			)

			select {
			case <-ctx.Done():
				return
			case <-time.After(c.retryInterval):
				continue
			}
		}

		select {
		case msgs <- msg:
		case <-ctx.Done():
			return
		}
	}
}

func (c *TopicConsumer) append(msg kafka.Message) bool {
	env, err := outbox.ParseEnvelope(msg.Value)
	if err != nil {
		c.log.Warn(
			"ingest envelope parse failed",
			zap.String(logger.FieldKafkaTopic, c.topic),
			zap.Error(err),
		)

		return false
	}

	switch c.kind {
	case RowKindEvent:
		if env.Type != events.EventEnvelopeType {
			c.log.Warn(
				"ingest record type skipped",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.String(logger.FieldEventType, env.Type),
			)

			return false
		}

		row, err := EventRowFromEnvelope(env)
		if err != nil {
			c.log.Warn(
				"ingest event decode failed",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.String(logger.FieldOutboxID, env.ID),
				zap.Error(err),
			)

			return false
		}

		c.writer.AppendEvent(row)
	case RowKindExposure:
		if env.Type != events.ExposureEnvelopeType {
			c.log.Warn(
				"ingest record type skipped",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.String(logger.FieldEventType, env.Type),
			)

			return false
		}

		row, err := ExposureRowFromEnvelope(env)
		if err != nil {
			c.log.Warn(
				"ingest exposure decode failed",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.String(logger.FieldOutboxID, env.ID),
				zap.Error(err),
			)

			return false
		}

		c.writer.AppendExposure(row)
	case RowKindDecision:
		if env.Type != "decision" {
			c.log.Warn(
				"ingest record type skipped",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.String(logger.FieldEventType, env.Type),
			)

			return false
		}

		row, err := DecisionRowFromEnvelope(env)
		if err != nil {
			c.log.Warn(
				"ingest decision decode failed",
				zap.String(logger.FieldKafkaTopic, c.topic),
				zap.String(logger.FieldOutboxID, env.ID),
				zap.Error(err),
			)

			return false
		}

		c.writer.AppendDecision(row)
	default:
		return false
	}

	return true
}
