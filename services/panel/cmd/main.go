package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
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

	validateSecurity(cfg, log)

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
		log,
	)
	if err != nil {
		log.Warn(
			"redis is unavailable, continuing without it",
			zap.Error(err),
		)
	} else {
		defer (*redis).Close()
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
		log.Warn(
			"s3 is unavailable, continuing without it",
			zap.Error(err),
		)
	}

	setGinMode(cfg.Environment, log)
	r := newRouter(log, cfg, postgres, redis, s3)

	log.Info("starting server", zap.String("address", cfg.Panel.HTTP.Address))

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
			log.Error("failed to start server", zap.Error(err))
			os.Exit(1)
		}
	}()

	log.Info("server started")

	<-done
	log.Info("stopping server")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.Panel.HTTP.ShutdownTimeout)
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

// validateSecurity fails fast on insecure production settings
// and warns about risky ones in any environment.
func validateSecurity(cfg *config.Config, log *zap.Logger) {
	secret := cfg.Auth.JWT.SecretKey

	if cfg.Environment == "prod" && insecureSecret(secret) {
		log.Error(
			"insecure JWT secret: set auth.jwt.secret_key in config or via ENV template before running in prod",
		)
		os.Exit(1)
	}

	if insecureSecret(secret) {
		log.Warn("insecure JWT secret in use; acceptable only for local development")
	}

	if slices.Contains(cfg.Panel.HTTP.CORS.AllowedOrigins, "*") {
		log.Warn("CORS allowed_origins contains '*': any site can call the API; restrict it in production")
	}
}

func insecureSecret(secret string) bool {
	return secret == "" || strings.HasPrefix(secret, "change-me")
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
