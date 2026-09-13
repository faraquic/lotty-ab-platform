package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

func aggregationForType(mt string) map[string]any {
	switch mt {
	case "count":
		return map[string]any{"event_type": "purchase_completed"}
	case "sum":
		return map[string]any{"event_type": "purchase_completed", "field": "order_value"}
	case "unique_count":
		return map[string]any{"event_type": "purchase_completed"}
	case "average":
		return map[string]any{"event_type": "purchase_completed", "field": "order_value"}
	case "percentile":
		return map[string]any{"event_type": "latency_reported", "field": "latency_ms", "level": 0.95}
	case "ratio":
		return map[string]any{
			"numerator":   map[string]any{"event_type": "purchase_completed"},
			"denominator": map[string]any{"event_type": "exposure"},
		}
	default:
		return map[string]any{"event_type": "purchase_completed"}
	}
}

func validAttribution() map[string]any {
	return map[string]any{"require_exposure": true, "window_days": 7, "fallback": "subject"}
}

func TestCreateMetric(t *testing.T) {
	types := []string{"count", "sum", "unique_count", "ratio", "percentile", "average"}

	for _, mt := range types {
		t.Run(mt, func(t *testing.T) {
			key := fmt.Sprintf("metric-%s-%s", mt, runID)
			name := fmt.Sprintf("Test %s Metric", mt)

			resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken,
				jsonBody(map[string]any{
					"key":         key,
					"name":        name,
					"metric_type": mt,
					"aggregation": aggregationForType(mt),
					"attribution": validAttribution(),
				}))
			defer resp.Body.Close()

			requireStatus(t, resp, http.StatusOK)
			requireJSONContentType(t, resp)

			result := decodeMetricResponse(t, resp)

			if !result.Success {
				t.Error("expected success=true")
			}
			if result.Data.Key != key {
				t.Errorf("key: got %q, want %q", result.Data.Key, key)
			}
			if result.Data.Name != name {
				t.Errorf("name: got %q, want %q", result.Data.Name, name)
			}
			if result.Data.MetricType != mt {
				t.Errorf("metric_type: got %q, want %q", result.Data.MetricType, mt)
			}
			if result.Data.IsBuiltin {
				t.Error("is_builtin should be false for API-created metrics")
			}
			if result.Data.Status != "active" {
				t.Errorf("status: got %q, want %q", result.Data.Status, "active")
			}
			if mt == "ratio" {
				if result.Data.Aggregation.Numerator == nil || result.Data.Aggregation.Numerator.EventType != "purchase_completed" {
					t.Errorf("aggregation.numerator: unexpected %+v", result.Data.Aggregation.Numerator)
				}
				if result.Data.Aggregation.Denominator == nil || result.Data.Aggregation.Denominator.EventType != "exposure" {
					t.Errorf("aggregation.denominator: unexpected %+v", result.Data.Aggregation.Denominator)
				}
			} else {
				if result.Data.Aggregation.EventType == nil {
					t.Error("aggregation.event_type should be set")
				}
			}
			if result.Data.Attribution.WindowDays != 7 {
				t.Errorf("attribution.window_days: got %d, want 7", result.Data.Attribution.WindowDays)
			}
			if result.Data.Attribution.Fallback != "subject" {
				t.Errorf("attribution.fallback: got %q, want %q", result.Data.Attribution.Fallback, "subject")
			}
			if !result.Data.Attribution.RequireExposure {
				t.Error("attribution.require_exposure should be true")
			}
			if result.Data.CreatedBy == nil {
				t.Error("created_by should be present")
			}
			if result.Data.CreatedBy != nil && result.Data.CreatedBy.ID == "" {
				t.Error("created_by ID should be positive")
			}
			if result.Data.UpdatedBy == nil {
				t.Error("updated_by should be present")
			}
			if result.Data.CreatedBy != nil && result.Data.UpdatedBy != nil {
				if result.Data.CreatedBy.ID != result.Data.UpdatedBy.ID {
					t.Error("created_by and updated_by should be the same on create")
				}
			}
			if result.Data.CreatedAt == "" || result.Data.UpdatedAt == "" {
				t.Error("created_at and updated_at should be set")
			}
		})
	}
}

