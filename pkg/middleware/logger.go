package middleware

import (
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func LoggerGin(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		defer func() {
			status := c.Writer.Status()
			fields := []zap.Field{
				zap.String(logger.FieldRequestID, GetRequestIDGin(c)),
				zap.String(logger.FieldRequestMethod, c.Request.Method),
				zap.String(logger.FieldRoute, c.FullPath()),
				zap.Int(logger.FieldResponseCode, status),
				zap.Float64(logger.FieldDurationMs, float64(time.Since(start).Nanoseconds())/1e6),
				zap.Int(logger.FieldResponseSize, c.Writer.Size()),
				zap.String(logger.FieldClientAddress, c.ClientIP()),
			}

			if userID, exists := c.Get(logger.FieldUserID); exists {
				if uid, ok := userID.(string); ok && uid != "" {
					fields = append(fields, zap.String(logger.FieldUserID, uid))
				}
			}

			if errValue, exists := c.Get(logger.FieldErrorType); exists {
				if errStr, ok := errValue.(string); ok && errStr != "" {
					fields = append(fields, zap.String(logger.FieldErrorType, errStr))
				}
			}

			switch {
			case status >= 500:
				log.Error("http request completed", fields...)
			case status == 401 || status == 403:
				log.Warn("http request completed", fields...)
			default:
				log.Info("http request completed", fields...)
			}
		}()

		c.Next()
	}
}

func LoggerFiber(log *zap.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		status := c.Response().StatusCode()
		fields := []zap.Field{
			zap.String(logger.FieldRequestID, GetRequestIDFiber(c)),
			zap.String(logger.FieldRequestMethod, c.Method()),
			zap.String(logger.FieldRoute, c.Route().Path),
			zap.Int(logger.FieldResponseCode, status),
			zap.Float64(logger.FieldDurationMs, float64(time.Since(start).Nanoseconds())/1e6),
			zap.Int(logger.FieldResponseSize, len(c.Response().Body())),
			zap.String(logger.FieldClientAddress, c.IP()),
		}

		if userID := c.Locals(logger.FieldUserID); userID != nil {
			if uid, ok := userID.(string); ok && uid != "" {
				fields = append(fields, zap.String(logger.FieldUserID, uid))
			}
		}

		if errValue := c.Locals(logger.FieldErrorType); errValue != nil {
			if errStr, ok := errValue.(string); ok && errStr != "" {
				fields = append(fields, zap.String(logger.FieldErrorType, errStr))
			}
		}

		switch {
		case status >= 500:
			log.Error("http request completed", fields...)
		case status == 401 || status == 403:
			log.Warn("http request completed", fields...)
		default:
			log.Info("http request completed", fields...)
		}

		return err
	}
}
