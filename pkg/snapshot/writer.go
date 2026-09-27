package snapshot

import (
	"context"
	"strconv"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	keySnapshot    = "labp:runtime:snapshot"
	keySnapshotRev = "labp:runtime:snapshot:rev"
	OpTimeout      = 5 * time.Second
)

// Writer persists snapshots to Redis and notifies runtimes through Kafka.
// The kafka writer must be topic-less (see database.NewBroadcastWriter):
// the snapshot topic is set on each message. A nil kafka writer disables
// publishing: Redis stays the source of truth and runtimes keep serving the
// last stored snapshot.
type Writer struct {
	r       *rueidis.Client
	brokers []string
	k       *kafka.Writer
	log     *zap.Logger
}

func NewWriter(r *rueidis.Client, brokers []string, k *kafka.Writer, log *zap.Logger) *Writer {
	return &Writer{r: r, brokers: brokers, k: k, log: log.Named("snapshot_writer")}
}

func (w *Writer) Set(s *Snapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), OpTimeout)
	defer cancel()
	return w.SetWithContext(ctx, s)
}

func (w *Writer) SetWithContext(ctx context.Context, s *Snapshot) error {
	b, err := json.Marshal(s)
	if err != nil {
		w.log.Error("failed to marshal snapshot", zap.Error(err))
		return err
	}

	rev := strconv.FormatUint(uint64(s.Revision), 10)

	cmds := []rueidis.Completed{
		(*w.r).B().Set().Key(keySnapshot).Value(string(b)).Build(),
		(*w.r).B().Set().Key(keySnapshotRev).Value(rev).Build(),
	}

	for _, resp := range (*w.r).DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			w.log.Error("failed to store snapshot", zap.Error(err))
			return err
		}
	}

	w.log.Debug(
		"snapshot stored",
		zap.String(logger.FieldCacheKeyNS, keySnapshot),
		zap.Uint64(logger.FieldSnapshotRevision, uint64(s.Revision)),
	)

	if w.k == nil {
		w.log.Warn("kafka writer unavailable; snapshot published to redis only")
		return nil
	}

	msg := kafka.Message{
		Topic: database.TopicSnapshot,
		Key:   []byte(rev),
		Value: b,
		Headers: []kafka.Header{
			{Key: "revision", Value: []byte(rev)},
		},
	}

	if err := w.publish(ctx, msg); err != nil {
		w.log.Error("failed to publish snapshot", zap.Error(err))
		return err
	}

	w.log.Debug(
		"snapshot published",
		zap.String(logger.FieldKafkaTopic, database.TopicSnapshot),
		zap.Uint64(logger.FieldSnapshotRevision, uint64(s.Revision)),
	)

	return nil
}

// publish sends msg, creating the snapshot topic first when the broker
// reports it as unknown (kafka-go metadata requests do not trigger
// server-side auto-creation). Only one creation attempt is made.
func (w *Writer) publish(ctx context.Context, msg kafka.Message) error {
	if err := w.k.WriteMessages(ctx, msg); err != nil {
		if !database.IsUnknownTopic(err) {
			return err
		}

		if ensureErr := database.EnsureTopic(ctx, w.brokers, database.TopicSnapshot, 1, 1); ensureErr != nil {
			w.log.Warn(
				"snapshot topic ensure failed",
				zap.String(logger.FieldKafkaTopic, database.TopicSnapshot),
				zap.Error(ensureErr),
			)

			return err
		}

		w.log.Info(
			"snapshot topic ensured",
			zap.String(logger.FieldKafkaTopic, database.TopicSnapshot),
		)

		return w.k.WriteMessages(ctx, msg)
	}

	return nil
}