func TestCreateMetric_WithDescription(t *testing.T) {
	key := "metric-with-desc-" + runID
	desc := "A metric with a description"

	resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken,
		jsonBody(map[string]any{
			"key":         key,
			"name":        "Metric With Description",
			"metric_type": "count",
			"description": desc,
			"aggregation": map[string]any{"event_type": "signup"},
			"attribution": validAttribution(),
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeMetricResponse(t, resp)

	if result.Data.Description == nil || *result.Data.Description != desc {
		t.Errorf("description: got %v, want %q", result.Data.Description, desc)
	}
}

func TestCreateMetric_DefaultStatus(t *testing.T) {
	key := "metric-default-status-" + runID

	resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken,
		jsonBody(map[string]any{
			"key":         key,
			"name":        "Default Status Metric",
			"metric_type": "count",
			"aggregation": map[string]any{"event_type": "test"},
			"attribution": validAttribution(),
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeMetricResponse(t, resp)

	if result.Data.Status != "active" {
		t.Errorf("status: got %q, want %q (default)", result.Data.Status, "active")
	}
}

func TestCreateMetric_DuplicateKey(t *testing.T) {
	baseKey := "dup-key-metric-" + runID
	_, createdKey := createMetric(t, baseKey, "Dup Key Metric", "count")

	resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken,
		jsonBody(map[string]any{
			"key":         createdKey,
			"name":        "Different Name",
			"metric_type": "count",
			"aggregation": map[string]any{"event_type": "test"},
			"attribution": validAttribution(),
		}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
}

func TestCreateMetric_DuplicateName(t *testing.T) {
	name := "Dup Name Metric " + runID
	createMetric(t, "dup-name-metric-"+runID, name, "count")

	resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken,
		jsonBody(map[string]any{
			"key":         "other-key-" + runID,
			"name":        name,
			"metric_type": "count",
			"aggregation": map[string]any{"event_type": "test"},
			"attribution": validAttribution(),
		}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
}

func TestCreateMetric_RejectsInvalidRequests(t *testing.T) {
	validAgg := func() map[string]any { return map[string]any{"event_type": "test"} }
	validAttr := func() map[string]any { return validAttribution() }
	cases := []struct {
		name         string
		payload      map[string]any
		expectedCode int
	}{
		{"empty body", map[string]any{}, http.StatusBadRequest},
		{"missing key", map[string]any{"metric_type": "count", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"missing name", map[string]any{"key": "test-m", "metric_type": "count", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"missing metric_type", map[string]any{"key": "test-m", "name": "Test", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"missing aggregation", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "attribution": validAttr()}, http.StatusBadRequest},
		{"missing attribution", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": validAgg()}, http.StatusBadRequest},
		{"invalid metric_type", map[string]any{"key": "test-m", "name": "Test", "metric_type": "invalid", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"key too short", map[string]any{"key": "ab", "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"key too long", map[string]any{"key": strings.Repeat("a", 129), "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"name too long", map[string]any{"key": "valid-key", "name": strings.Repeat("a", 257), "metric_type": "count", "aggregation": validAgg(), "attribution": validAttr()}, http.StatusBadRequest},
		{"malformed aggregation JSON", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": "not-json", "attribution": validAttr()}, http.StatusBadRequest},
		{"malformed attribution JSON", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": "not-json"}, http.StatusBadRequest},
		{"array aggregation", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": []any{1, 2}, "attribution": validAttr()}, http.StatusBadRequest},
		{"scalar aggregation", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": 42, "attribution": validAttr()}, http.StatusBadRequest},
		{"null aggregation", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": nil, "attribution": validAttr()}, http.StatusBadRequest},
		{"empty aggregation", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": map[string]any{}, "attribution": validAttr()}, http.StatusBadRequest},
		{"count with field", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": map[string]any{"event_type": "e", "field": "f"}, "attribution": validAttr()}, http.StatusBadRequest},
		{"sum without field", map[string]any{"key": "test-m", "name": "Test", "metric_type": "sum", "aggregation": map[string]any{"event_type": "e"}, "attribution": validAttr()}, http.StatusBadRequest},
		{"percentile without level", map[string]any{"key": "test-m", "name": "Test", "metric_type": "percentile", "aggregation": map[string]any{"event_type": "e", "field": "f"}, "attribution": validAttr()}, http.StatusBadRequest},
		{"percentile level out of range", map[string]any{"key": "test-m", "name": "Test", "metric_type": "percentile", "aggregation": map[string]any{"event_type": "e", "field": "f", "level": 1.5}, "attribution": validAttr()}, http.StatusBadRequest},
		{"ratio without denominator", map[string]any{"key": "test-m", "name": "Test", "metric_type": "ratio", "aggregation": map[string]any{"numerator": map[string]any{"event_type": "a"}}, "attribution": validAttr()}, http.StatusBadRequest},
		{"ratio with top-level event_type", map[string]any{"key": "test-m", "name": "Test", "metric_type": "ratio", "aggregation": map[string]any{"event_type": "a", "numerator": map[string]any{"event_type": "a"}, "denominator": map[string]any{"event_type": "b"}}, "attribution": validAttr()}, http.StatusBadRequest},
		{"attribution window zero", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": map[string]any{"require_exposure": true, "window_days": 0, "fallback": "subject"}}, http.StatusBadRequest},
		{"attribution window too large", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": map[string]any{"require_exposure": true, "window_days": 99, "fallback": "subject"}}, http.StatusBadRequest},
		{"attribution bad fallback", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": map[string]any{"require_exposure": true, "window_days": 7, "fallback": "last_touch"}}, http.StatusBadRequest},
		{"attribution missing fallback", map[string]any{"key": "test-m", "name": "Test", "metric_type": "count", "aggregation": validAgg(), "attribution": map[string]any{"require_exposure": true, "window_days": 7}}, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken, jsonBody(tc.payload))
			defer resp.Body.Close()
			requireErrorResponse(t, resp, tc.expectedCode, "BAD_REQUEST")
		})
	}
}

func TestGetMetric(t *testing.T) {
	id, key := createMetric(t, "get-metric", "Get Metric", "count")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeMetricResponse(t, resp)

	if result.Data.ID != id {
		t.Errorf("id: got %s, want %s", result.Data.ID, id)
	}
	if result.Data.Key != key {
		t.Errorf("key: got %q, want %q", result.Data.Key, key)
	}
	if result.Data.Name != "Get Metric" {
		t.Errorf("name: got %q, want %q", result.Data.Name, "Get Metric")
	}
	if result.Data.CreatedBy == nil || result.Data.UpdatedBy == nil {
		t.Error("created_by and updated_by should be present")
	}
}

func TestGetMetric_InvalidID(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics/abc", adminToken, nil)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestGetMetric_Nonexistent(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics/0198f4c0-dead-7000-8000-000000000001", adminToken, nil)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusNotFound, "NOT_FOUND")
}

func TestGetMetric_Archived(t *testing.T) {
	id, _ := createMetric(t, "get-archived", "Get Archived Metric", "count")

	archiveResp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"status": "archived"}))
	defer archiveResp.Body.Close()
	requireStatus(t, archiveResp, http.StatusOK)

	defaultResp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken, nil)
	defer defaultResp.Body.Close()
	requireErrorResponse(t, defaultResp, http.StatusNotFound, "NOT_FOUND")

	archivedResp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s?archived=true", id), adminToken, nil)
	defer archivedResp.Body.Close()
	requireStatus(t, archivedResp, http.StatusOK)

	result := decodeMetricResponse(t, archivedResp)
	if result.Data.ID != id {
		t.Errorf("id: got %s, want %s", result.Data.ID, id)
	}
	if result.Data.Status != "archived" {
		t.Errorf("status: got %q, want %q", result.Data.Status, "archived")
	}
}

func TestListMetrics(t *testing.T) {
	_, keyA := createMetric(t, "list-metric-a", "List Metric A", "count")
	_, keyB := createMetric(t, "list-metric-b", "List Metric B", "sum")

	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics?limit=100", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodePaginatedMetricsResponse(t, resp)

	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Data.Data) < 2 {
		t.Errorf("expected at least 2 metrics, got %d", len(result.Data.Data))
	}

	foundA := false
	foundB := false
	for _, m := range result.Data.Data {
		if m.Key == keyA {
			foundA = true
		}
		if m.Key == keyB {
			foundB = true
		}
	}
	if !foundA {
		t.Errorf("%s not found in list", keyA)
	}
	if !foundB {
		t.Errorf("%s not found in list", keyB)
	}

	for _, m := range result.Data.Data {
		if m.CreatedBy != nil {
			t.Error("list items should not have created_by")
		}
		if m.UpdatedBy != nil {
			t.Error("list items should not have updated_by")
		}
	}

	if result.Data.Meta.Count != len(result.Data.Data) {
		t.Errorf("meta.count: got %d, want %d", result.Data.Meta.Count, len(result.Data.Data))
	}
	if result.Data.Meta.Total < int64(result.Data.Meta.Count) {
		t.Errorf("meta.total (%d) < meta.count (%d)", result.Data.Meta.Total, result.Data.Meta.Count)
	}
}

func TestListMetrics_Pagination(t *testing.T) {
	_, _ = createMetric(t, "page-metric-a", "Page Metric A", "count")
	_, _ = createMetric(t, "page-metric-b", "Page Metric B", "sum")
	_, _ = createMetric(t, "page-metric-c", "Page Metric C", "average")

	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics?limit=2&offset=0", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodePaginatedMetricsResponse(t, resp)

	if len(result.Data.Data) != 2 {
		t.Errorf("limit=2: got %d metrics, want 2", len(result.Data.Data))
	}
	if result.Data.Meta.Limit != 2 {
		t.Errorf("meta.limit: got %d, want 2", result.Data.Meta.Limit)
	}
	if result.Data.Meta.Offset != 0 {
		t.Errorf("meta.offset: got %d, want 0", result.Data.Meta.Offset)
	}
	if !result.Data.Meta.HasNext {
		t.Error("expected has_next=true on first page")
	}
}

func TestListMetrics_OffsetBeyondTotal(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics?limit=1&offset=100000", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	result := decodePaginatedMetricsResponse(t, resp)

	if len(result.Data.Data) != 0 {
		t.Errorf("offset beyond total: got %d metrics, want 0", len(result.Data.Data))
	}
	if result.Data.Meta.Count != 0 {
		t.Errorf("meta.count: got %d, want 0", result.Data.Meta.Count)
	}
	if result.Data.Meta.HasNext {
		t.Error("expected has_next=false when offset beyond total")
	}
}

func TestListMetrics_ExcludesArchived(t *testing.T) {
	_, keyActive := createMetric(t, "list-active-metric", "List Active Metric", "count")
	idArchived, keyArchived := createMetric(t, "list-archived-metric", "List Archived Metric", "sum")

	archiveResp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", idArchived), adminToken, jsonBody(map[string]any{"status": "archived"}))
	archiveResp.Body.Close()

	defaultResp := doRequest(http.MethodGet, "/api/v1/panel/metrics?limit=100", adminToken, nil)
	defer defaultResp.Body.Close()
	defaultResult := decodePaginatedMetricsResponse(t, defaultResp)

	for _, m := range defaultResult.Data.Data {
		if m.Key == keyArchived {
			t.Error("archived metric should not appear in default list")
		}
	}

	defaultFound := false
	for _, m := range defaultResult.Data.Data {
		if m.Key == keyActive {
			defaultFound = true
		}
	}
	if !defaultFound {
		t.Error("active metric should appear in default list")
	}

	archivedResp := doRequest(http.MethodGet, "/api/v1/panel/metrics?limit=100&archived=true", adminToken, nil)
	defer archivedResp.Body.Close()
	archivedResult := decodePaginatedMetricsResponse(t, archivedResp)

	foundArchived := false
	for _, m := range archivedResult.Data.Data {
		if m.Key == keyArchived {
			foundArchived = true
		}
	}
	if !foundArchived {
		t.Error("archived metric should appear when archived=true")
	}
}

func TestUpdateMetric(t *testing.T) {
	id, _ := createMetric(t, "update-metric", "Update Metric", "count")

	newName := "Updated Metric Name"
	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{
			"name":        newName,
			"description": "Updated description",
			"aggregation": map[string]any{"event_type": "updated_event"},
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeMetricResponse(t, resp)

	if result.Data.Name != newName {
		t.Errorf("name: got %q, want %q", result.Data.Name, newName)
	}
	if result.Data.Description == nil || *result.Data.Description != "Updated description" {
		t.Errorf("description: got %v, want 'Updated description'", result.Data.Description)
	}
	if result.Data.Aggregation.EventType == nil || *result.Data.Aggregation.EventType != "updated_event" {
		t.Errorf("aggregation.event_type: unexpected %+v", result.Data.Aggregation)
	}
	if result.Data.CreatedBy == nil || result.Data.UpdatedBy == nil {
		t.Error("created_by and updated_by should be present")
	}
}

func TestUpdateMetric_Archive(t *testing.T) {
	id, _ := createMetric(t, "archive-metric", "Archive Metric", "count")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"status": "archived"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeMetricResponse(t, resp)
	if result.Data.Status != "archived" {
		t.Errorf("status: got %q, want %q", result.Data.Status, "archived")
	}

	getDefault := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken, nil)
	defer getDefault.Body.Close()
	requireErrorResponse(t, getDefault, http.StatusNotFound, "NOT_FOUND")

	getArchived := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s?archived=true", id), adminToken, nil)
	defer getArchived.Body.Close()
	requireStatus(t, getArchived, http.StatusOK)
}

func TestUpdateMetric_UpdateArchived(t *testing.T) {
	id, _ := createMetric(t, "update-archived", "Update Archived Metric", "count")

	archiveResp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"status": "archived"}))
	defer archiveResp.Body.Close()
	requireStatus(t, archiveResp, http.StatusOK)

	updateResp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"name": "Re-archived Name"}))
	defer updateResp.Body.Close()
	requireStatus(t, updateResp, http.StatusOK)

	getResp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s?archived=true", id), adminToken, nil)
	defer getResp.Body.Close()
	result := decodeMetricResponse(t, getResp)
	if result.Data.Name != "Re-archived Name" {
		t.Errorf("name: got %q, want %q", result.Data.Name, "Re-archived Name")
	}
}

func TestUpdateMetric_PreservesCreatedBy(t *testing.T) {
	id, _ := createMetric(t, "preserve-audit", "Preserve Audit", "count")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken, nil)
	defer resp.Body.Close()
	before := decodeMetricResponse(t, resp)

	updateResp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"name": "Preserve Audit Updated"}))
	defer updateResp.Body.Close()

	requireStatus(t, updateResp, http.StatusOK)
	after := decodeMetricResponse(t, updateResp)

	if before.Data.CreatedBy.ID != after.Data.CreatedBy.ID {
		t.Error("created_by should remain stable after update")
	}
}

func TestUpdateMetric_Nonexistent(t *testing.T) {
	resp := doRequest(http.MethodPatch, "/api/v1/panel/metrics/0198f4c0-dead-7000-8000-000000000001", adminToken,
		jsonBody(map[string]any{"name": "test"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusNotFound, "NOT_FOUND")
}

func TestUpdateMetric_InvalidAggregation(t *testing.T) {
	id, _ := createMetric(t, "invalid-agg-update", "Invalid Agg Update", "count")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"aggregation": "not-json"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestUpdateMetric_EmptyAggregation(t *testing.T) {
	id, _ := createMetric(t, "empty-agg-update", "Empty Agg Update", "count")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"aggregation": map[string]any{}}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestUpdateMetric_WrongShapeForType(t *testing.T) {
	id, _ := createMetric(t, "shape-agg-update", "Shape Agg Update", "count")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken,
		jsonBody(map[string]any{"aggregation": map[string]any{
			"numerator":   map[string]any{"event_type": "a"},
			"denominator": map[string]any{"event_type": "b"},
		}}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestMetrics_NoSecretsInResponse(t *testing.T) {
	id, _ := createMetric(t, "no-secrets-metric", "No Secrets Metric", "count")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", id), adminToken, nil)
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&raw)

	data, ok := raw["data"]
	if !ok {
		t.Fatal("response missing data field")
	}

	var metricFields map[string]json.RawMessage
	json.Unmarshal(data, &metricFields)

	for _, field := range []string{"password_hash", "password"} {
		if _, exists := metricFields[field]; exists {
			t.Errorf("response must not contain %s", field)
		}
	}
}

func TestMetrics_JSONContentType(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics", adminToken, nil)
	defer resp.Body.Close()

	requireJSONContentType(t, resp)
}

func TestUnauthenticatedMetric_RejectedFromAllEndpoints(t *testing.T) {
	id, _ := createMetric(t, "unauth-metric", "Unauth Metric", "count")

	t.Run("GET /metrics", func(t *testing.T) {
		resp := doRequest(http.MethodGet, "/api/v1/panel/metrics", "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /metrics/:id", func(t *testing.T) {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("POST /metrics", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", "",
			jsonBody(map[string]any{
				"key":         "should-not-work",
				"metric_type": "count",
				"aggregation": map[string]any{"event_type": "test"},
				"attribution": validAttribution(),
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("PATCH /metrics/:id", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", id), "",
			jsonBody(map[string]any{"name": "hack"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestViewerCanManageMetrics(t *testing.T) {
	viewerEmail := testEmail("viewer-metrics")
	createResp := doRequest(http.MethodPost, "/api/v1/panel/users", adminToken,
		jsonBody(map[string]string{
			"full_name": testFullName("viewer-metrics"),
			"email":     viewerEmail,
			"password":  "testpass123",
			"role":      "viewer",
		}))
	if createResp.StatusCode != http.StatusOK {
		t.Skipf("failed to create viewer: %d", createResp.StatusCode)
	}
	createResp.Body.Close()

	viewerToken := login(viewerEmail, "testpass123")
	if viewerToken == "" {
		t.Skip("could not login as viewer")
	}

	targetID, _ := createMetric(t, "viewer-target-metric", "Viewer Target Metric", "count")

	t.Run("can create metric", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", viewerToken,
			jsonBody(map[string]any{
				"key":         "viewer-create-" + runID,
				"name":        "Viewer Create Metric",
				"metric_type": "count",
				"aggregation": map[string]any{"event_type": "test"},
				"attribution": validAttribution(),
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can list metrics", func(t *testing.T) {
		resp := doRequest(http.MethodGet, "/api/v1/panel/metrics", viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can get metric by id", func(t *testing.T) {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/metrics/%s", targetID), viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can update metric", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/metrics/%s", targetID), viewerToken,
			jsonBody(map[string]any{"name": "Viewer Target Metric Updated"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})
}

func TestUpdateMetric_NegativeID(t *testing.T) {
	resp := doRequest(http.MethodPatch, "/api/v1/panel/metrics/-1", adminToken,
		jsonBody(map[string]any{"name": "test"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestGetMetric_NegativeID(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/metrics/-1", adminToken, nil)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func createMetric(t *testing.T, key, name, metricType string, extra ...map[string]any) (string, string) {
	t.Helper()
	uniqueKey := key + "-" + runID

	payload := map[string]any{
		"key":         uniqueKey,
		"name":        name,
		"metric_type": metricType,
		"aggregation": aggregationForType(metricType),
		"attribution": validAttribution(),
	}
	for _, e := range extra {
		for k, v := range e {
			payload[k] = v
		}
	}

	resp := doRequest(http.MethodPost, "/api/v1/panel/metrics", adminToken, jsonBody(payload))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("createMetric(%s): expected 200, got %d", uniqueKey, resp.StatusCode)
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	return result.Data.ID, uniqueKey
}
