package main

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	authdomain "github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/auth"
	usersdomain "github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
	libauth "github.com/faraquic/lotty-ab-platform/services/panel/internal/lib/auth"
)

func newRouter(log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool, redisClient *redis.Client) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true

	addMiddleware(r, cfg, log)

	apiV1 := r.Group("/api/v1")

	repo := usersdomain.NewRepository(pool)
	svc := usersdomain.NewService(repo, log)
	usersHandler := usersdomain.NewHandler(svc, log)

	tokenizer := libauth.NewJWTManager(cfg.Auth.JWT.SecretKey, cfg.Auth.JWT.TTL)
	redisTTL := cfg.Auth.JWT.RedisTTL
	if redisTTL <= 0 {
		redisTTL = cfg.Auth.JWT.TTL
	}
	authRepo := authdomain.NewRepository(pool)
	authSvc := authdomain.NewService(authRepo, tokenizer, redisClient, cfg.Auth.JWT.TTL, redisTTL, log)
	authHandler := authdomain.NewHandler(authSvc, log)
	authHandler.RegisterRoutes(apiV1)

	authMW, err := authdomain.NewMiddleware(cfg, authSvc, log)
	if err != nil {
		log.Fatal("failed to init auth middleware", zap.Error(err))
	}

	protected := apiV1.Group("", authMW.Handler())
	usersHandler.RegisterRoutes(protected)

	return r
}

func addMiddleware(r *gin.Engine, cfg *config.Config, log *zap.Logger) {
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(corsMiddleware(cfg.Panel.HTTPConfig.CORS))
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

	if len(corsCfg.AllowOrigins) == 0 {
		corsCfg.AllowOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
	if len(corsCfg.AllowMethods) == 0 {
		corsCfg.AllowMethods = []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	}
	if corsCfg.MaxAge == 0 {
		corsCfg.MaxAge = 12 * time.Hour
	}

	return cors.New(corsCfg)
}
