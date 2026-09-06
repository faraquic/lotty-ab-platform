package middleware

import (
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
)

func CallerIDGin(c *gin.Context) string {
	id, _ := c.Get(logger.FieldUserID)
	if id == nil {
		return ""
	}
	uid, _ := id.(string)
	return uid
}

func CallerIDFiber(c fiber.Ctx) string {
	id := c.Locals(logger.FieldUserID)
	if id == nil {
		return ""
	}
	uid, _ := id.(string)
	return uid
}
