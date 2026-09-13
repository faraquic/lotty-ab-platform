package health

import (
	"context"
	"net/http"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/dto"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

type Handler struct {
	redis       *rueidis.Client
	snapshot    *snapshot.Reader
	environment string
	log         *zap.Logger
}

func NewHandler(redis *rueidis.Client, snapshot *snapshot.Reader, environment string, log *zap.Logger) *Handler {
	return &Handler{redis, snapshot, environment, log}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)
}

func (h *Handler) health(c fiber.Ctx) {
	c.Status(http.StatusOK).SendString("OK")
}

func (h *Handler) ready(c fiber.Ctx) {
	ctx, cancel := context.WithTimeout(c.RequestCtx(), 3*time.Second)
	defer cancel()

	cache := h.checkCache(ctx)
	snap := h.checkSnapshot(ctx)

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
	resp := dto.ReadyResponse{
		Service:     config.ServiceName,
		Version:     config.ServiceVersion,
		Environment: h.environment,
		Timestamp:   time.Now().UTC(),
		Components: map[string]dto.ComponentStatus{
			"service":  {Status: dto.StatusOK},
			"cache":    cache,
			"snapshot": snap,
		},
	}

	c.JSON(resp)
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

func (h *Handler) checkSnapshot(ctx context.Context) dto.ComponentStatus {
	if h.snapshot == nil {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: "snapshot storage not initialized"}
	}

	exists, err := h.snapshot.Exists(ctx)
	if err != nil {
		h.log.Warn("health check: snapshot exists failed", zap.Error(err))
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: err.Error()}
	}
	if !exists {
		return dto.ComponentStatus{Status: dto.StatusUnavailable, Message: "snapshot not found"}
	}

	return dto.ComponentStatus{Status: dto.StatusOK}
}
