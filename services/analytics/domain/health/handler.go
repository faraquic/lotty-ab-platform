package health

import (
	"net/http"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/dto"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type Handler struct {
	environment string
	log         *zap.Logger
}

func NewHandler(environment string, log *zap.Logger) *Handler {
	return &Handler{environment, log}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)
}

func (h *Handler) health(c fiber.Ctx) {
	c.Status(http.StatusOK).SendString("OK")
}

func (h *Handler) ready(c fiber.Ctx) {
	resp := dto.ReadyResponse{
		Service:     config.ServiceName,
		Version:     config.ServiceVersion,
		Environment: h.environment,
		Timestamp:   time.Now().UTC(),
		Components: map[string]dto.ComponentStatus{
			"service": {Status: dto.StatusOK},
		},
	}

	c.JSON(resp)
}
