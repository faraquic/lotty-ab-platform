package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

func TestGetMe(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	requireJSONContentType(t, resp)

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Data.ID < 1 {
		t.Error("expected positive user id")
	}
	if result.Data.Role != "admin" {
		t.Errorf("role: got %q, want %q", result.Data.Role, "admin")
	}
}

func TestGetMe_Unauthenticated(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", "", nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusUnauthorized)
}

func TestGetMe_NoSecretsInResponse(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", adminToken, nil)
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&raw)

	data, ok := raw["data"]
	if !ok {
		t.Fatal("response missing data field")
	}

	var userFields map[string]json.RawMessage
	json.Unmarshal(data, &userFields)

	if _, exists := userFields["password_hash"]; exists {
		t.Error("/me must not expose password_hash")
	}
	if _, exists := userFields["password"]; exists {
		t.Error("/me must not expose password")
	}
	if _, exists := userFields["id"]; !exists {
		t.Error("/me missing id field")
	}
	if _, exists := userFields["role"]; !exists {
		t.Error("/me missing role field")
	}
}

func TestGetMe_InvalidToken(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", "not-a-real-jwt", nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateAndListUser(t *testing.T) {
	id, email := createUser(t, "viewer", "list-user")

	listResp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=100", adminToken, nil)
	defer listResp.Body.Close()

	requireStatus(t, listResp, http.StatusOK)
	requireJSONContentType(t, listResp)

	var listResult struct {
		Success bool `json:"success"`
		Data    []struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}
	decodeJSON(listResp, &listResult)

	if !listResult.Success {
		t.Error("expected success=true")
	}

	found := false
	for _, u := range listResult.Data {
		if u.ID == id && u.Email == email {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created user (id=%d, email=%s) not found in list", id, email)
	}
}

func TestListUsers_Pagination(t *testing.T) {
	createUser(t, "viewer", "page-a")
	createUser(t, "viewer", "page-b")

	resp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=1&offset=0", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	var result struct {
		Success bool `json:"success"`
		Data    []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Data) != 1 {
		t.Errorf("limit=1: got %d users, want 1", len(result.Data))
	}
}

func TestListUsers_DefaultLimit(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/users", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	var result struct {
		Success bool `json:"success"`
		Data    []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Data) == 0 {
		t.Error("expected at least one user (bootstrap admin)")
	}
}

func TestCreateUser_AllValidRoles(t *testing.T) {
	roles := []string{"admin", "experimenter", "approver", "viewer"}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			id, _ := createUser(t, role, "role-"+role)
			if id < 1 {
				t.Errorf("expected positive id for role %s", role)
			}
		})
	}
}

