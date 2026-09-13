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
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad("runtime")
	log := logger.SetupLogger(cfg.Environment, cfg.LogLevel)

	config.ValidateSecurity(cfg, log)

	log.Info(
		"runtime service starting",
		zap.String(logger.FieldServiceName, config.ServiceName),
		zap.String(logger.FieldServiceVersion, config.ServiceVersion),
		zap.String(logger.FieldEnvironment, cfg.Environment),
		zap.String(logger.FieldServerAddress, cfg.Runtime.HTTP.Address),
	)

	connectCtx, cancelConnect := connectContext()
	defer cancelConnect()

	redis, err := database.NewRedis(
		connectCtx,
		cfg.Database.Redis.Address,
		log,
	)
	if err != nil {
		log.Error(
			"redis unavailable; startup aborted",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.Error(err),
		)
		os.Exit(1)
	}
	defer (*redis).Close()

	app, snapReader := NewApp(&fiber.Config{
		ReadTimeout:  cfg.Runtime.HTTP.Timeout,
		WriteTimeout: cfg.Runtime.HTTP.Timeout,
		IdleTimeout:  cfg.Runtime.HTTP.IdleTimeout,
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
	}, log, cfg, redis)

	log.Info(
		"server listening",
		zap.String(logger.FieldServerAddress, cfg.Runtime.HTTP.Address),
	)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	listenConfig := fiber.ListenConfig{
		ShutdownTimeout:       cfg.Runtime.HTTP.ShutdownTimeout,
		DisableStartupMessage: true,
	}

	go func() {
		if err := app.Listen(cfg.Runtime.HTTP.Address, listenConfig); err != nil {
			log.Error("server listen failed", zap.Error(err))
			os.Exit(1)
		}
	}()

	<-done
	log.Info(
		"graceful shutdown requested",
		zap.String(logger.FieldServerAddress, cfg.Runtime.HTTP.Address),
	)

	if err := app.Shutdown(); err != nil {
		log.Error(
			"graceful shutdown failed; forcing stop",
			zap.Error(err),
			zap.Duration(logger.FieldShutdownTimeout, cfg.Runtime.HTTP.ShutdownTimeout),
		)

		return
	}

	snapReader.Stop()

	log.Info(
		"graceful shutdown completed",
		zap.String(logger.FieldServerAddress, cfg.Runtime.HTTP.Address),
	)
}

func connectContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
