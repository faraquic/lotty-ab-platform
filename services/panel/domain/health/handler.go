package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/health", h.health)
}

func (h *Handler) health(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}
