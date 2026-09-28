package events

import (
	"errors"
	"net/http"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log.Named("events")}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/events/batch", h.ingestEvents)
	r.Post("/exposures/batch", h.ingestExposures)
	r.Get("/event-types", h.listEventTypes)
}

func (h *Handler) ingestEvents(c fiber.Ctx) error {
	var req EventBatchRequest
	if err := api.ValidateRequestFiber(c, &req); err != nil {
		return nil
	}

	res, err := h.svc.IngestEvents(c.Context(), req)
	if err != nil {
		return h.ingestError(c, err)
	}

	return h.batchResponse(c, res)
}

func (h *Handler) ingestExposures(c fiber.Ctx) error {
	var req ExposureBatchRequest
	if err := api.ValidateRequestFiber(c, &req); err != nil {
		return nil
	}

	res, err := h.svc.IngestExposures(c.Context(), req)
	if err != nil {
		return h.ingestError(c, err)
	}

	return h.batchResponse(c, res)
}

func (h *Handler) listEventTypes(c fiber.Ctx) error {
	types, err := h.svc.ListEventTypes(c.Context())
	if err != nil {
		h.log.Error("list event types failed", zap.Error(err))
		return api.InternalErrorFiber(c)
	}

	out := make([]EventTypeResponse, 0, len(types))
	for _, t := range types {
		out = append(out, EventTypeResponse{
			Key:             t.Key,
			Name:            t.Name,
			Description:     t.Description,
			SchemaVersion:   t.SchemaVersion,
			RequireExposure: t.RequireExposure,
			Status:          t.Status,
		})
	}

	return api.OKFiber(c, out)
}

func (h *Handler) batchResponse(c fiber.Ctx, res BatchData) error {
	switch {
	case res.Rejected > 0 && res.Accepted+res.Duplicates == 0:
		return c.Status(http.StatusBadRequest).JSON(api.Response{Success: false, Data: res, Error: &api.ErrorInfo{Code: api.BadRequest, Message: "all items rejected"}})
	case res.Rejected > 0:
		return c.Status(http.StatusMultiStatus).JSON(api.Response{Success: true, Data: res})
	default:
		return c.Status(http.StatusAccepted).JSON(api.Response{Success: true, Data: res})
	}
}

func (h *Handler) ingestError(c fiber.Ctx, err error) error {
	if errors.Is(err, ErrBrokerUnavailable) {
		h.log.Warn("events broker unavailable", zap.Error(err))
		return api.ErrorFiber(c, http.StatusServiceUnavailable, api.ServiceUnavailable, "event broker unavailable")
	}

	h.log.Error(
		"events ingest failed",
		zap.String(logger.FieldRoute, c.Route().Path),
		zap.Error(err),
	)

	return api.InternalErrorFiber(c)
}
