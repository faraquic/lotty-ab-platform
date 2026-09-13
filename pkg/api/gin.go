package api

import (
	"github.com/gin-gonic/gin"
)

func OKGin(c *gin.Context, data any) {
	OK(c.Writer, data)
}

func OKWithMetaGin(c *gin.Context, data any, meta *PaginationMeta) {
	OKWithMeta(c.Writer, data, meta)
}

func ErrorGin(c *gin.Context, status int, code, message string) {
	Error(c.Writer, status, code, message)
}

func InternalErrorGin(c *gin.Context) {
	InternalError(c.Writer)
}

func ValidateRequestGin(c *gin.Context, dst any) error {
	return ValidateRequest(c.Writer, c.Request, dst)
}
