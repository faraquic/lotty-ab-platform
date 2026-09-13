package api

import (
	"errors"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

func OKFiber(c fiber.Ctx, data any) error {
	return c.Status(http.StatusOK).JSON(Response{Success: true, Data: data})
}

func OKWithMetaFiber(c fiber.Ctx, data any, meta *PaginationMeta) error {
	return c.Status(http.StatusOK).JSON(Response{Success: true, Data: data, Meta: meta})
}

func ErrorFiber(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(Response{Success: false, Error: &ErrorInfo{Code: code, Message: message}})
}

func InternalErrorFiber(c fiber.Ctx) error {
	return ErrorFiber(c, http.StatusInternalServerError, InternalServerError, InternalServerMessage)
}

func ValidateRequestFiber(c fiber.Ctx, dst any) error {
	body := c.Body()
	if len(body) > maxBodyBytes {
		_ = ErrorFiber(c, http.StatusRequestEntityTooLarge, PayloadTooLarge, "request body too large")
		return errors.New("request body too large")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		_ = ErrorFiber(c, http.StatusBadRequest, BadRequest, err.Error())
		return err
	}
	if err := validate.Struct(dst); err != nil {
		var invalid *validator.InvalidValidationError
		if errors.As(err, &invalid) {
			_ = InternalErrorFiber(c)
			return err
		}
		_ = ErrorFiber(c, http.StatusBadRequest, BadRequest, err.Error())
		return err
	}
	return nil
}
