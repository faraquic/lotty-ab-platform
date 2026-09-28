package ingest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	pkgclickhouse "github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/services/analytics/domain/attribution"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	catalogRefreshInterval = time.Minute
	validationPartitions   = 6
	dlqPartitions          = 3
)

func StartPipeline(ctx context.Context, cfg *config.Config, brokers []string, pool *pgxpool.Pool, redis *rueidis.Client, log *zap.Logger) func() {
	noop := func() {}

	if len(brokers) == 0 {
		log.Warn("kafka brokers are empty; analytics pipeline disabled")
		return noop
	}

	topics := cfg.Analytics.Kafka

	if topics.EventsTopic == "" || topics.EventsValidatedTopic == "" || topics.EventsDedupedTopic == "" || topics.DLQTopic == "" || topics.DecisionsTopic == "" {
		log.Error("analytics pipeline topics are not fully configured; pipeline disabled")
		return noop
	}

	if topics.EventsTopic == topics.EventsValidatedTopic ||
		topics.EventsTopic == topics.EventsDedupedTopic ||
		topics.EventsValidatedTopic == topics.EventsDedupedTopic {
		log.Error("analytics pipeline event topics must be distinct; pipeline disabled")
		return noop
	}

	groupID := topics.PipelineGroupID
	if groupID == "" {
		groupID = topics.GroupID
	}

	ingestGroupID := topics.IngestGroupID
	if ingestGroupID == "" {
		ingestGroupID = groupID
	}

	attributionGroupID := topics.AttributionGroupID
	if attributionGroupID == "" {
		attributionGroupID = groupID
	}

	dlqWriter := mustWriter(ctx, brokers, topics.DLQTopic, log)
	validatedWriter := mustWriter(ctx, brokers, topics.EventsValidatedTopic, log)
	dedupedWriter := mustWriter(ctx, brokers, topics.EventsDedupedTopic, log)

	if dlqWriter == nil || validatedWriter == nil || dedupedWriter == nil {
		closeWriters(dlqWriter, validatedWriter, dedupedWriter)
		log.Error("analytics pipeline writers unavailable; pipeline disabled")
		return noop
	}

	for _, topic := range []string{topics.EventsValidatedTopic, topics.EventsDedupedTopic} {
		if err := database.EnsureTopic(ctx, brokers, topic, validationPartitions, 1); err != nil {
			log.Warn("pipeline topic ensure failed", zap.String(logger.FieldKafkaTopic, topic), zap.Error(err))
		}
	}

	if err := database.EnsureTopic(ctx, brokers, topics.DLQTopic, dlqPartitions, 1); err != nil {
		log.Warn("pipeline topic ensure failed", zap.String(logger.FieldKafkaTopic, topics.DLQTopic), zap.Error(err))
	}

	catalog, err := NewValidationCatalog(pgEventCatalog{db: pool}, catalogRefreshInterval, log)
	if err != nil {
		closeWriters(dlqWriter, validatedWriter, dedupedWriter)
		log.Error("validation catalog init failed; pipeline disabled", zap.Error(err))
		return noop
	}

	validator := &Validator{catalog: catalog, now: time.Now}
	dedup := NewDedupStore(pool, log)

	eventsRawReader := mustReader(ctx, brokers, topics.EventsTopic, groupID, log)
	validatedReader := mustReader(ctx, brokers, topics.EventsValidatedTopic, groupID, log)
	dedupedReader := mustReader(ctx, brokers, topics.EventsDedupedTopic, ingestGroupID, log)
	exposuresReader := mustReader(ctx, brokers, topics.ExposuresTopic, ingestGroupID, log)
	decisionsReader := mustReader(ctx, brokers, topics.DecisionsTopic, ingestGroupID, log)

	if eventsRawReader == nil || validatedReader == nil {
		closeWriters(dlqWriter, validatedWriter, dedupedWriter)
		closeReaders(eventsRawReader, validatedReader)
		log.Error("analytics pipeline readers unavailable; pipeline disabled")
		return noop
	}

	runCtx, stop := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	startConsumer := func(name string, r *kafka.Reader, topic string, h consumer.MessageHandler, fwd *kafka.Writer, fwdTopic string) {
		if r == nil {
			log.Warn("consumer skipped: reader unavailable", zap.String("consumer", name))
			return
		}

		c := consumer.New(r, h, consumer.Options{
			Topic:        topic,
			Forward:      fwd,
			ForwardTopic: fwdTopic,
			DLQ:          dlqWriter,
			DLQTopic:     topics.DLQTopic,
			GroupID:      groupID,
		}, log)

		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := c.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("pipeline consumer stopped", zap.String("consumer", name), zap.Error(err))
			}
		}()
	}

	startConsumer("validator", eventsRawReader, topics.EventsTopic, validator, validatedWriter, topics.EventsValidatedTopic)
	startConsumer("dedup", validatedReader, topics.EventsValidatedTopic, dedup, dedupedWriter, topics.EventsDedupedTopic)

	catalog.Start(runCtx)

	var chConn clickhouse.Conn
	var chWriter *pkgclickhouse.Writer
	flushInterval := cfg.Analytics.ClickHouse.FlushInterval
	if flushInterval <= 0 {
		flushInterval = time.Second
	}

	chConn, err = database.NewClickHouse(ctx, cfg.Database.ClickHouse.DSN, log)
	if err != nil {
		log.Warn("clickhouse unavailable; clickhouse ingest disabled", zap.Error(err))
	} else {
		chWriter = pkgclickhouse.NewWriter(pkgclickhouse.WrapConn(chConn), cfg.Analytics.ClickHouse.BatchSize, log)
	}

	startAttribution(ctx, cfg, brokers, pool, redis, log, topics, attributionGroupID, runCtx, &wg, dlqWriter, chWriter)

	if chWriter != nil {
		startCHConsumer := func(name string, r *kafka.Reader, kind RowKind, topic string) {
			if r == nil {
				log.Warn("clickhouse consumer skipped: reader unavailable", zap.String("consumer", name))
				return
			}

			c := NewTopicConsumer(r, chWriter, kind, topic, flushInterval, log)

			wg.Add(1)

			go func() {
				defer wg.Done()

				if err := c.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
					log.Error("clickhouse consumer stopped", zap.String("consumer", name), zap.Error(err))
				}
			}()
		}

		startCHConsumer("clickhouse-events", dedupedReader, RowKindEvent, topics.EventsDedupedTopic)
		startCHConsumer("clickhouse-exposures", exposuresReader, RowKindExposure, topics.ExposuresTopic)
		startCHConsumer("clickhouse-decisions", decisionsReader, RowKindDecision, topics.DecisionsTopic)
	}

	return func() {
		stop()
		wg.Wait()

		closeReaders(eventsRawReader, validatedReader, dedupedReader, exposuresReader, decisionsReader)

		if chConn != nil {
			flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := chWriter.Flush(flushCtx); err != nil {
				log.Warn("pipeline final flush failed", zap.Error(err))
			}

			_ = chConn.Close()
		}

		closeWriters(dlqWriter, validatedWriter, dedupedWriter)
	}
}

