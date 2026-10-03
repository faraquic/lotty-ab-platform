package main

import (
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/metrics"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	decidedomain "github.com/faraquic/lotty-ab-platform/services/runtime/domain/decide"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/runtime/domain/health"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/rueidis"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func NewApp(fiberConfig *fiber.Config, log *zap.Logger, cfg *config.Config, redisClient *rueidis.Client, decisionsWriter *kafka.Writer) (*fiber.App, *snapshot.Reader, func()) {
	app := fiber.New(*fiberConfig)

	addMiddleware(app, cfg, log)

	apiV1 := app.Group("/api/v1/runtime")

	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	snapReader := snapshot.NewReader(newSnapshotConsumer(cfg, log), redisClient, log,
		snapshot.WithRevalidation(cfg.Runtime.RevalidateInterval),
	)

	healthdomain.NewHandler().RegisterRoutes(apiV1)

	var publisher *decidedomain.DecisionPublisher
	if decisionsWriter != nil {
		publisher = decidedomain.NewDecisionPublisher(decisionsWriter, 1024, log)
	}

	decideRepo := decidedomain.NewRepository(snapReader, log)
	var decideSvc *decidedomain.Service
	if publisher != nil {
		decideSvc = decidedomain.NewServiceWithPublisher(decideRepo, publisher, cfg.Runtime.MaxStaleAge, log)
	} else {
		decideSvc = decidedomain.NewService(decideRepo, cfg.Runtime.MaxStaleAge, log)
	}
	decideHandler := decidedomain.NewHandler(decideSvc, log)
	decideHandler.RegisterRoutes(apiV1)

	cleanup := func() {
		if publisher != nil {
			publisher.Stop()
		}
	}

	return app, snapReader, cleanup
}

// newSnapshotConsumer builds the snapshot topic consumer without pinging the
// brokers: runtime must start even when Kafka is down, serving the Redis
// bootstrap until the subscription catches up. It returns nil when no brokers
// are configured, which disables the subscription entirely — kafka.NewReader
// panics on an empty broker list, and Kafka is optional in this project.
func newSnapshotConsumer(cfg *config.Config, log *zap.Logger) *kafka.Reader {
	brokers := database.ParseBrokers(cfg.Database.Kafka.Brokers)
	if len(brokers) == 0 {
		log.Info("snapshot kafka consumer disabled; redis revalidation only")

		return nil
	}

	groupID := cfg.Runtime.Kafka.GroupID
	if groupID == "" {
		groupID = "labp-runtime-snapshot"
	}

	k := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       database.TopicSnapshot,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})

	log.Info("snapshot kafka consumer configured",
		zap.String(logger.FieldKafkaGroupID, groupID),
		zap.String(logger.FieldKafkaTopic, database.TopicSnapshot),
		zap.Strings(logger.FieldKafkaBrokers, brokers),
	)

	return k
}

func addMiddleware(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	app.Use(middleware.RecoveryFiber(log))
	app.Use(middleware.RequestIDFiber())
	app.Use(middleware.LoggerFiber(log))
	app.Use(metrics.MiddlewareFiber("runtime"))
	app.Use(corsMiddleware(cfg.Runtime.HTTP.CORS))
}

func corsMiddleware(cfg config.CORSConfig) fiber.Handler {
	corsCfg := cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		ExposeHeaders:    cfg.ExposeHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           int(cfg.MaxAge.Seconds()),
	}

	return cors.New(corsCfg)
}
