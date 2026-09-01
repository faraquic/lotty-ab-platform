package middleware

import (
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
)

func CallerIDGin(c *gin.Context) int64 {
	id, _ := c.Get(logger.FieldUserID)
	v, _ := id.(int64)
	return v
}

func CallerIDFiber(c fiber.Ctx) int64 {
	id := c.Locals(logger.FieldUserID)
	v, _ := id.(int64)
	return v
}
