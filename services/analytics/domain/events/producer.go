package events

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/goccy/go-json"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	EventEnvelopeType    = "event"
	ExposureEnvelopeType = "exposure"
)

type Record struct {
	Topic   string
	Key     string
	ID      string
	Type    string
	Payload any
}

type Producer struct {
	writer  *kafka.Writer
	brokers []string
	log     *zap.Logger
	timeout time.Duration
}

func NewProducer(writer *kafka.Writer, brokers []string, log *zap.Logger) *Producer {
	return &Producer{
		writer:  writer,
		brokers: brokers,
		log:     log.Named("events_producer"),
		timeout: 10 * time.Second,
	}
}

func (p *Producer) Publish(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	if p.writer == nil {
		return ErrBrokerUnavailable
	}

	msgs := make([]kafka.Message, 0, len(records))

	for _, rec := range records {
		data, err := json.Marshal(rec.Payload)
		if err != nil {
			return err
		}

		envelope, err := json.Marshal(outbox.Envelope{
			ID:     rec.ID,
			Type:   rec.Type,
			Source: config.ServiceName,
			Time:   time.Now().UTC(),
			Data:   data,
		})
		if err != nil {
			return err
		}

		msg := kafka.Message{Topic: rec.Topic, Value: envelope}
		if key := strings.TrimSpace(rec.Key); key != "" {
			msg.Key = []byte(key)
		}

		msgs = append(msgs, msg)
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if err := p.writer.WriteMessages(publishCtx, msgs...); err != nil {
		if !database.IsUnknownTopic(err) {
			return errors.Join(ErrBrokerUnavailable, err)
		}

		if ensureErr := p.ensureTopics(publishCtx, records); ensureErr != nil {
			return errors.Join(ErrBrokerUnavailable, err)
		}

		retryCtx, retryCancel := context.WithTimeout(ctx, p.timeout)
		defer retryCancel()

		if err := p.writer.WriteMessages(retryCtx, msgs...); err != nil {
			return errors.Join(ErrBrokerUnavailable, err)
		}
	}

	p.log.Info(
		"events published",
		zap.Int(logger.FieldBatchSize, len(msgs)),
	)

	return nil
}

func (p *Producer) ensureTopics(ctx context.Context, records []Record) error {
	seen := map[string]struct{}{}

	for _, rec := range records {
		if _, ok := seen[rec.Topic]; ok {
			continue
		}
		seen[rec.Topic] = struct{}{}

		if err := database.EnsureTopic(ctx, p.brokers, rec.Topic, 6, 1); err != nil {
			p.log.Warn(
				"events topic ensure failed",
				zap.String(logger.FieldKafkaTopic, rec.Topic),
				zap.Error(err),
			)

			return err
		}

		p.log.Info(
			"events topic ensured",
			zap.String(logger.FieldKafkaTopic, rec.Topic),
		)
	}

	return nil
}

func (p *Producer) Close() error {
	if p.writer == nil {
		return nil
	}

	return p.writer.Close()
}
