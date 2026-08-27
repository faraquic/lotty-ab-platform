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
	authdomain "github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/auth"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/health"
	usersdomain "github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
)

func newRouter(log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool, redisClient *rueidis.Client, s3Client *s3.Client) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true

	addMiddleware(r, cfg, log)

	apiV1 := r.Group("/api/panel/v1")

	healthdomain.NewHandler(pool, redisClient, s3Client, cfg.Environment).RegisterRoutes(apiV1)

	tokenizer := libauth.NewJWTManager(cfg.Auth.JWT.SecretKey, cfg.Auth.JWT.TTL)
	authRepo := authdomain.NewRepository(pool)
	authSvc := authdomain.NewService(authRepo, tokenizer, redisClient, cfg.Auth.JWT.TTL, log)
	authHandler := authdomain.NewHandler(authSvc, log)
	authHandler.RegisterRoutes(apiV1)

	repo := usersdomain.NewRepository(pool)
	svc := usersdomain.NewService(repo, authSvc, log)
	usersHandler := usersdomain.NewHandler(svc, log)

	bootstrapAdmin(log, svc, cfg.Auth.Bootstrap)

	authMW, err := authdomain.NewMiddleware(cfg, authSvc, log)
	if err != nil {
		log.Fatal("failed to init auth middleware", zap.Error(err))
	}

	anyAuthGroup := apiV1.Group("", authMW.Handler(nil))
	usersHandler.RegisterMeRoute(anyAuthGroup)

	adminGroup := apiV1.Group("", authMW.Handler([]usersdomain.Role{usersdomain.RoleAdmin}))
	usersHandler.RegisterRoutes(adminGroup)

	return r
}

func bootstrapAdmin(log *zap.Logger, svc *usersdomain.Service, cfg config.BootstrapConfig) {
	if cfg.Password == "" {
		log.Warn("auth.bootstrap.password is not set; cannot create the first admin automatically (fill it in config or via ENV template)")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := svc.EnsureBootstrapAdmin(ctx, cfg.Username, cfg.Email, cfg.Password)
	if err != nil {
		log.Error("bootstrap admin failed", zap.Error(err))
		return
	}

	if !created {
		log.Debug("users table is not empty; bootstrap skipped")
	}
}

func addMiddleware(r *gin.Engine, cfg *config.Config, log *zap.Logger) {
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
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
