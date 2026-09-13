package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/goccy/go-json"

	"github.com/go-playground/validator/v10"
)

var validate = func() *validator.Validate {
	v := validator.New()
	v.SetTagName("binding")

	return v
}()

type Response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
	Meta    any        `json:"meta,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

type PaginationMeta struct {
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	Count   int   `json:"count"`
	Total   int64 `json:"total"`
	HasNext bool  `json:"has_next"`
}

const (
	StatusOK            = "OK"
	BadRequest          = "BAD_REQUEST"
	Unauthorized        = "UNAUTHORIZED"
	Forbidden           = "FORBIDDEN"
	NotFound            = "NOT_FOUND"
	Conflict            = "CONFLICT"
	PayloadTooLarge     = "PAYLOAD_TOO_LARGE"
	InternalServerError = "INTERNAL_SERVER_ERROR"
	ServiceUnavailable  = "SERVICE_UNAVAILABLE"
	SnapshotUnavailable = "SNAPSHOT_UNAVAILABLE"
	UnprocessableEntity = "UNPROCESSABLE_ENTITY"
	TooManyRequests     = "TOO_MANY_REQUESTS"
	BadGateway          = "BAD_GATEWAY"

	InternalServerMessage = "internal server error"
)

func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

func OKWithMeta(w http.ResponseWriter, data any, meta *PaginationMeta) {
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

func Error(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}

func InternalError(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, InternalServerError, InternalServerMessage)
}

// maxBodyBytes caps request bodies to protect against oversized payloads.
// The limit is enforced explicitly because some JSON decoders (e.g.
// goccy/go-json) swallow the underlying *http.MaxBytesError, so relying
// on http.MaxBytesReader alone is not enough to guarantee a 413.
const maxBodyBytes = 1 << 20 // 1 MiB

func ValidateRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		Error(w, http.StatusBadRequest, BadRequest, "invalid request body")
		return err
	}

	if len(body) > maxBodyBytes {
		Error(w, http.StatusRequestEntityTooLarge, PayloadTooLarge, "request body too large")
		return errors.New("request body too large")
	}

	if err := json.Unmarshal(body, dst); err != nil {
		Error(w, http.StatusBadRequest, BadRequest, err.Error())
		return err
	}

	if err := validate.Struct(dst); err != nil {
		var invalid *validator.InvalidValidationError
		if errors.As(err, &invalid) {
			InternalError(w)
			return err
		}

		Error(w, http.StatusBadRequest, BadRequest, err.Error())
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
