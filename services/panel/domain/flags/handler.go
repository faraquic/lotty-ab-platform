package flags

import (
	"errors"
	"net/http"

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
	flags := rg.Group("/flags")
	{
		flags.GET("", h.list)
		flags.GET("/:id", h.getByID)
	}
}

func (h *Handler) RegisterWriteRoutes(rg *gin.RouterGroup) {
	flags := rg.Group("/flags")
	{
		flags.POST("", h.create)
		flags.PATCH("/:id", h.update)
		flags.DELETE("/:id", h.delete)
	}
}

func (h *Handler) create(c *gin.Context) {
	var req CreateFlagRequest
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

type listQuery struct {
	api.ListQuery
	Type      string `form:"type"`
	CreatedBy string `form:"created_by"`
}

func (h *Handler) list(c *gin.Context) {
	var q listQuery
	if !api.BindListQuery(c, &q) {
		return
	}
	q.Normalize()

	filter := ListFilter{
		Search: q.Search(),
		Sort:   q.Sort,
		Order:  q.Order,
	}

	if q.Type != "" {
		t := TypeFlag(q.Type)
		if !t.Valid() {
			api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid type filter")
			return
		}
		filter.Type = &t
	}

	if q.CreatedBy != "" {
		if _, err := uuid.Parse(q.CreatedBy); err != nil {
			api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid created_by filter")
			return
		}
		filter.CreatedBy = &q.CreatedBy
	}

	resp, err := h.svc.List(c.Request.Context(), q.Limit, q.Offset, filter)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OKWithMeta(c.Writer, resp.Data, &resp.Meta)
}

func (h *Handler) getByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	resp, err := h.svc.GetByID(c.Request.Context(), id)
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

	var req UpdateFlagRequest
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

func (h *Handler) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), middleware.CallerIDGin(c), id); err != nil {
		h.respondError(c, err)
		return
	}

	c.Writer.WriteHeader(http.StatusNoContent)
}

func parseID(c *gin.Context) (string, bool) {
	idStr := c.Param("id")
	if _, err := uuid.Parse(idStr); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid flag id")
		return "", false
	}

	return idStr, true
}

func (h *Handler) respondError(c *gin.Context, err error) {
	w := c.Writer
	switch {
	case errors.Is(err, ErrNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, err.Error())
	case errors.Is(err, ErrActiveExperiment), errors.Is(err, ErrConflictKeys):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrConflictNames):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrInvalidTypeFlag), errors.Is(err, ErrInvalidValue):
		api.Error(w, http.StatusBadRequest, api.BadRequest, err.Error())
	default:
		logger.SetErrorType(c, logger.ErrorTypeInternalError)
		h.log.Error(
			"unexpected error",
			zap.Error(err),
			zap.String(logger.FieldErrorType, "internal_error"),
		)
		api.InternalError(w)
	}
}
