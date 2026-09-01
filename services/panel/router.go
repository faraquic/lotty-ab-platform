package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
	"go.uber.org/zap"

	libauth "github.com/faraquic/lotty-ab-platform/pkg/auth"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	authdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/auth"
	flagsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/flags"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/health"
	metricsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/metrics"
	usersdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

func newRouter(log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool, redisClient *rueidis.Client, s3Client *s3.Client) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true

	addMiddleware(r, cfg, log)

	apiV1 := r.Group("/api/v1/panel")

	healthdomain.NewHandler(pool, redisClient, s3Client, cfg.Environment, log).RegisterRoutes(apiV1)

	tokenizer := libauth.NewJWTManager(cfg.Auth.JWT.SecretKey, cfg.Auth.JWT.TTL)
	authRepo := authdomain.NewRepository(pool)
	authSvc := authdomain.NewService(authRepo, tokenizer, redisClient, cfg.Auth.JWT.TTL, log)
	authHandler := authdomain.NewHandler(authSvc, log)
	authHandler.RegisterRoutes(apiV1)

	repo := usersdomain.NewRepository(pool)
	var storage usersdomain.Storage
	if s3Client != nil {
		storage = usersdomain.NewS3Storage(s3Client, cfg.Database.S3.Bucket, cfg.Database.S3.Endpoint)
	}
	svc := usersdomain.NewService(repo, authSvc, storage, log)
	usersHandler := usersdomain.NewHandler(svc, log)

	bootstrapAdmin(log, svc, cfg.Auth.Bootstrap)

	flagsRepo := flagsdomain.NewRepository(pool)
	flagsSvc := flagsdomain.NewService(flagsRepo, log)
	flagsHandler := flagsdomain.NewHandler(flagsSvc, log)

	authMW, err := authdomain.NewMiddleware(cfg, authSvc, log)
	if err != nil {
		log.Fatal("auth middleware init failed", zap.Error(err))
	}

	anyAuthGroup := apiV1.Group("", authMW.Handler(nil))
	usersHandler.RegisterMeRoute(anyAuthGroup)

	adminGroup := apiV1.Group("", authMW.Handler([]usersdomain.Role{usersdomain.RoleAdmin}))
	usersHandler.RegisterRoutes(adminGroup)

	flagsHandler.RegisterRoutes(anyAuthGroup)

	metricsRepo := metricsdomain.NewRepository(pool)
	metricsSvc := metricsdomain.NewService(metricsRepo, log)
	metricsHandler := metricsdomain.NewHandler(metricsSvc, log)
	metricsHandler.RegisterRoutes(anyAuthGroup)

	return r
}

func bootstrapAdmin(log *zap.Logger, svc *usersdomain.Service, cfg config.BootstrapConfig) {
	if cfg.PasswordHash == "" {
		log.Warn("auth.bootstrap.password_hash is not set; cannot create the first admin automatically")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := svc.EnsureBootstrapAdmin(ctx, cfg.Username, cfg.Email, cfg.PasswordHash)
	if err != nil {
		log.Error("bootstrap admin creation failed", zap.Error(err))
		return
	}

	if !created {
		log.Info("users table is not empty; bootstrap skipped")
	}
}

func addMiddleware(r *gin.Engine, cfg *config.Config, log *zap.Logger) {
	r.Use(middleware.RecoveryGin(log))
	r.Use(middleware.RequestIDGin())
	r.Use(middleware.LoggerGin(log))
	r.Use(corsMiddleware(cfg.Panel.HTTP.CORS))
}

func corsMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	corsCfg := cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		ExposeHeaders:    cfg.ExposeHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	}

	return cors.New(corsCfg)
}
