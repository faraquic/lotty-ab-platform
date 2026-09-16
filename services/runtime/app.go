package main

import (
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	decidedomain "github.com/faraquic/lotty-ab-platform/services/runtime/domain/decide"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/runtime/domain/health"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

func NewApp(fiberConfig *fiber.Config, log *zap.Logger, cfg *config.Config, redisClient *rueidis.Client) (*fiber.App, *snapshot.Reader) {
	app := fiber.New(*fiberConfig)

	addMiddleware(app, cfg, log)

	apiV1 := app.Group("/api/v1/runtime")

	snapReader := snapshot.NewReader(redisClient, log)

	healthdomain.NewHandler(redisClient, snapReader, cfg.Environment, log).RegisterRoutes(apiV1)

	decideRepo := decidedomain.NewRepository(snapReader, log)
	decideSvc := decidedomain.NewService(decideRepo, cfg.Runtime.MaxStaleAge, log)
	decideHandler := decidedomain.NewHandler(decideSvc, log)
	decideHandler.RegisterRoutes(apiV1)

	return app, snapReader
}

func addMiddleware(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	app.Use(middleware.RecoveryFiber(log))
	app.Use(middleware.RequestIDFiber())
	app.Use(middleware.LoggerFiber(log))
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
