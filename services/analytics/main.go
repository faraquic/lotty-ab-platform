package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
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

	app := NewApp(&fiber.Config{
		ReadTimeout:  cfg.Analytics.HTTP.Timeout,
		WriteTimeout: cfg.Analytics.HTTP.Timeout,
		IdleTimeout:  cfg.Analytics.HTTP.IdleTimeout,
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
	}, log, cfg)

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
