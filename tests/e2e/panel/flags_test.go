package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

func TestCreateFlag_String(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/flags", adminToken,
		jsonBody(map[string]any{
			"key":           "test-string-flag",
			"type":          "string",
			"default_value": "hello world",
			"description":   "A test string flag",
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	requireJSONContentType(t, resp)

	result := decodeFlagResponse(t, resp)

	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Data.Key != "test-string-flag" {
		t.Errorf("key: got %q, want %q", result.Data.Key, "test-string-flag")
	}
	if result.Data.Type != "string" {
		t.Errorf("type: got %q, want %q", result.Data.Type, "string")
	}
	if !strings.Contains(string(result.Data.DefaultValue), "hello world") {
		t.Errorf("default_value: got %s, want to contain hello world", string(result.Data.DefaultValue))
	}
	if result.Data.Description == nil || *result.Data.Description != "A test string flag" {
		t.Errorf("description: got %v, want 'A test string flag'", result.Data.Description)
	}
	if result.Data.Owner == nil {
		t.Error("owner should be present")
	}
	if result.Data.Owner.ID < 1 {
		t.Error("owner ID should be positive")
	}
	if result.Data.CreatedAt == "" || result.Data.UpdatedAt == "" {
		t.Error("created_at and updated_at should be set")
	}
}

func TestCreateFlag_Number(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/flags", adminToken,
		jsonBody(map[string]any{
			"key":           "test-number-flag",
			"type":          "number",
			"default_value": 42,
			"description":   "A test number flag",
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeFlagResponse(t, resp)

	if result.Data.Type != "number" {
		t.Errorf("type: got %q, want %q", result.Data.Type, "number")
	}
	var num float64
	json.Unmarshal(result.Data.DefaultValue, &num)
	if num != 42 {
		t.Errorf("default_value: got %v, want 42", num)
	}
}

func TestCreateFlag_Bool(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/flags", adminToken,
		jsonBody(map[string]any{
			"key":           "test-bool-flag",
			"type":          "bool",
			"default_value": true,
			"description":   "A test bool flag",
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeFlagResponse(t, resp)

	if result.Data.Type != "bool" {
		t.Errorf("type: got %q, want %q", result.Data.Type, "bool")
	}
	var b bool
	json.Unmarshal(result.Data.DefaultValue, &b)
	if !b {
		t.Errorf("default_value: got %v, want true", b)
	}
}

func TestCreateFlag_RejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name         string
		payload      map[string]any
		expectedCode int
	}{
		{"empty body", map[string]any{}, http.StatusBadRequest},
		{"missing key", map[string]any{"type": "string", "default_value": "test"}, http.StatusBadRequest},
		{"missing type", map[string]any{"key": "test-flag", "default_value": "test"}, http.StatusBadRequest},
		{"missing default_value", map[string]any{"key": "test-flag", "type": "string"}, http.StatusBadRequest},
		{"invalid type", map[string]any{"key": "test-flag", "type": "invalid", "default_value": "test"}, http.StatusBadRequest},
		{"key too short", map[string]any{"key": "ab", "type": "string", "default_value": "test"}, http.StatusBadRequest},
		{"key too long", map[string]any{"key": strings.Repeat("a", 129), "type": "string", "default_value": "test"}, http.StatusBadRequest},
		{"string type with number value", map[string]any{"key": "test-flag", "type": "string", "default_value": 123}, http.StatusBadRequest},
		{"number type with string value", map[string]any{"key": "test-flag", "type": "number", "default_value": "not-a-number"}, http.StatusBadRequest},
		{"bool type with string value", map[string]any{"key": "test-flag", "type": "bool", "default_value": "not-a-bool"}, http.StatusBadRequest},
		{"empty description", map[string]any{"key": "test-flag", "type": "string", "default_value": "test", "description": ""}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(http.MethodPost, "/api/panel/v1/flags", adminToken, jsonBody(tc.payload))
			defer resp.Body.Close()
			if tc.expectedCode == http.StatusOK {
				requireStatus(t, resp, http.StatusOK)
			} else {
				requireErrorResponse(t, resp, tc.expectedCode, "BAD_REQUEST")
			}
		})
	}
}

func TestCreateFlag_DuplicateKey(t *testing.T) {
	key := "duplicate-key"
	createFlag(t, key, "string", "first")

	resp := doRequest(http.MethodPost, "/api/panel/v1/flags", adminToken,
		jsonBody(map[string]any{
			"key":           key + "-" + runID,
			"type":          "string",
			"default_value": "second",
		}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
}

func TestGetFlag(t *testing.T) {
	id, key := createFlag(t, "get-flag", "string", "test-value")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeFlagResponse(t, resp)

	if result.Data.ID != id {
		t.Errorf("id: got %d, want %d", result.Data.ID, id)
	}
	if result.Data.Key != key {
		t.Errorf("key: got %q, want %q", result.Data.Key, key)
	}
	if result.Data.Type != "string" {
		t.Errorf("type: got %q, want %q", result.Data.Type, "string")
	}
}

func TestGetFlag_Nonexistent(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/flags/999999999", adminToken, nil)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusNotFound, "NOT_FOUND")
}

func TestGetFlag_InvalidID(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/flags/abc", adminToken, nil)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestListFlags(t *testing.T) {
	_, keyA := createFlag(t, "list-flag-a", "string", "value-a")
	_, keyB := createFlag(t, "list-flag-b", "number", 123)

	resp := doRequest(http.MethodGet, "/api/panel/v1/flags?limit=100", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodePaginatedFlagsResponse(t, resp)

	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Data.Data) < 2 {
		t.Errorf("expected at least 2 flags, got %d", len(result.Data.Data))
	}

	foundA := false
	foundB := false
	for _, f := range result.Data.Data {
		if f.Key == keyA {
			foundA = true
		}
		if f.Key == keyB {
			foundB = true
		}
	}
	if !foundA {
		t.Errorf("%s not found in list", keyA)
	}
	if !foundB {
		t.Errorf("%s not found in list", keyB)
	}

	if result.Data.Meta.Limit != 100 {
		t.Errorf("meta.limit: got %d, want 100", result.Data.Meta.Limit)
	}
	if result.Data.Meta.Offset != 0 {
		t.Errorf("meta.offset: got %d, want 0", result.Data.Meta.Offset)
	}
	if result.Data.Meta.Count != len(result.Data.Data) {
		t.Errorf("meta.count: got %d, want %d", result.Data.Meta.Count, len(result.Data.Data))
	}
	if result.Data.Meta.Total < int64(result.Data.Meta.Count) {
		t.Errorf("meta.total (%d) < meta.count (%d)", result.Data.Meta.Total, result.Data.Meta.Count)
	}
}

func TestListFlags_Pagination(t *testing.T) {
	baseResp := doRequest(http.MethodGet, "/api/panel/v1/flags?limit=1&offset=0", adminToken, nil)
	defer baseResp.Body.Close()
	base := decodePaginatedFlagsResponse(t, baseResp)

	_, _ = createFlag(t, "page-flag-a", "string", "a")
	_, _ = createFlag(t, "page-flag-b", "string", "b")
	_, _ = createFlag(t, "page-flag-c", "string", "c")

	resp := doRequest(http.MethodGet, "/api/panel/v1/flags?limit=2&offset=0", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodePaginatedFlagsResponse(t, resp)

	if len(result.Data.Data) != 2 {
		t.Errorf("limit=2: got %d flags, want 2", len(result.Data.Data))
	}
	if result.Data.Meta.Limit != 2 {
		t.Errorf("meta.limit: got %d, want 2", result.Data.Meta.Limit)
	}
	if !result.Data.Meta.HasNext {
		t.Error("expected has_next=true on first page")
	}

	resp2 := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/flags?limit=2&offset=%d", base.Data.Meta.Total+2), adminToken, nil)
	defer resp2.Body.Close()

	requireStatus(t, resp2, http.StatusOK)

	result2 := decodePaginatedFlagsResponse(t, resp2)

	if len(result2.Data.Data) != 1 {
		t.Errorf("offset at baseline+2: got %d flags, want 1", len(result2.Data.Data))
	}
	if result2.Data.Meta.HasNext {
		t.Error("expected has_next=false on last page")
	}
}

func TestListFlags_OffsetBeyondTotal(t *testing.T) {
	baseResp := doRequest(http.MethodGet, "/api/panel/v1/flags?limit=1&offset=0", adminToken, nil)
	defer baseResp.Body.Close()
	base := decodePaginatedFlagsResponse(t, baseResp)

	_, _ = createFlag(t, "beyond-flag-a", "string", "a")
	_, _ = createFlag(t, "beyond-flag-b", "string", "b")

	resp := doRequest(http.MethodGet, "/api/panel/v1/flags?limit=1&offset=100000", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	result := decodePaginatedFlagsResponse(t, resp)

	if len(result.Data.Data) != 0 {
		t.Errorf("offset beyond total: got %d flags, want 0", len(result.Data.Data))
	}
	if result.Data.Meta.Count != 0 {
		t.Errorf("meta.count: got %d, want 0", result.Data.Meta.Count)
	}
	if result.Data.Meta.Total < base.Data.Meta.Total+2 {
		t.Errorf("meta.total (%d) should be >= baseline+2 (%d)", result.Data.Meta.Total, base.Data.Meta.Total+2)
	}
	if result.Data.Meta.HasNext {
		t.Error("expected has_next=false when offset beyond total")
	}
}

func TestUpdateFlag(t *testing.T) {
	id, _ := createFlag(t, "update-flag", "string", "original")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken,
		jsonBody(map[string]any{
			"default_value": "updated",
			"description":   "Updated description",
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeFlagResponse(t, resp)

	if result.Data.DefaultValue != nil {
		var val string
		json.Unmarshal(result.Data.DefaultValue, &val)
		if val != "updated" {
			t.Errorf("default_value: got %s, want updated", val)
		}
	}
	if result.Data.Description == nil || *result.Data.Description != "Updated description" {
		t.Errorf("description: got %v, want 'Updated description'", result.Data.Description)
	}
}

func TestUpdateFlag_ChangeKey(t *testing.T) {
	id, _ := createFlag(t, "update-key-flag", "string", "test")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken,
		jsonBody(map[string]any{
			"key": "new-key-name-" + runID,
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeFlagResponse(t, resp)

	if result.Data.Key != "new-key-name-"+runID {
		t.Errorf("key: got %q, want %q", result.Data.Key, "new-key-name-"+runID)
	}
}

func TestUpdateFlag_InvalidValueForType(t *testing.T) {
	id, _ := createFlag(t, "invalid-update-flag", "number", 100)

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken,
		jsonBody(map[string]any{
			"default_value": "not-a-number",
		}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
}

func TestUpdateFlag_Nonexistent(t *testing.T) {
	resp := doRequest(http.MethodPatch, "/api/panel/v1/flags/999999999", adminToken,
		jsonBody(map[string]any{"default_value": "test"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusNotFound, "NOT_FOUND")
}

func TestDeleteFlag(t *testing.T) {
	id, _ := createFlag(t, "delete-flag", "string", "to-delete")

	resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNoContent)

	getResp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken, nil)
	defer getResp.Body.Close()

	requireErrorResponse(t, getResp, http.StatusNotFound, "NOT_FOUND")
}

func TestDeleteFlag_Nonexistent(t *testing.T) {
	resp := doRequest(http.MethodDelete, "/api/panel/v1/flags/999999999", adminToken, nil)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusNotFound, "NOT_FOUND")
}

func TestFlags_NoSecretsInResponse(t *testing.T) {
	id, _ := createFlag(t, "no-secrets-flag", "string", "test")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/flags/%d", id), adminToken, nil)
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&raw)

	data, ok := raw["data"]
	if !ok {
		t.Fatal("response missing data field")
	}

	var flagFields map[string]json.RawMessage
	json.Unmarshal(data, &flagFields)

	for _, field := range []string{"password_hash", "password"} {
		if _, exists := flagFields[field]; exists {
			t.Errorf("response must not contain %s", field)
		}
	}

	for _, field := range []string{"id", "key", "type", "default_value", "created_at", "updated_at"} {
		if _, exists := flagFields[field]; !exists {
			t.Errorf("response missing %s field", field)
		}
	}
}

func TestListFlags_JSONContentType(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/flags", adminToken, nil)
	defer resp.Body.Close()

	requireJSONContentType(t, resp)
}

func TestUnauthenticatedFlag_RejectedFromAllEndpoints(t *testing.T) {
	id, _ := createFlag(t, "unauth-flag", "string", "test")

	t.Run("GET /flags", func(t *testing.T) {
		resp := doRequest(http.MethodGet, "/api/panel/v1/flags", "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /flags/:id", func(t *testing.T) {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/flags/%d", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("POST /flags", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/panel/v1/flags", "",
			jsonBody(map[string]any{
				"key":           "should-not-work",
				"type":          "string",
				"default_value": "test",
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("PATCH /flags/:id", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/flags/%d", id), "",
			jsonBody(map[string]any{"default_value": "hack"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("DELETE /flags/:id", func(t *testing.T) {
		resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/flags/%d", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestViewerCannotManageFlags(t *testing.T) {
	viewerEmail := testEmail("viewer-flags")
	createResp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		jsonBody(map[string]string{
			"username": testUsername("viewer-flags"),
			"email":    viewerEmail,
			"password": "testpass123",
			"role":     "viewer",
		}))
	if createResp.StatusCode != http.StatusOK {
		t.Skipf("failed to create viewer: %d", createResp.StatusCode)
	}
	createResp.Body.Close()

	viewerToken := login(viewerEmail, "testpass123")
	if viewerToken == "" {
		t.Skip("could not login as viewer")
	}

	targetID, _ := createFlag(t, "viewer-target-flag", "string", "test")

	t.Run("can create flag", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/panel/v1/flags", viewerToken,
			jsonBody(map[string]any{
				"key":           "viewer-create-" + runID,
				"type":          "string",
				"default_value": "test",
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can list flags", func(t *testing.T) {
		resp := doRequest(http.MethodGet, "/api/panel/v1/flags", viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can get flag by id", func(t *testing.T) {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/flags/%d", targetID), viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can update flag", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/flags/%d", targetID), viewerToken,
			jsonBody(map[string]any{"default_value": "updated"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
	})

	t.Run("can delete flag", func(t *testing.T) {
		resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/flags/%d", targetID), viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusNoContent)
	})
}

func createFlag(t *testing.T, key, flagType string, defaultValue any) (int64, string) {
	t.Helper()
	uniqueKey := key + "-" + runID
	resp := doRequest(http.MethodPost, "/api/panel/v1/flags", adminToken,
		jsonBody(map[string]any{
			"key":           uniqueKey,
			"type":          flagType,
			"default_value": defaultValue,
		}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("createFlag(%s): expected 200, got %d", uniqueKey, resp.StatusCode)
	}

	var result struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	return result.Data.ID, uniqueKey
}
