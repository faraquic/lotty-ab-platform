package main

import (
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/metrics"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/analytics/domain/health"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func NewApp(fiberConfig *fiber.Config, log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool, writer *kafka.Writer, brokers []string) *fiber.App {
	app := fiber.New(*fiberConfig)

	addMiddleware(app, cfg, log)

	apiV1 := app.Group("/api/v1/analytics")

	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	healthdomain.NewHandler().RegisterRoutes(apiV1)

	eventsRepo := events.NewRepository(pool)
	eventsProducer := events.NewProducer(writer, brokers, log)
	eventsSvc := events.NewService(eventsRepo, eventsProducer, cfg, log)
	events.NewHandler(eventsSvc, log).RegisterRoutes(apiV1)

	return app
}

func addMiddleware(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	app.Use(middleware.RecoveryFiber(log))
	app.Use(middleware.RequestIDFiber())
	app.Use(middleware.LoggerFiber(log))
	app.Use(metrics.MiddlewareFiber("analytics"))
	app.Use(corsMiddleware(cfg.Analytics.HTTP.CORS))
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