func TestGetUserByID(t *testing.T) {
	id, email := createUser(t, "viewer", "getid-user")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	var result struct {
		Data struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	if result.Data.ID != id {
		t.Errorf("id: got %d, want %d", result.Data.ID, id)
	}
	if result.Data.Email != email {
		t.Errorf("email: got %q, want %q", result.Data.Email, email)
	}
}

func TestUsers_RejectsInvalidOrNonexistentIDs(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"non-numeric id", "/api/panel/v1/users/abc", http.StatusBadRequest, "BAD_REQUEST"},
		{"negative id", "/api/panel/v1/users/-1", http.StatusBadRequest, "BAD_REQUEST"},
		{"nonexistent id", "/api/panel/v1/users/999999999", http.StatusNotFound, "NOT_FOUND"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(http.MethodGet, tc.path, adminToken, nil)
			defer resp.Body.Close()
			requireErrorResponse(t, resp, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestUpdateUserEmail(t *testing.T) {
	id, _ := createUser(t, "viewer", "update-user")

	newEmail := fmt.Sprintf("updated-%s@test.local", runID)
	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken,
		jsonBody(map[string]string{"email": newEmail}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	var result struct {
		Data struct {
			Email string `json:"email"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	if result.Data.Email != newEmail {
		t.Errorf("email: got %q, want %q", result.Data.Email, newEmail)
	}
}

func TestUpdateUserRole(t *testing.T) {
	id, _ := createUser(t, "viewer", "update-role-target")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken,
		jsonBody(map[string]string{"role": "approver"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	var result struct {
		Data struct {
			Role string `json:"role"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	if result.Data.Role != "approver" {
		t.Errorf("role: got %q, want %q", result.Data.Role, "approver")
	}
}

func TestUpdateUser_NonexistentID(t *testing.T) {
	resp := doRequest(http.MethodPatch, "/api/panel/v1/users/999999999", adminToken,
		jsonBody(map[string]string{"email": "new@test.local"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNotFound)
}

func TestDeleteUser(t *testing.T) {
	id, _ := createUser(t, "viewer", "delete-user")

	resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNoContent)

	getResp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken, nil)
	defer getResp.Body.Close()

	requireStatus(t, getResp, http.StatusNotFound)
}

func TestDeleteUser_NonexistentID(t *testing.T) {
	resp := doRequest(http.MethodDelete, "/api/panel/v1/users/999999999", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNotFound)
}

func TestSelfDeleteForbidden(t *testing.T) {
	var me struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	meResp := doRequest(http.MethodGet, "/api/panel/v1/me", adminToken, nil)
	defer meResp.Body.Close()
	decodeJSON(meResp, &me)

	resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%d", me.Data.ID), adminToken, nil)
	defer resp.Body.Close()

	result := requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")
	if !strings.Contains(result.Error.Message, "cannot delete") {
		t.Errorf("error message should mention self-delete, got %q", result.Error.Message)
	}
}

func TestSelfRoleChangeForbidden(t *testing.T) {
	var me struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	meResp := doRequest(http.MethodGet, "/api/panel/v1/me", adminToken, nil)
	defer meResp.Body.Close()
	decodeJSON(meResp, &me)

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%d", me.Data.ID), adminToken,
		jsonBody(map[string]string{"role": "viewer"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")
}

func TestAdminCanDeleteAnotherAdmin(t *testing.T) {
	id, _ := createUser(t, "admin", "del-other-admin")

	resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNoContent)
}

func TestAdminCanDemoteAnotherAdmin(t *testing.T) {
	id, _ := createUser(t, "admin", "demote-other-admin")

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken,
		jsonBody(map[string]string{"role": "viewer"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	email := testEmail("dup-email")
	createUser(t, "viewer", "dup-email")

	resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		jsonBody(map[string]string{
			"username": testUsername("dup-email-2"),
			"email":    email,
			"password": "testpass123",
			"role":     "viewer",
		}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	username := testUsername("dup-user")
	createUser(t, "viewer", "dup-user")

	resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		jsonBody(map[string]string{
			"username": username,
			"email":    testEmail("dup-user-diff"),
			"password": "testpass123",
			"role":     "viewer",
		}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusConflict)
}

func TestViewerCannotManageUsers(t *testing.T) {
	viewerEmail := testEmail("viewer-cant-manage")
	createResp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		jsonBody(map[string]string{
			"username": testUsername("viewer-cant-manage"),
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

	targetID, _ := createUser(t, "viewer", "viewer-manage-target")

	t.Run("cannot create user", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/panel/v1/users", viewerToken,
			jsonBody(map[string]string{
				"username": testUsername("should-not-exist"),
				"email":    testEmail("should-not-exist"),
				"password": "testpass123",
				"role":     "viewer",
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("cannot list users", func(t *testing.T) {
		resp := doRequest(http.MethodGet, "/api/panel/v1/users", viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("cannot get user by id", func(t *testing.T) {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%d", targetID), viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("cannot update user", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%d", targetID), viewerToken,
			jsonBody(map[string]string{"email": "hacker@test.local"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("cannot delete user", func(t *testing.T) {
		resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%d", targetID), viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})
}

func TestUnauthenticatedUser_RejectedFromAllEndpoints(t *testing.T) {
	id, _ := createUser(t, "viewer", "unauth-target")

	t.Run("GET /users", func(t *testing.T) {
		resp := doRequest(http.MethodGet, "/api/panel/v1/users", "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("GET /users/:id", func(t *testing.T) {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%d", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("POST /users", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/panel/v1/users", "",
			jsonBody(map[string]string{
				"username": testUsername("should-not-work"),
				"email":    testEmail("should-not-work"),
				"password": "testpass123",
				"role":     "viewer",
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("PATCH /users/:id", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%d", id), "",
			jsonBody(map[string]string{"email": "hacker@test.local"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("DELETE /users/:id", func(t *testing.T) {
		resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%d", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestCreateUser_RejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]string
	}{
		{
			name:    "empty body",
			payload: map[string]string{},
		},
		{
			name:    "missing email",
			payload: map[string]string{"username": "valid-user", "password": "testpass123", "role": "viewer"},
		},
		{
			name:    "missing password",
			payload: map[string]string{"username": "valid-user", "email": "valid@test.local", "role": "viewer"},
		},
		{
			name:    "missing role",
			payload: map[string]string{"username": "valid-user", "email": "valid@test.local", "password": "testpass123"},
		},
		{
			name:    "missing username",
			payload: map[string]string{"email": "valid@test.local", "password": "testpass123", "role": "viewer"},
		},
		{
			name:    "invalid email format",
			payload: map[string]string{"username": "valid-user", "email": "not-an-email", "password": "testpass123", "role": "viewer"},
		},
		{
			name:    "empty role",
			payload: map[string]string{"username": testUsername("bad-role"), "email": testEmail("bad-role"), "password": "testpass123", "role": ""},
		},
		{
			name:    "unknown role",
			payload: map[string]string{"username": testUsername("bad-role"), "email": testEmail("bad-role"), "password": "testpass123", "role": "superadmin"},
		},
		{
			name:    "capitalized role",
			payload: map[string]string{"username": testUsername("bad-role"), "email": testEmail("bad-role"), "password": "testpass123", "role": "Admin"},
		},
		{
			name:    "username too short",
			payload: map[string]string{"username": "ab", "email": testEmail("short-username"), "password": "testpass123", "role": "viewer"},
		},
		{
			name:    "username too long",
			payload: map[string]string{"username": strings.Repeat("a", 65), "email": testEmail("long-username"), "password": "testpass123", "role": "viewer"},
		},
		{
			name:    "password too short",
			payload: map[string]string{"username": testUsername("short-pass"), "email": testEmail("short-pass"), "password": "short", "role": "viewer"},
		},
		{
			name: "all fields invalid",
			payload: map[string]string{
				"username": "x",
				"email":    "not-an-email",
				"password": "short",
				"role":     "bogus",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken, jsonBody(tc.payload))
			defer resp.Body.Close()
			requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
		})
	}
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		strings.NewReader("{invalid json"))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusBadRequest)
}

func TestCreateUser_NoSecretsInResponse(t *testing.T) {
	id, _ := createUser(t, "viewer", "no-secrets")

	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%d", id), adminToken, nil)
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&raw)

	data, ok := raw["data"]
	if !ok {
		t.Fatal("response missing data field")
	}

	var userFields map[string]json.RawMessage
	json.Unmarshal(data, &userFields)

	if _, exists := userFields["password_hash"]; exists {
		t.Error("response must not contain password_hash")
	}
	if _, exists := userFields["password"]; exists {
		t.Error("response must not contain password")
	}

	if _, exists := userFields["id"]; !exists {
		t.Error("response missing id field")
	}
	if _, exists := userFields["username"]; !exists {
		t.Error("response missing username field")
	}
	if _, exists := userFields["email"]; !exists {
		t.Error("response missing email field")
	}
	if _, exists := userFields["role"]; !exists {
		t.Error("response missing role field")
	}
	if _, exists := userFields["created_at"]; !exists {
		t.Error("response missing created_at field")
	}
	if _, exists := userFields["updated_at"]; !exists {
		t.Error("response missing updated_at field")
	}
}

func TestListUsers_NoSecretsInResponse(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=5", adminToken, nil)
	defer resp.Body.Close()

	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&raw)

	for i, user := range raw.Data {
		if _, exists := user["password_hash"]; exists {
			t.Errorf("user[%d] must not contain password_hash", i)
		}
		if _, exists := user["password"]; exists {
			t.Errorf("user[%d] must not contain password", i)
		}
	}
}

func TestListUsers_JSONContentType(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/users", adminToken, nil)
	defer resp.Body.Close()

	requireJSONContentType(t, resp)
}
