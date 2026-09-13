package e2e

import (
	"io"
	"net/http"
	"testing"

	"github.com/goccy/go-json"
)

type userResponseData struct {
	ID        string  `json:"id"`
	FullName  string  `json:"full_name"`
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
	ID           string            `json:"id"`
	Key          string            `json:"key"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	DefaultValue json.RawMessage   `json:"default_value"`
	Description  *string           `json:"description"`
	CreatedBy    *userResponseData `json:"created_by"`
	UpdatedBy    *userResponseData `json:"updated_by"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

type paginatedFlagData struct {
	Data []flagResponseData     `json:"data"`
	Meta paginationMetaResponse `json:"meta"`
}

type eventRefData struct {
	EventType string  `json:"event_type"`
	Field     *string `json:"field"`
}

type aggregationData struct {
	EventType   *string       `json:"event_type"`
	Field       *string       `json:"field"`
	Level       *float64      `json:"level"`
	Numerator   *eventRefData `json:"numerator"`
	Denominator *eventRefData `json:"denominator"`
}

type attributionData struct {
	RequireExposure bool   `json:"require_exposure"`
	WindowDays      int    `json:"window_days"`
	Fallback        string `json:"fallback"`
}

type metricResponseData struct {
	ID          string            `json:"id"`
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	MetricType  string            `json:"metric_type"`
	Aggregation aggregationData   `json:"aggregation"`
	Attribution attributionData   `json:"attribution"`
	IsBuiltin   bool              `json:"is_builtin"`
	Status      string            `json:"status"`
	CreatedBy   *userResponseData `json:"created_by"`
	UpdatedBy   *userResponseData `json:"updated_by"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type paginatedMetricData struct {
	Data []metricResponseData   `json:"data"`
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

func decodeMetricResponse(t *testing.T, resp *http.Response) apiResponse[metricResponseData] {
	t.Helper()
	return decodeResponse[apiResponse[metricResponseData]](t, resp)
}

func decodePaginatedMetricsResponse(t *testing.T, resp *http.Response) apiResponse[paginatedMetricData] {
	t.Helper()
	return decodeResponse[apiResponse[paginatedMetricData]](t, resp)
}
