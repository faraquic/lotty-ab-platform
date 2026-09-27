package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const TopicSnapshot = "snapshot"

// batchFlushInterval bounds how long a writer waits to fill a batch.
// Control-plane messages are rare and latency-sensitive, so flush fast
// instead of waiting out kafka-go's 1s default.
const batchFlushInterval = 20 * time.Millisecond

// ParseBrokers splits a KAFKA_BROKERS-style value ("host:port" or a
// comma-separated "host:port,..." list) into broker addresses.
func ParseBrokers(brokers string) []string {
	var out []string

	for _, broker := range strings.Split(brokers, ",") {
		if broker = strings.TrimSpace(broker); broker != "" {
			out = append(out, broker)
		}
	}

	return out
}

// PingBrokers dials each broker and succeeds when at least one of them
// accepts a connection.
func PingBrokers(ctx context.Context, brokers []string) error {
	if len(brokers) == 0 {
		return errors.New("kafka brokers are empty")
	}

	var err error

	for _, broker := range brokers {
		dialCtx, cancel := context.WithTimeout(ctx, pingTimeout)

		var conn *kafka.Conn

		conn, err = kafka.DialContext(dialCtx, "tcp", broker)
		cancel()

		if err != nil {
			continue
		}

		if closeErr := conn.Close(); closeErr != nil {
			return fmt.Errorf("close kafka connection: %w", closeErr)
		}

		return nil
	}

	return fmt.Errorf("ping kafka %v: %w", brokers, err)
}

// NewWriter returns a Writer for topic after verifying that at least one
// broker is reachable. The caller owns the writer and must close it.
func NewWriter(ctx context.Context, brokers []string, topic string, log *zap.Logger) (*kafka.Writer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are empty")
	}

	if topic == "" {
		return nil, errors.New("kafka topic is empty")
	}

	if err := PingBrokers(ctx, brokers); err != nil {
		return nil, err
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		BatchTimeout: batchFlushInterval,
	}

	log.Info(
		"connected to kafka",
		zap.Strings(logger.FieldKafkaBrokers, brokers),
		zap.String(logger.FieldKafkaTopic, topic),
	)

	return writer, nil
}

// NewBroadcastWriter returns a Writer without a fixed topic after verifying
// that at least one broker is reachable. Each message must carry its own
// topic. Used by outbox relays. The caller owns the writer and must close it.
func NewBroadcastWriter(ctx context.Context, brokers []string, log *zap.Logger) (*kafka.Writer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are empty")
	}

	if err := PingBrokers(ctx, brokers); err != nil {
		return nil, err
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		BatchTimeout: batchFlushInterval,
	}

	log.Info(
		"connected to kafka",
		zap.Strings(logger.FieldKafkaBrokers, brokers),
	)

	return writer, nil
}

// EnsureTopic creates topic with the given partitions and replication
// factor. An already existing topic is not an error.
func EnsureTopic(ctx context.Context, brokers []string, topic string, partitions, replicationFactor int) error {
	if len(brokers) == 0 {
		return errors.New("kafka brokers are empty")
	}

	if topic == "" {
		return errors.New("kafka topic is empty")
	}

	if partitions <= 0 {
		partitions = 1
	}

	if replicationFactor <= 0 {
		replicationFactor = 1
	}

	res, err := (&kafka.Client{Addr: kafka.TCP(brokers...)}).CreateTopics(ctx, &kafka.CreateTopicsRequest{
		Topics: []kafka.TopicConfig{
			{
				Topic:             topic,
				NumPartitions:     partitions,
				ReplicationFactor: replicationFactor,
			},
		},
	})
	if err != nil {
		return err
	}

	if terr := res.Errors[topic]; terr != nil && !errors.Is(terr, kafka.TopicAlreadyExists) {
		return fmt.Errorf("create topic %s: %w", topic, terr)
	}

	return nil
}

// IsUnknownTopic reports whether err is (or contains) an unknown-topic
// response. WriteMessages returns WriteErrors, which does not unwrap, so
// each entry is inspected.
func IsUnknownTopic(err error) bool {
	var werr kafka.WriteErrors
	if errors.As(err, &werr) {
		for _, e := range werr {
			if errors.Is(e, kafka.UnknownTopicOrPartition) {
				return true
			}
		}

		return false
	}

	return errors.Is(err, kafka.UnknownTopicOrPartition)
}

// NewReader returns a consumer-group Reader for topic after verifying that
// at least one broker is reachable. A group without committed offsets starts
// from the first offset. The caller owns the reader and must close it.
func NewReader(ctx context.Context, brokers []string, topic string, groupID string, log *zap.Logger) (*kafka.Reader, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are empty")
	}

	if topic == "" {
		return nil, errors.New("kafka topic is empty")
	}

	if groupID == "" {
		return nil, errors.New("kafka group id is empty")
	}

	if err := PingBrokers(ctx, brokers); err != nil {
		return nil, err
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	log.Info(
		"connected to kafka",
		zap.Strings(logger.FieldKafkaBrokers, brokers),
		zap.String(logger.FieldKafkaTopic, topic),
		zap.String(logger.FieldKafkaGroupID, groupID),
	)

	return reader, nil
}
