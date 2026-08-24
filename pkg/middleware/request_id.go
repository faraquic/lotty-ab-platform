package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !isValidUUIDv7(id) {
			generated, err := uuid.NewV7()
			if err != nil {
				c.AbortWithStatus(500)
				return
			}
			id = generated.String()
		}

		c.Set(requestIDKey, id)
		c.Writer.Header().Set(RequestIDHeader, id)

		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	id, _ := c.Get(requestIDKey)
	s, _ := id.(string)

	return s
}

func isValidUUIDv7(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u.Version() == 7
}
