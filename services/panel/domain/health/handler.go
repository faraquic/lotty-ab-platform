package health

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/dto"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

type Handler struct {
	pool        *pgxpool.Pool
	redis       *rueidis.Client
	s3          *s3.Client
	snapshot    *snapshot.Reader
	environment string
	log         *zap.Logger
}

func NewHandler(pool *pgxpool.Pool, redis *rueidis.Client, s3 *s3.Client, snapshotReader *snapshot.Reader, environment string, log *zap.Logger) *Handler {
	return &Handler{pool, redis, s3, snapshotReader, environment, log}
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
	snap := h.checkSnapshot(ctx)
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
	if snap.Status != dto.StatusOK {
		h.log.Warn(
			"readiness probe: snapshot unavailable",
			zap.String(logger.FieldSnapshotStatus, string(snap.Status)),
			zap.String(logger.FieldSnapshotMsg, snap.Message),
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
			"service":  {Status: dto.StatusOK, Criticality: dto.CriticalityRequired},
			"database": db,
			"cache":    cache,
			"snapshot": snap,
			"storage":  storage,
		},
	}

	status := http.StatusOK
	if db.Status != dto.StatusOK || cache.Status != dto.StatusOK || snap.Status != dto.StatusOK {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, resp)
}

func (h *Handler) checkDatabase(ctx context.Context) dto.ComponentStatus {
	if h.pool == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "database pool not initialized"}
	}

	if err := h.pool.Ping(ctx); err != nil {
		h.log.Error("health check: database ping failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "database unavailable"}
	}

	return dto.ComponentStatus{Status: dto.StatusOK, Criticality: dto.CriticalityRequired}
}

func (h *Handler) checkCache(ctx context.Context) dto.ComponentStatus {
	if h.redis == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "redis not connected"}
	}

	if err := (*h.redis).Do(ctx, (*h.redis).B().Ping().Build()).Error(); err != nil {
		h.log.Warn("health check: redis ping failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "cache unavailable"}
	}

	return dto.ComponentStatus{Status: dto.StatusOK, Criticality: dto.CriticalityRequired}
}

func (h *Handler) checkSnapshot(ctx context.Context) dto.ComponentStatus {
	if h.snapshot == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "snapshot storage not initialized"}
	}

	exists, err := h.snapshot.Exists(ctx)
	if err != nil {
		h.log.Warn("health check: snapshot exists failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "snapshot unavailable"}
	}
	if !exists {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityRequired, Message: "snapshot not found"}
	}

	return dto.ComponentStatus{Status: dto.StatusOK, Criticality: dto.CriticalityRequired}
}

func (h *Handler) checkS3(ctx context.Context) dto.ComponentStatus {
	if h.s3 == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityOptional, Message: "s3 not connected"}
	}

	if _, err := h.s3.ListBuckets(ctx, &s3.ListBucketsInput{}); err != nil {
		h.log.Warn("health check: s3 list buckets failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Criticality: dto.CriticalityOptional, Message: "storage unavailable"}
	}

	return dto.ComponentStatus{Status: dto.StatusOK, Criticality: dto.CriticalityOptional}
}
