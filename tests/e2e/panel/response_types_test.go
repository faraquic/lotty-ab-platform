package e2e

import (
	"io"
	"net/http"
	"testing"

	"github.com/goccy/go-json"
)

type userResponseData struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Role      string  `json:"role"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type paginationMetaResponse struct {
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	Count   int   `json:"count"`
	Total   int64 `json:"total"`
	HasNext bool  `json:"has_next"`
}

type flagResponseData struct {
	ID           int64             `json:"id"`
	Key          string            `json:"key"`
	Type         string            `json:"type"`
	DefaultValue json.RawMessage   `json:"default_value"`
	Description  *string           `json:"description"`
	Owner        *userResponseData `json:"owner"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

type paginatedFlagData struct {
	Data []flagResponseData     `json:"data"`
	Meta paginationMetaResponse `json:"meta"`
}

type apiResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

func decodeResponse[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}
