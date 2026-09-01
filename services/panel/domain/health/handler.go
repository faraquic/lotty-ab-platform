package health

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/dto"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

type Handler struct {
	pool        *pgxpool.Pool
	redis       *rueidis.Client
	s3          *s3.Client
	environment string
	log         *zap.Logger
}

func NewHandler(pool *pgxpool.Pool, redis *rueidis.Client, s3 *s3.Client, environment string, log *zap.Logger) *Handler {
	return &Handler{pool, redis, s3, environment, log}
}

func (h *Handler) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/health", h.health)
	g.GET("/ready", h.ready)
}

func (h *Handler) health(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func (h *Handler) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	db := h.checkDatabase(ctx)
	cache := h.checkCache(ctx)
	storage := h.checkS3(ctx)

	if db.Status != dto.StatusOK {
		h.log.Warn(
			"readiness probe: database unavailable",
			zap.String(logger.FieldDBStatus, string(db.Status)),
			zap.String(logger.FieldDBMsg, db.Message),
		)
	}
	if cache.Status != dto.StatusOK {
		h.log.Warn(
			"readiness probe: cache unavailable",
			zap.String(logger.FieldCacheStatus, string(cache.Status)),
			zap.String(logger.FieldCacheMsg, cache.Message),
		)
	}
	if storage.Status != dto.StatusOK {
		h.log.Warn(
			"readiness probe: storage unavailable",
			zap.String(logger.FieldStorageStatus, string(storage.Status)),
			zap.String(logger.FieldStorageMsg, storage.Message),
		)
	}

	resp := dto.ReadyResponse{
		Service:     config.ServiceName,
		Version:     config.ServiceVersion,
		Environment: h.environment,
		Timestamp:   time.Now().UTC(),
		Components: map[string]dto.ComponentStatus{
			"service":  {Status: dto.StatusOK},
			"database": db,
			"cache":    cache,
			"storage":  storage,
		},
	}

	status := http.StatusOK
	if db.Status != dto.StatusOK {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, resp)
}

func (h *Handler) checkDatabase(ctx context.Context) dto.ComponentStatus {
	if h.pool == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: "database pool not initialized"}
	}

	if err := h.pool.Ping(ctx); err != nil {
		h.log.Error("health check: database ping failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: err.Error()}
	}

	return dto.ComponentStatus{Status: dto.StatusOK}
}

func (h *Handler) checkCache(ctx context.Context) dto.ComponentStatus {
	if h.redis == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: "redis not connected"}
	}

	if err := (*h.redis).Do(ctx, (*h.redis).B().Ping().Build()).Error(); err != nil {
		h.log.Warn("health check: redis ping failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: err.Error()}
	}

	return dto.ComponentStatus{Status: dto.StatusOK}
}

func (h *Handler) checkS3(ctx context.Context) dto.ComponentStatus {
	if h.s3 == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: "s3 not connected"}
	}

	if _, err := h.s3.ListBuckets(ctx, &s3.ListBucketsInput{}); err != nil {
		h.log.Warn("health check: s3 list buckets failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: err.Error()}
	}

	return dto.ComponentStatus{Status: dto.StatusOK}
}
