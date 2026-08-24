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
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad()
	log := logger.SetupLogger(cfg.Environment)

	log.Info("starting Panel Lotty AB Platform", zap.String("environment", cfg.Environment))
	log.Debug("debug messages are enabled")

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
		log.Error("failed to connect to postgres", zap.Error(err))
		os.Exit(1)
	}
	defer postgres.Close()

	redis, err := database.NewRedis(
		connectCtx,
		cfg.Database.Redis.Address,
		cfg.Database.Redis.Password,
		cfg.Database.Redis.DB,
		log,
	)
	if err != nil {
		log.Warn("redis is unavailable, continuing without it",
			zap.Error(err),
		)
	} else {
		defer redis.Close()
	}

	setGinMode(cfg.Environment, log)
	r := newRouter(log, cfg, postgres, redis)

	log.Info("starting server", zap.String("address", cfg.Panel.HTTPConfig.Address))

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:              cfg.Panel.HTTPConfig.Address,
		Handler:           r,
		ReadTimeout:       cfg.Panel.HTTPConfig.Timeout,
		ReadHeaderTimeout: cfg.Panel.HTTPConfig.Timeout,
		WriteTimeout:      cfg.Panel.HTTPConfig.Timeout,
		IdleTimeout:       cfg.Panel.HTTPConfig.IdleTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("failed to start server", zap.Error(err))
			os.Exit(1)
		}
	}()

	log.Info("server started")

	<-done
	log.Info("stopping server")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.Panel.HTTPConfig.ShutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown server", zap.Error(err))

		return
	}

	log.Info("server stopped")
}

func connectContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func setGinMode(environment string, log *zap.Logger) {
	switch environment {
	case "prod":
		gin.SetMode(gin.ReleaseMode)
	case "dev", "local":
		gin.SetMode(gin.DebugMode)
	default:
		log.Warn("unknown environment, defaulting to release mode", zap.String("environment", environment))
		gin.SetMode(gin.ReleaseMode)
	}
}
