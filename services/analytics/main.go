package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/services/analytics/domain/ingest"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/rueidis"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad("analytics")
	log := logger.SetupLogger(cfg.Environment, cfg.LogLevel)

	log.Info(
		"analytics service starting",
		zap.String(logger.FieldServiceName, config.ServiceName),
		zap.String(logger.FieldServiceVersion, config.ServiceVersion),
		zap.String(logger.FieldEnvironment, cfg.Environment),
		zap.String(logger.FieldServerAddress, cfg.Analytics.HTTP.Address),
	)

	config.ValidateSecurity(cfg, log)

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 5*time.Second)
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

	var redis *rueidis.Client
	redis, err = database.NewRedis(connectCtx, cfg.Database.Redis.Address, log)
	if err != nil {
		log.Warn("redis unavailable; attribution disabled",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.Error(err),
		)
		redis = nil
	} else {
		defer (*redis).Close()
	}

	brokers := database.ParseBrokers(cfg.Database.Kafka.Brokers)
	writer := startEventsWriter(connectCtx, cfg, brokers, log)
	if writer != nil {
		defer writer.Close()
	}

	ingestStop := ingest.StartPipeline(connectCtx, cfg, brokers, postgres, redis, log)
	defer ingestStop()

	app := NewApp(&fiber.Config{
		ReadTimeout:  cfg.Analytics.HTTP.Timeout,
		WriteTimeout: cfg.Analytics.HTTP.Timeout,
		IdleTimeout:  cfg.Analytics.HTTP.IdleTimeout,
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
	}, log, cfg, postgres, writer, brokers)

	log.Info(
		"server listening",
		zap.String(logger.FieldServerAddress, cfg.Analytics.HTTP.Address),
	)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	listenConfig := fiber.ListenConfig{
		ShutdownTimeout:       cfg.Analytics.HTTP.ShutdownTimeout,
		DisableStartupMessage: true,
	}

	go func() {
		if err := app.Listen(cfg.Analytics.HTTP.Address, listenConfig); err != nil {
			log.Error("server listen failed", zap.Error(err))
			os.Exit(1)
		}
	}()

	<-done
	log.Info(
		"graceful shutdown requested",
		zap.String(logger.FieldServerAddress, cfg.Analytics.HTTP.Address),
	)

	if err := app.Shutdown(); err != nil {
		log.Error(
			"graceful shutdown failed; forcing stop",
			zap.Error(err),
			zap.Duration(logger.FieldShutdownTimeout, cfg.Analytics.HTTP.ShutdownTimeout),
		)

		return
	}

	log.Info(
		"graceful shutdown completed",
		zap.String(logger.FieldServerAddress, cfg.Analytics.HTTP.Address),
	)
}

func startEventsWriter(ctx context.Context, cfg *config.Config, brokers []string, log *zap.Logger) *kafka.Writer {
	if len(brokers) == 0 {
		log.Warn("kafka brokers are empty; event ingestion disabled")
		return nil
	}

	writer, err := database.NewBroadcastWriter(ctx, brokers, log)
	if err != nil {
		log.Warn("kafka unavailable; event ingestion disabled",
			zap.Strings(logger.FieldKafkaBrokers, brokers),
			zap.Error(err),
		)
		return nil
	}

	for _, topic := range []string{cfg.Analytics.Kafka.EventsTopic, cfg.Analytics.Kafka.ExposuresTopic} {
		if topic == "" {
			continue
		}

		if err := database.EnsureTopic(ctx, brokers, topic, 6, 1); err != nil {
			log.Warn("events topic ensure failed",
				zap.String(logger.FieldKafkaTopic, topic),
				zap.Error(err),
			)
		}
	}

	return writer
}