func mustWriter(ctx context.Context, brokers []string, topic string, log *zap.Logger) *kafka.Writer {
	w, err := database.NewWriter(ctx, brokers, topic, log)
	if err != nil {
		log.Warn("kafka writer unavailable", zap.String(logger.FieldKafkaTopic, topic), zap.Error(err))
		return nil
	}

	return w
}

func mustReader(ctx context.Context, brokers []string, topic, groupID string, log *zap.Logger) *kafka.Reader {
	r, err := database.NewReader(ctx, brokers, topic, groupID, log)
	if err != nil {
		log.Warn("kafka reader unavailable", zap.String(logger.FieldKafkaTopic, topic), zap.Error(err))
		return nil
	}

	return r
}

func closeWriters(writers ...*kafka.Writer) {
	for _, w := range writers {
		if w != nil {
			_ = w.Close()
		}
	}
}

func closeReaders(readers ...*kafka.Reader) {
	for _, r := range readers {
		if r != nil {
			_ = r.Close()
		}
	}
}

func startAttribution(
	ctx context.Context,
	cfg *config.Config,
	brokers []string,
	pool *pgxpool.Pool,
	redis *rueidis.Client,
	log *zap.Logger,
	topics config.AnalyticsKafkaConfig,
	attributionGroupID string,
	runCtx context.Context,
	wg *sync.WaitGroup,
	dlqWriter *kafka.Writer,
	chWriter *pkgclickhouse.Writer,
) {
	if redis == nil {
		log.Warn("redis unavailable; attribution disabled")
		return
	}

	window := time.Duration(cfg.Analytics.Attribution.WindowDays) * 24 * time.Hour
	if window <= 0 {
		window = 7 * 24 * time.Hour
	}

	grace := time.Duration(cfg.Analytics.Attribution.LateEventGraceHours) * time.Hour
	if grace <= 0 {
		grace = time.Hour
	}

	attrDedupedReader := mustReader(ctx, brokers, topics.EventsDedupedTopic, attributionGroupID, log)
	attrExposuresReader := mustReader(ctx, brokers, topics.ExposuresTopic, attributionGroupID, log)

	if attrDedupedReader == nil || attrExposuresReader == nil {
		closeReaders(attrDedupedReader, attrExposuresReader)
		log.Error("attribution readers unavailable; attribution disabled")
		return
	}

	pendingStore := attribution.NewRedisPendingStore(redis, log)

	exposureCatalog, err := attribution.NewExposureCatalog(
		attribution.PGExposureCatalog{DB: pool},
		catalogRefreshInterval,
		log,
	)
	if err != nil {
		closeReaders(attrDedupedReader, attrExposuresReader)
		log.Error("attribution catalog init failed; attribution disabled", zap.Error(err))
		return
	}

	exposureCatalog.Start(runCtx)

	startAttrConsumer := func(name string, r *kafka.Reader, topic string, h consumer.MessageHandler) {
		if r == nil {
			log.Warn("attribution consumer skipped: reader unavailable", zap.String("consumer", name))
			return
		}

		c := consumer.New(r, h, consumer.Options{
			Topic:    topic,
			DLQ:      dlqWriter,
			DLQTopic: topics.DLQTopic,
			GroupID:  attributionGroupID,
		}, log)

		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := c.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("attribution consumer stopped", zap.String("consumer", name), zap.Error(err))
			}
		}()
	}

	exposureRegistry := attribution.NewExposureRegistry(pendingStore, window, grace, time.Now, log)
	startAttrConsumer("exposure-registry", attrExposuresReader, topics.ExposuresTopic, exposureRegistry)

	if chWriter != nil {
		attributor := attribution.NewAttributor(
			pendingStore,
			exposureCatalog,
			attribution.NewCHWriterAdapter(chWriter),
			window,
			grace,
			time.Now,
			log,
		)
		startAttrConsumer("attribution", attrDedupedReader, topics.EventsDedupedTopic, attributor)

		sweeper := attribution.NewSweeper(
			pendingStore,
			attribution.NewCHWriterAdapter(chWriter),
			time.Now,
			log,
		)
		sweeper.Run(runCtx, time.Minute)
	} else {
		closeReaders(attrDedupedReader)
		log.Warn("clickhouse unavailable; attribution consumer disabled")
	}
}
