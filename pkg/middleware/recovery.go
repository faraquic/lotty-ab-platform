package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type PanicData struct {
	RequestID string
	Method    string
	Route     string
	Value     any
	Stack     []byte
}

func logPanic(log *zap.Logger, data *PanicData) {
	log.Error(
		"panic recovered",
		zap.String(logger.FieldRequestID, data.RequestID),
		zap.String(logger.FieldRequestMethod, data.Method),
		zap.String(logger.FieldRoute, data.Route),
		zap.String(logger.FieldErrorType, "panic"),
		zap.String(logger.FieldPanicMsg, fmt.Sprintf("%v", data.Value)),
		zap.ByteString(logger.FieldStack, data.Stack),
	)
}

func panicResponse() map[string]any {
	return map[string]any{
		"success": false,
		"error": map[string]string{
			"code":    api.InternalServerError,
			"message": api.InternalServerMessage,
		},
	}
}

func RecoveryGin(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				data := &PanicData{
					RequestID: GetRequestIDGin(c),
					Method:    c.Request.Method,
					Route:     c.FullPath(),
					Value:     r,
					Stack:     debug.Stack(),
				}
				logPanic(log, data)
				c.Set(logger.FieldErrorType, string(logger.ErrorTypePanic))
				c.AbortWithStatusJSON(http.StatusInternalServerError, panicResponse())
			}
		}()
		c.Next()
	}
}

func RecoveryFiber(log *zap.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				data := &PanicData{
					RequestID: GetRequestIDFiber(c),
					Method:    c.Method(),
					Route:     c.Route().Path,
					Value:     r,
					Stack:     debug.Stack(),
				}
				logPanic(log, data)
				c.Set(logger.FieldErrorType, string(logger.ErrorTypePanic))
				_ = c.Status(fiber.StatusInternalServerError).JSON(panicResponse())
			}
		}()
		return c.Next()
	}
}
