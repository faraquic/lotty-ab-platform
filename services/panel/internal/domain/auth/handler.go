package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	libauth "github.com/faraquic/lotty-ab-platform/pkg/auth"
)

type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/login", h.login)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if err := api.ValidateRequest(c.Writer, c.Request, &req); err != nil {
		return
	}

	token, expiry, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			api.Error(c.Writer, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		case errors.Is(err, libauth.ErrInvalidToken), errors.Is(err, libauth.ErrExpiredToken):
			api.Error(c.Writer, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		default:
			h.log.Error("login failed", zap.Error(err))
			api.InternalError(c.Writer)
		}
		return
	}

	api.OK(c.Writer, LoginResponse{Token: token, ExpiresAt: expiry})
}
