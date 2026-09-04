package metrics

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc, log}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	metrics := rg.Group("/metrics")
	{
		metrics.POST("", h.create)
		metrics.GET("", h.list)
		metrics.GET("/:id", h.getByID)
		metrics.PATCH("/:id", h.update)
	}
}

func (h *Handler) create(c *gin.Context) {
	var req CreateMetricRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Create(c.Request.Context(), middleware.CallerIDGin(c), req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) list(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}
	includeArchived := strings.EqualFold(c.Query("archived"), "true")

	resp, err := h.svc.List(c.Request.Context(), limit, offset, includeArchived)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) getByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	includeArchived := strings.EqualFold(c.Query("archived"), "true")

	resp, err := h.svc.GetByID(c.Request.Context(), id, includeArchived)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req UpdateMetricRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Update(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func parseID(c *gin.Context) (string, bool) {
	idStr := c.Param("id")
	if _, err := uuid.Parse(idStr); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid metric id")
		return "", false
	}

	return idStr, true
}

func (h *Handler) respondError(c *gin.Context, err error) {
	w := c.Writer
	switch {
	case errors.Is(err, ErrNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, err.Error())
	case errors.Is(err, ErrConflictKeys):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrConflictNames):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrInvalidMetricConfig), errors.Is(err, ErrInvalidMetricType):
		api.Error(w, http.StatusBadRequest, api.BadRequest, err.Error())
	case errors.Is(err, ErrBuiltinProtected), errors.Is(err, ErrBuiltinDelete):
		logger.SetErrorType(c, logger.ErrorTypeAuthorizationDenied)
		api.Error(w, http.StatusForbidden, api.Forbidden, err.Error())
	default:
		logger.SetErrorType(c, logger.ErrorTypeInternalError)
		h.log.Error("unexpected error",
			zap.Error(err),
			zap.String(logger.FieldErrorType, "internal_error"),
		)
		api.InternalError(w)
	}
}
