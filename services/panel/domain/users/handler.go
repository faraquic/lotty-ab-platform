package users

import (
	"errors"
	"net/http"
	"strconv"

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
	users := rg.Group("/users")
	{
		users.POST("", h.create)
		users.GET("", h.list)
		users.GET("/:id", h.getByID)
		users.PATCH("/:id", h.update)
		users.DELETE("/:id", h.delete)
		users.POST("/:id/avatar", h.uploadAvatar)
	}
}

func (h *Handler) RegisterMeRoute(rg *gin.RouterGroup) {
	rg.GET("/me", h.me)
	rg.POST("/me/avatar", h.uploadMyAvatar)
}

func (h *Handler) me(c *gin.Context) {
	resp, err := h.svc.GetByID(c.Request.Context(), middleware.CallerIDGin(c))
	if err != nil {
		h.respondError(c, err)
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

	resp, err := h.svc.List(c.Request.Context(), limit, offset)
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

	var req UpdateUserRequest
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

func (h *Handler) uploadAvatar(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	h.handleAvatarUpload(c, id)
}

func (h *Handler) uploadMyAvatar(c *gin.Context) {
	h.handleAvatarUpload(c, middleware.CallerIDGin(c))
}

func (h *Handler) handleAvatarUpload(c *gin.Context, userID string) {
	file, err := c.FormFile("avatar")
	if err != nil || file.Size == 0 {
		resp, err := h.svc.DeleteAvatar(c.Request.Context(), userID)
		if err != nil {
			h.respondError(c, err)
			return
		}
		api.OK(c.Writer, resp)
		return
	}

	resp, err := h.svc.UploadAvatar(c.Request.Context(), userID, file)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func parseID(c *gin.Context) (string, bool) {
	idStr := c.Param("id")
	_, err := uuid.Parse(idStr)
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid user id")
		return "", false
	}

	return idStr, true
}

func (h *Handler) respondError(c *gin.Context, err error) {
	w := c.Writer
	switch {
	case errors.Is(err, ErrNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, err.Error())
	case errors.Is(err, ErrConflict):
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrInvalidRole):
		api.Error(w, http.StatusBadRequest, api.BadRequest, err.Error())
	case errors.Is(err, ErrSelfDelete), errors.Is(err, ErrSelfRoleChange):
		logger.SetErrorType(c, logger.ErrorTypeAuthorizationDenied)
		api.Error(w, http.StatusForbidden, api.Forbidden, err.Error())
	case errors.Is(err, ErrLastAdmin):
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrInvalidFileType), errors.Is(err, ErrFileTooLarge):
		api.Error(w, http.StatusBadRequest, api.BadRequest, err.Error())
	case errors.Is(err, ErrStorageUnavailable):
		logger.SetErrorType(c, logger.ErrorTypeStorageUnavailable)
		api.Error(w, http.StatusServiceUnavailable, api.ServiceUnavailable, err.Error())
	default:
		logger.SetErrorType(c, logger.ErrorTypeInternalError)
		h.log.Error("unexpected error",
			zap.Error(err),
			zap.String(logger.FieldErrorType, "internal_error"),
		)
		api.InternalError(w)
	}
}
