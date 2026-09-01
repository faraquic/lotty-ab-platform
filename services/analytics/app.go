package main

import (
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/analytics/domain/health"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"go.uber.org/zap"
)

func NewApp(fiberConfig *fiber.Config, log *zap.Logger, cfg *config.Config) *fiber.App {
	app := fiber.New(*fiberConfig)

	addMiddleware(app, cfg, log)

	apiV1 := app.Group("/api/v1/analytics")

	healthdomain.NewHandler(cfg.Environment, log).RegisterRoutes(apiV1)

	return app
}

func addMiddleware(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	app.Use(middleware.RecoveryFiber(log))
	app.Use(middleware.RequestIDFiber())
	app.Use(middleware.LoggerFiber(log))
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
