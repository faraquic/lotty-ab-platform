package reviews

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

func (h *Handler) RegisterGroupRoutes(rg *gin.RouterGroup) {
	groups := rg.Group("/approver-groups")
	{
		groups.POST("", h.createGroup)
		groups.GET("", h.listGroups)
		groups.GET("/:id", h.getGroup)
		groups.PATCH("/:id", h.updateGroup)
		groups.POST("/:id/members", h.addMember)
		groups.DELETE("/:id/members/:user_id", h.removeMember)
	}

	exp := rg.Group("/experimenters")
	{
		exp.PUT("/:id/group", h.setExperimenterGroup)
	}
}

func (h *Handler) RegisterReviewRoutes(rg *gin.RouterGroup) {
	reviews := rg.Group("/reviews")
	{
		reviews.GET("", h.listReviews)
		reviews.GET("/:id", h.getReview)
		reviews.POST("/:id/approvals", h.actOnReview)
		reviews.POST("/:id/comments", h.addComment)
		reviews.PATCH("/comments/:cid/resolve", h.resolveComment)
		reviews.DELETE("/comments/:cid", h.deleteComment)
	}
}

func (h *Handler) createGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.CreateGroup(c.Request.Context(), middleware.CallerIDGin(c), req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) listGroups(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.svc.ListGroups(c.Request.Context(), limit, offset, true)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OKWithMeta(c.Writer, resp.Data, &resp.Meta)
}

func (h *Handler) getGroup(c *gin.Context) {
	id, ok := parseID(c, "group")
	if !ok {
		return
	}

	resp, err := h.svc.GetGroup(c.Request.Context(), id, true)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) updateGroup(c *gin.Context) {
	id, ok := parseID(c, "group")
	if !ok {
		return
	}

	var req UpdateGroupRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.UpdateGroup(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) addMember(c *gin.Context) {
	id, ok := parseID(c, "group")
	if !ok {
		return
	}

	var req AddMemberRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.AddMember(c.Request.Context(), id, req.UserID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) removeMember(c *gin.Context) {
	id, ok := parseID(c, "group")
	if !ok {
		return
	}

	userID := c.Param("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid user id")
		return
	}

	resp, err := h.svc.RemoveMember(c.Request.Context(), id, userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) setExperimenterGroup(c *gin.Context) {
	userID := c.Param("id")
	if _, err := uuid.Parse(userID); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid user id")
		return
	}

	var req SetExperimenterGroupRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	if err := h.svc.SetExperimenterGroup(c.Request.Context(), userID, req); err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, nil)
}

func (h *Handler) listReviews(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	var status ReviewStatus
	if raw := c.Query("status"); raw != "" {
		status = ReviewStatus(raw)
		if !status.Valid() {
			api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid status filter")
			return
		}
	}

	resp, err := h.svc.ListReviews(c.Request.Context(), limit, offset, status)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OKWithMeta(c.Writer, resp.Data, &resp.Meta)
}

func (h *Handler) getReview(c *gin.Context) {
	id, ok := parseID(c, "review")
	if !ok {
		return
	}

	resp, err := h.svc.GetReview(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) actOnReview(c *gin.Context) {
	id, ok := parseID(c, "review")
	if !ok {
		return
	}

	var req ActOnReviewRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	resp, err := h.svc.Act(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) addComment(c *gin.Context) {
	id, ok := parseID(c, "review")
	if !ok {
		return
	}

	var req AddCommentRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}
	if req.ParentID != nil {
		if _, err := uuid.Parse(*req.ParentID); err != nil {
			api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid parent id")
			return
		}
	}

	resp, err := h.svc.AddComment(c.Request.Context(), middleware.CallerIDGin(c), id, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, resp)
}

func (h *Handler) resolveComment(c *gin.Context) {
	cid := c.Param("cid")
	if _, err := uuid.Parse(cid); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid comment id")
		return
	}

	var req ResolveCommentRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	comment, err := h.svc.ResolveCommentByID(c.Request.Context(), middleware.CallerIDGin(c), cid, req)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, comment)
}

func (h *Handler) deleteComment(c *gin.Context) {
	cid := c.Param("cid")
	if _, err := uuid.Parse(cid); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid comment id")
		return
	}

	if err := h.svc.DeleteCommentByID(c.Request.Context(), middleware.CallerIDGin(c), cid); err != nil {
		h.respondError(c, err)
		return
	}

	c.Writer.WriteHeader(http.StatusNoContent)
}

func parseID(c *gin.Context, what string) (string, bool) {
	idStr := c.Param("id")
	if _, err := uuid.Parse(idStr); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid "+what+" id")
		return "", false
	}

	return idStr, true
}

func (h *Handler) respondError(c *gin.Context, err error) {
	w := c.Writer
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrGroupNotFound), errors.Is(err, ErrCommentNotFound), errors.Is(err, ErrUserNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, err.Error())
	case errors.Is(err, ErrConflictName), errors.Is(err, ErrDuplicateMember), errors.Is(err, ErrDuplicateAction), errors.Is(err, ErrReviewExists):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrReviewClosed):
		logger.SetErrorType(c, logger.ErrorTypeConflict)
		api.Error(w, http.StatusConflict, api.Conflict, err.Error())
	case errors.Is(err, ErrForbidden):
		logger.SetErrorType(c, logger.ErrorTypeAuthorizationDenied)
		api.Error(w, http.StatusForbidden, api.Forbidden, err.Error())
	case errors.Is(err, ErrInvalidThreshold), errors.Is(err, ErrInvalidComment), errors.Is(err, ErrInvalidDecision), errors.Is(err, ErrMemberNotFound):
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
