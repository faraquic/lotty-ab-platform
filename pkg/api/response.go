package api

import (
	"errors"
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
	Meta    *Meta      `json:"meta,omitempty"`
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

const (
	StatusOK            = "OK"
	BadRequest          = "BAD_REQUEST"
	NotFound            = "NOT_FOUND"
	PayloadTooLarge     = "PAYLOAD_TOO_LARGE"
	InternalServerError = "INTERNAL_SERVER_ERROR"

	internalServerMessage = "internal server error"
)

func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

func Error(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}

func InternalError(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, InternalServerError, internalServerMessage)
}

func ValidateRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			Error(w, http.StatusRequestEntityTooLarge, PayloadTooLarge, "request body too large")
			return err
		}

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
