package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad("panel")
	log := logger.SetupLogger(cfg.Environment, cfg.LogLevel)

	config.ValidateSecurity(cfg, log)

	log.Info("panel service starting",
		zap.String(logger.FieldServiceName, config.ServiceName),
		zap.String(logger.FieldServiceVersion, config.ServiceVersion),
		zap.String(logger.FieldEnvironment, cfg.Environment),
		zap.String(logger.FieldServerAddress, cfg.Panel.HTTP.Address),
	)

	connectCtx, cancelConnect := connectContext()
	defer cancelConnect()

	postgres, err := database.NewPostgres(
		connectCtx,
		cfg.Database.Postgres.DSN,
		cfg.Database.Postgres.MaxConns,
		cfg.Database.Postgres.MinConns,
		log,
	)
	if err != nil {
		log.Error("postgres unavailable; startup aborted",
			zap.String(logger.FieldDBSystem, "postgresql"),
			zap.Error(err),
		)
		os.Exit(1)
	}
	defer postgres.Close()

	redis, err := database.NewRedis(
		connectCtx,
		cfg.Database.Redis.Address,
		log,
	)
	if err != nil {
		log.Error("redis unavailable; startup aborted",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.Error(err),
		)
		os.Exit(1)
	}
	defer (*redis).Close()

	chConn, err := database.NewClickHouse(connectCtx, cfg.Database.ClickHouse.DSN, log)
	if err != nil {
		log.Warn("clickhouse unavailable; reports disabled",
			zap.String(logger.FieldDBSystem, "clickhouse"),
			zap.Error(err),
		)
	} else {
		defer chConn.Close()
	}

	s3, err := database.NewS3(
		connectCtx,
		database.S3Config{
			Bucket:    cfg.Database.S3.Bucket,
			Region:    cfg.Database.S3.Region,
			Endpoint:  cfg.Database.S3.Endpoint,
			AccessKey: cfg.Database.S3.AccessKey,
			SecretKey: cfg.Database.S3.SecretKey,
		},
		log,
	)
	if err != nil {
		log.Warn("s3 unavailable; continuing without storage",
			zap.String(logger.FieldStorageSystem, "s3"),
			zap.Error(err),
		)
	}

	outboxStop := startOutboxPublisher(connectCtx, cfg, postgres, log)

	snapKafkaWriter := startSnapshotWriter(connectCtx, cfg, log)
	if snapKafkaWriter != nil {
		defer snapKafkaWriter.Close()
	}

	setGinMode(cfg.Environment, log)
	r, refresher, snapReader := newRouter(log, cfg, postgres, redis, s3, snapKafkaWriter, chConn)

	log.Info("server listening",
		zap.String(logger.FieldServerAddress, cfg.Panel.HTTP.Address),
	)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:              cfg.Panel.HTTP.Address,
		Handler:           r,
		ReadTimeout:       cfg.Panel.HTTP.Timeout,
		ReadHeaderTimeout: cfg.Panel.HTTP.Timeout,
		WriteTimeout:      cfg.Panel.HTTP.Timeout,
		IdleTimeout:       cfg.Panel.HTTP.IdleTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server listen failed", zap.Error(err))
			os.Exit(1)
		}
	}()

	<-done
	log.Info("graceful shutdown requested",
		zap.String(logger.FieldServerAddress, cfg.Panel.HTTP.Address),
	)

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.Panel.HTTP.ShutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed; forcing stop",
			zap.Error(err),
			zap.Duration(logger.FieldShutdownTimeout, cfg.Panel.HTTP.ShutdownTimeout),
		)

		return
	}

	refresher.Stop()
	snapReader.Stop()
	outboxStop()

	log.Info("graceful shutdown completed",
		zap.String(logger.FieldServerAddress, cfg.Panel.HTTP.Address),
	)
}

func connectContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// startSnapshotWriter connects the snapshot Kafka publisher. When Kafka is
// unreachable it returns nil and the snapshot writer persists to Redis only.
func startSnapshotWriter(ctx context.Context, cfg *config.Config, log *zap.Logger) *kafka.Writer {
	brokers := database.ParseBrokers(cfg.Database.Kafka.Brokers)
	if len(brokers) == 0 {
		log.Warn("kafka brokers are empty; snapshots published to redis only")
		return nil
	}

	writer, err := database.NewBroadcastWriter(ctx, brokers, log)
	if err != nil {
		log.Warn("kafka unavailable; snapshots published to redis only",
			zap.Strings(logger.FieldKafkaBrokers, brokers),
			zap.Error(err),
		)
		return nil
	}

	return writer
}

// startOutboxPublisher connects to Kafka and runs the outbox relay in the
// background. When Kafka is unreachable it logs a warning and returns a
// no-op stop: pending rows accumulate in the outbox table and are published
// once Kafka is available and the service restarts.
func startOutboxPublisher(ctx context.Context, cfg *config.Config, postgres *pgxpool.Pool, log *zap.Logger) func() {
	noop := func() {}

	brokers := database.ParseBrokers(cfg.Database.Kafka.Brokers)
	if len(brokers) == 0 {
		log.Warn("kafka brokers are empty; outbox relay disabled")
		return noop
	}

	writer, err := database.NewBroadcastWriter(ctx, brokers, log)
	if err != nil {
		log.Warn("kafka unavailable; continuing without outbox relay",
			zap.Strings(logger.FieldKafkaBrokers, brokers),
			zap.Error(err),
		)
		return noop
	}

	publisher := outbox.NewPublisher(postgres, brokers, writer, log)

	publisherCtx, stop := context.WithCancel(context.Background())

	go func() {
		defer writer.Close()

		if err := publisher.Run(publisherCtx); err != nil {
			log.Error("outbox relay stopped", zap.Error(err))
		}
	}()

	return stop
}

func setGinMode(environment string, log *zap.Logger) {
	switch environment {
	case "prod":
		gin.SetMode(gin.ReleaseMode)
	case "dev", "local":
		gin.SetMode(gin.DebugMode)
	default:
		log.Warn("unknown environment, defaulting to release mode",
			zap.String(logger.FieldEnvironment, environment),
		)
		gin.SetMode(gin.ReleaseMode)
	}
}
