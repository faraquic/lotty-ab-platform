package logger

import (
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
)

type ErrorType string

const (
	ErrorTypeValidationError        ErrorType = "validation_error"
	ErrorTypeAuthenticationFailed   ErrorType = "authentication_failed"
	ErrorTypeAuthorizationDenied    ErrorType = "authorization_denied"
	ErrorTypeNotFound               ErrorType = "not_found"
	ErrorTypeConflict               ErrorType = "conflict"
	ErrorTypeDatabaseUnavailable    ErrorType = "database_unavailable"
	ErrorTypeDatabaseQueryFailed    ErrorType = "database_query_failed"
	ErrorTypeRedisUnavailable       ErrorType = "redis_unavailable"
	ErrorTypeRedisSessionMissing    ErrorType = "redis_session_missing"
	ErrorTypeStorageUnavailable     ErrorType = "storage_unavailable"
	ErrorTypeStorageOperationFailed ErrorType = "storage_operation_failed"
	ErrorTypeInternalError          ErrorType = "internal_error"
	ErrorTypePanic                  ErrorType = "panic"
)

func SetErrorType(c *gin.Context, t ErrorType) {
	c.Set("error.type", string(t))
}

func SetErrorTypeFiber(c fiber.Ctx, t ErrorType) {
	c.Locals("error.type", string(t))
}
