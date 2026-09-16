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

type experimentVariantData struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Value     json.RawMessage `json:"value"`
	WeightBP  int             `json:"weight_bp"`
	IsControl bool            `json:"is_control"`
}

type experimentVersionData struct {
	ID               string  `json:"id"`
	VersionNum       int     `json:"version_num"`
	ReviewID         *string `json:"review_id"`
	WeightsTotal     int     `json:"weights_total"`
	DistributionSalt string  `json:"distribution_salt"`
}

type experimentResponseData struct {
	ID                 string                  `json:"id"`
	FlagID             string                  `json:"flag_id"`
	Name               string                  `json:"name"`
	Status             string                  `json:"status"`
	CurrentVersionID   *string                 `json:"current_version_id"`
	OwnerID            string                  `json:"owner_id"`
	Version            int                     `json:"version"`
	GuardrailPaused    bool                    `json:"guardrail_paused"`
	CompletionDecision *string                 `json:"completion_decision"`
	CompletionReason   *string                 `json:"completion_reason"`
	CurrentVersion     *experimentVersionData  `json:"current_version"`
	Variants           []experimentVariantData `json:"variants"`
	CreatedAt          string                  `json:"created_at"`
	UpdatedAt          string                  `json:"updated_at"`
}

type reviewApprovalData struct {
	ID         string  `json:"id"`
	ReviewerID string  `json:"reviewer_id"`
	Decision   string  `json:"decision"`
	Comment    *string `json:"comment"`
	CreatedAt  string  `json:"created_at"`
}

type reviewCommentData struct {
	ID        string              `json:"id"`
	AuthorID  string              `json:"author_id"`
	ParentID  *string             `json:"parent_id"`
	Body      string              `json:"body"`
	Resolved  bool                `json:"resolved"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
	Replies   []reviewCommentData `json:"replies"`
}

type reviewResponseData struct {
	ID           string               `json:"id"`
	ExperimentID string               `json:"experiment_id"`
	VersionID    string               `json:"version_id"`
	VersionNum   int                  `json:"version_num"`
	Status       string               `json:"status"`
	Approvals    []reviewApprovalData `json:"approvals"`
	Comments     []reviewCommentData  `json:"comments"`
	CreatedBy    string               `json:"created_by"`
	CreatedAt    string               `json:"created_at"`
	UpdatedAt    string               `json:"updated_at"`
}

type groupMemberData struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type groupResponseData struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	MinApprovals int               `json:"min_approvals"`
	Status       string            `json:"status"`
	Members      []groupMemberData `json:"members"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

type apiResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

type apiPaginatedResponse[T any] struct {
	Success bool                   `json:"success"`
	Data    []T                    `json:"data"`
	Meta    paginationMetaResponse `json:"meta"`
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

func decodePaginatedMetricsResponse(t *testing.T, resp *http.Response) apiPaginatedResponse[metricResponseData] {
	t.Helper()
	return decodeResponse[apiPaginatedResponse[metricResponseData]](t, resp)
}

func decodeExperimentResponse(t *testing.T, resp *http.Response) apiResponse[experimentResponseData] {
	t.Helper()
	return decodeResponse[apiResponse[experimentResponseData]](t, resp)
}

func decodePaginatedExperimentsResponse(t *testing.T, resp *http.Response) apiPaginatedResponse[experimentResponseData] {
	t.Helper()
	return decodeResponse[apiPaginatedResponse[experimentResponseData]](t, resp)
}

func decodeReviewResponse(t *testing.T, resp *http.Response) apiResponse[reviewResponseData] {
	t.Helper()
	return decodeResponse[apiResponse[reviewResponseData]](t, resp)
}

func decodeGroupResponse(t *testing.T, resp *http.Response) apiResponse[groupResponseData] {
	t.Helper()
	return decodeResponse[apiResponse[groupResponseData]](t, resp)
}
