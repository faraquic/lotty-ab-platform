package health

import (
	"context"
	"net/http"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
)

type Handler struct {
	pool        *pgxpool.Pool
	redis       *rueidis.Client
	environment string
}

func NewHandler(pool *pgxpool.Pool, redis *rueidis.Client, environment string) *Handler {
	return &Handler{
		pool:        pool,
		redis:       redis,
		environment: environment,
	}
}

func (h *Handler) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/health", h.health)
	g.GET("/ready", h.ready)
}

// health is a liveness probe: the process is up. It performs no
// downstream checks so it stays green even during dependency outages.
func (h *Handler) health(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// ready is a readiness probe: the service can serve traffic. A failed
// database check makes the service not ready (503); Redis is optional,
// so its outage is reported but does not fail readiness.
func (h *Handler) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	db := h.checkDatabase(ctx)
	cache := h.checkCache(ctx)

	resp := ReadyResponse{
		Service:     config.ServiceName,
		Version:     config.ServiceVersion,
		Environment: h.environment,
		Timestamp:   time.Now().UTC(),
		Components: map[string]ComponentStatus{
			"service":  {Status: StatusOK},
			"database": db,
			"cache":    cache,
		},
	}

	status := http.StatusOK
	if db.Status != StatusOK {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, resp)
}

func (h *Handler) checkDatabase(ctx context.Context) ComponentStatus {
	if h.pool == nil {
		return ComponentStatus{Status: StatusUnavailable, Message: "database pool not initialized"}
	}

	if err := h.pool.Ping(ctx); err != nil {
		return ComponentStatus{Status: StatusUnavailable, Message: err.Error()}
	}

	return ComponentStatus{Status: StatusOK}
}

func (h *Handler) checkCache(ctx context.Context) ComponentStatus {
	if h.redis == nil {
		return ComponentStatus{Status: StatusUnavailable, Message: "redis not connected"}
	}

	if err := (*h.redis).Do(ctx, (*h.redis).B().Ping().Build()).Error(); err != nil {
		return ComponentStatus{Status: StatusUnavailable, Message: err.Error()}
	}

	return ComponentStatus{Status: StatusOK}
}
