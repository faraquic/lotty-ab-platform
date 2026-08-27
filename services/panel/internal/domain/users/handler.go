package users

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
)

type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	{
		users.POST("", h.create)
		users.GET("", h.list)
		users.GET("/:id", h.getByID)
		users.PATCH("/:id", h.update)
		users.DELETE("/:id", h.delete)
	}
}

func (h *Handler) RegisterMeRoute(rg *gin.RouterGroup) {
	rg.GET("/me", h.me)
}

func (h *Handler) me(c *gin.Context) {
	resp, err := h.svc.GetByID(c.Request.Context(), callerID(c))
	if err != nil {
		h.respondError(c.Writer, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) create(c *gin.Context) {
	var req CreateUserRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		h.respondError(c.Writer, err)
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

	resp, err := h.svc.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.respondError(c.Writer, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) getByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	resp, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		h.respondError(c.Writer, err)
		return
	}

	api.OK(c.Writer, resp)
}

// CtxUserIDKey is set by the auth middleware after token verification;
// handlers read the caller's id from it.
const CtxUserIDKey = "user_id"

func callerID(c *gin.Context) int64 {
	id, _ := c.Get(CtxUserIDKey)

	v, _ := id.(int64)

	return v
}

func (h *Handler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req UpdateUserRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Update(c.Request.Context(), callerID(c), id, req)
	if err != nil {
		h.respondError(c.Writer, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), callerID(c), id); err != nil {
		h.respondError(c.Writer, err)
		return
	}

	c.Writer.WriteHeader(http.StatusNoContent)
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid user id")
		return 0, false
	}

	return id, true
}

func (h *Handler) respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, err.Error())
	case errors.Is(err, ErrConflict):
		api.Error(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, ErrInvalidRole):
		api.Error(w, http.StatusBadRequest, api.BadRequest, err.Error())
	case errors.Is(err, ErrSelfDelete), errors.Is(err, ErrSelfRoleChange):
		api.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, ErrLastAdmin):
		api.Error(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		h.log.Error("internal error", zap.Error(err))
		api.InternalError(w)
	}
}
