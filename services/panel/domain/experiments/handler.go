package experiments

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	flagsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/flags"
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
	experiments := rg.Group("/experiments")
	{
		experiments.GET("", h.list)
		experiments.GET("/:id", h.getByID)
	}
}

func (h *Handler) RegisterWriteRoutes(rg *gin.RouterGroup) {
	experiments := rg.Group("/experiments")
	{
		experiments.POST("", h.create)
		experiments.PATCH("/:id", h.updateDraft)
		experiments.POST("/:id/versions", h.createVersion)
		experiments.PUT("/:id/variants", h.setVariants)
		experiments.POST("/:id/submit", h.submit)
		experiments.POST("/:id/start", h.start)
		experiments.POST("/:id/pause", h.pause)
		experiments.POST("/:id/resume", h.resume)
		experiments.POST("/:id/complete", h.complete)
		experiments.POST("/:id/rollout", h.rollout)
		experiments.POST("/:id/archive", h.archive)
	}
}

func (h *Handler) RegisterInternalRoutes(rg *gin.RouterGroup) {
	internal := rg.Group("/internal/experiments")
	{
		internal.POST("/:id/pause", h.internalPause)
		internal.POST("/:id/rollback", h.internalRollback)
	}
}

func (h *Handler) create(c *gin.Context) {
	var req CreateExperimentRequest
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

	var status Status
	if raw := c.Query("status"); raw != "" {
		status = Status(raw)
		if !status.Valid() {
			api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid status filter")
			return
		}
	}

	resp, err := h.svc.List(c.Request.Context(), limit, offset, status)
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

func (h *Handler) updateDraft(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req UpdateDraftRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.UpdateDraft(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) createVersion(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req CreateVersionRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.CreateVersion(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) setVariants(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req SetVariantsRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.SetVariants(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) transition(c *gin.Context, fn func(ctx context.Context, callerID, id string, req TransitionRequest) (ExperimentResponse, error)) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req TransitionRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := fn(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) submit(c *gin.Context) {
	h.transition(c, h.svc.Submit)
}

func (h *Handler) start(c *gin.Context) {
	h.transition(c, h.svc.Start)
}

func (h *Handler) pause(c *gin.Context) {
	h.transition(c, h.svc.Pause)
}

func (h *Handler) resume(c *gin.Context) {
	h.transition(c, h.svc.Resume)
}

func (h *Handler) archive(c *gin.Context) {
	h.transition(c, h.svc.Archive)
}

func (h *Handler) internalPause(c *gin.Context) {
	h.internalTransition(c, h.svc.InternalPause)
}

func (h *Handler) internalRollback(c *gin.Context) {
	h.internalTransition(c, h.svc.InternalRollback)
}

func (h *Handler) internalTransition(c *gin.Context, fn func(ctx context.Context, callerID, id string, version int) (ExperimentResponse, error)) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req TransitionRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := fn(c.Request.Context(), middleware.CallerIDGin(c), id, req.Version)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) complete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req CompleteRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Complete(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) rollout(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req RolloutRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Rollout(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func parseID(c *gin.Context) (string, bool) {
	idStr := c.Param("id")
	if _, err := uuid.Parse(idStr); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid experiment id")
		return "", false
	}

	return idStr, true
}

func (h *Handler) respondError(c *gin.Context, err error) {
	w := c.Writer
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrVersionNotFound), errors.Is(err, ErrFlagNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, err.Error())
	case errors.Is(err, ErrConflictName), errors.Is(err, ErrFlagBusy):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrVersionConflict), errors.Is(err, ErrVersionNotApproved):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrForbidden):
		logger.SetErrorType(c, logger.ErrorTypeAuthorizationDenied)
		api.Error(w, http.StatusForbidden, api.Forbidden, err.Error())
	case errors.Is(err, ErrInvalidVariants), errors.Is(err, ErrInvalidTargeting),
		errors.Is(err, ErrInvalidWeights), errors.Is(err, ErrVersionNotReady),
		errors.Is(err, ErrReasonRequired), errors.Is(err, ErrWinnerRequired),
		errors.Is(err, ErrVersionImmutable), errors.Is(err, flagsdomain.ErrInvalidValue):
		api.Error(w, http.StatusUnprocessableEntity, api.UnprocessableEntity, err.Error())
	default:
		logger.SetErrorType(c, logger.ErrorTypeInternalError)
		h.log.Error("unexpected error",
			zap.Error(err),
			zap.String(logger.FieldErrorType, "internal_error"),
		)
		api.InternalError(w)
	}
}
