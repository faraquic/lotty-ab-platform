package decide

import (
	"net/http"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log.Named("decide")}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/decide", h.decide)
}

func (h *Handler) decide(c fiber.Ctx) error {
	var req CreateDecisionRequest
	if err := api.ValidateRequestFiber(c, &req); err != nil {
		return nil
	}

	resp, err := h.svc.Decide(req, middleware.GetRequestIDFiber(c))
	if err != nil {
		if err == ErrSnapshotUnavailable {
			return api.ErrorFiber(c, http.StatusServiceUnavailable, api.SnapshotUnavailable, "snapshot not ready")
		}
		h.log.Error("decide failed", zap.Error(err))
		return api.InternalErrorFiber(c)
	}

	return api.OKFiber(c, resp)
}
