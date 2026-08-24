package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Debug(
			"request received",
			zap.String("request_id", GetRequestID(c)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("remote_addr", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)

		start := time.Now()

		defer func() {
			log.Info(
				"request completed",
				zap.String("request_id", GetRequestID(c)),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
				zap.Int("status", c.Writer.Status()),
				zap.Int("bytes", c.Writer.Size()),
				zap.Float64("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			)
		}()

		c.Next()
	}
}
