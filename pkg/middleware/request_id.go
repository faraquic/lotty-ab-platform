package middleware

import (
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

func RequestIDGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := resolveID(c.GetHeader(RequestIDHeader))
		c.Set(logger.FieldRequestID, id)
		c.Writer.Header().Set(RequestIDHeader, id)
		c.Next()
	}
}

func RequestIDFiber() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := resolveID(c.Get(RequestIDHeader))
		c.Locals(logger.FieldRequestID, id)
		c.Set(RequestIDHeader, id)
		return c.Next()
	}
}

func GetRequestIDGin(c *gin.Context) string {
	id, _ := c.Get(logger.FieldRequestID)
	s, _ := id.(string)
	return s
}

func GetRequestIDFiber(c fiber.Ctx) string {
	id, _ := c.Locals(logger.FieldRequestID).(string)
	return id
}

func resolveID(header string) string {
	if isValidUUIDv7(header) {
		return header
	}
	generated, err := uuid.NewV7()
	if err != nil {
		return ""
	}
	return generated.String()
}

func isValidUUIDv7(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u.Version() == 7
}
