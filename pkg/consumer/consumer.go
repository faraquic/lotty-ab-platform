package consumer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

var (
	ErrDrop         = errors.New("consumer: drop message")
	ErrNonRetryable = errors.New("consumer: non-retryable error")
)

type MessageFetcher interface {
	ReadMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

type MessageHandler interface {
	Handle(ctx context.Context, msg kafka.Message) error
	IsRetryable(err error) bool
}

type HandlerFunc func(ctx context.Context, msg kafka.Message) error

func (f HandlerFunc) Handle(ctx context.Context, msg kafka.Message) error {
	return f(ctx, msg)
}

func (f HandlerFunc) IsRetryable(error) bool {
	return true
}

const (
	defaultMaxAttempts = 3
	defaultBaseBackoff = time.Second
	defaultMaxBackoff  = time.Minute
)

type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type Options struct {
	Topic        string
	Forward      MessageWriter
	ForwardTopic string
	DLQ          MessageWriter
	DLQTopic     string
	GroupID      string
	MaxAttempts  int
	BaseBackoff  time.Duration
	MaxBackoff   time.Duration
}

type Consumer struct {
	fetcher MessageFetcher
	handler MessageHandler
	opts    Options
	log     *zap.Logger
}

func New(fetcher MessageFetcher, handler MessageHandler, opts Options, log *zap.Logger) *Consumer {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultMaxAttempts
	}

	if opts.BaseBackoff <= 0 {
		opts.BaseBackoff = defaultBaseBackoff
	}

	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = defaultMaxBackoff
	}

	if opts.MaxBackoff < opts.BaseBackoff {
		opts.MaxBackoff = opts.BaseBackoff
	}

	return &Consumer{
		fetcher: fetcher,
		handler: handler,
		opts:    opts,
		log:     log,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	if c.fetcher == nil {
		return errors.New("consumer: fetcher is nil")
	}

	topic := c.opts.Topic

	for {
		msg, err := c.fetcher.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			c.log.Warn("consumer read failed", zap.String(logger.FieldKafkaTopic, topic), zap.Error(err))

			continue
		}

		attempts, err := c.process(ctx, msg)
		switch {
		case err == nil:
		case errors.Is(err, ErrDrop):
			c.log.Debug("message dropped by handler", zap.String(logger.FieldKafkaTopic, topic))
		default:
			c.sendDLQ(ctx, msg, attempts, err)
		}

		if err := c.fetcher.CommitMessages(ctx, msg); err != nil {
			c.log.Warn("consumer commit failed", zap.String(logger.FieldKafkaTopic, topic), zap.Error(err))
		}
	}
}

func (c *Consumer) process(ctx context.Context, msg kafka.Message) (int, error) {
	topic := c.opts.Topic

	for attempt := 1; ; attempt++ {
		err := c.handler.Handle(ctx, msg)
		if err == nil && c.opts.Forward != nil {
			fwd := kafka.Message{Key: msg.Key, Value: msg.Value}

			if err = c.opts.Forward.WriteMessages(ctx, fwd); err == nil {
				return attempt, nil
			}

			err = fmt.Errorf("forward to %s: %w", c.opts.ForwardTopic, err)
		}

		if errors.Is(err, ErrDrop) {
			return attempt, err
		}

		if !c.handler.IsRetryable(err) || attempt >= c.opts.MaxAttempts {
			return attempt, err
		}

		delay := c.backoff(attempt)

		c.log.Warn(
			"consumer handler failed; retrying",
			zap.String(logger.FieldKafkaTopic, topic),
			zap.Int("attempt", attempt),
			zap.Duration("backoff", delay),
			zap.Error(err),
		)

		select {
		case <-ctx.Done():
			return attempt, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (c *Consumer) backoff(attempt int) time.Duration {
	delay := c.opts.BaseBackoff

	for i := 1; i < attempt && delay < c.opts.MaxBackoff; i++ {
		delay *= 2
	}

	if delay > c.opts.MaxBackoff {
		return c.opts.MaxBackoff
	}

	return delay
}

func (c *Consumer) sendDLQ(ctx context.Context, msg kafka.Message, attempts int, cause error) {
	if c.opts.DLQ == nil {
		c.log.Error(
			"message dead but no DLQ configured",
			zap.String(logger.FieldKafkaTopic, c.opts.Topic),
			zap.Error(cause),
		)

		return
	}

	groupID := c.opts.GroupID

	out := kafka.Message{
		Key:   msg.Key,
		Value: msg.Value,
		Headers: []kafka.Header{
			{Key: "dlq.origin_topic", Value: []byte(c.opts.Topic)},
			{Key: "dlq.origin_group", Value: []byte(groupID)},
			{Key: "dlq.attempts", Value: []byte(strconv.Itoa(attempts))},
			{Key: "dlq.error", Value: []byte(cause.Error())},
		},
	}

	if err := c.opts.DLQ.WriteMessages(ctx, out); err != nil {
		c.log.Error(
			"dlq write failed",
			zap.String(logger.FieldKafkaTopic, c.opts.DLQTopic),
			zap.Error(err),
		)

		return
	}

	c.log.Error(
		"message moved to dlq",
		zap.String(logger.FieldKafkaTopic, c.opts.DLQTopic),
		zap.String("dlq.origin_topic", c.opts.Topic),
		zap.String(logger.FieldKafkaGroupID, groupID),
		zap.Int("dlq.attempts", attempts),
		zap.Error(cause),
	)
}
