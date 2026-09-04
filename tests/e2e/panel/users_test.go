package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

func TestGetMe(t *testing.T) {
	resp := getMe(t, adminToken)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	requireJSONContentType(t, resp)

	result := decodeMeResponse(t, resp)

	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Data.ID == "" {
		t.Error("expected positive user id")
	}
	if result.Data.Role != "admin" {
		t.Errorf("role: got %q, want %q", result.Data.Role, "admin")
	}
}

func TestGetMe_Unauthenticated(t *testing.T) {
	resp := getMe(t, "")
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusUnauthorized)
}

func TestGetMe_NoSecretsInResponse(t *testing.T) {
	resp := getMe(t, adminToken)
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
	resp := getMe(t, "not-a-real-jwt")
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateAndListUser(t *testing.T) {
	id, email := createUser(t, "viewer", "list-user")

	listResp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=100", adminToken, nil)
	defer listResp.Body.Close()

	requireStatus(t, listResp, http.StatusOK)
	requireJSONContentType(t, listResp)

	result := decodePaginatedUsersResponse(t, listResp)

	if !result.Success {
		t.Error("expected success=true")
	}

	found := false
	for _, u := range result.Data {
		if u.ID == id && u.Email == email {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created user (id=%s, email=%s) not found in list", id, email)
	}
}

func TestListUsers_Pagination(t *testing.T) {
	createUser(t, "viewer", "page-a")
	createUser(t, "viewer", "page-b")

	resp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=1&offset=0", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodePaginatedUsersResponse(t, resp)

	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Data) != 1 {
		t.Errorf("limit=1: got %d users, want 1", len(result.Data))
	}
	if result.Meta.Limit != 1 {
		t.Errorf("meta.limit: got %d, want 1", result.Meta.Limit)
	}
	if result.Meta.Offset != 0 {
		t.Errorf("meta.offset: got %d, want 0", result.Meta.Offset)
	}
	if result.Meta.Count != 1 {
		t.Errorf("meta.count: got %d, want 1", result.Meta.Count)
	}
	if !result.Meta.HasNext {
		t.Error("expected has_next=true on first page with more results")
	}
}

func TestListUsers_Pagination_HasNextFalseOnFinalPage(t *testing.T) {
	createUser(t, "viewer", "final-a")
	createUser(t, "viewer", "final-b")

	// Get total count first
	firstResp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=1&offset=0", adminToken, nil)
	defer firstResp.Body.Close()
	first := decodePaginatedUsersResponse(t, firstResp)

	// Fetch the last page (offset = total - 1)
	lastOffset := int(first.Meta.Total) - 1
	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users?limit=1&offset=%d", lastOffset), adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	result := decodePaginatedUsersResponse(t, resp)

	if len(result.Data) != 1 {
		t.Errorf("final page: got %d users, want 1", len(result.Data))
	}
	if result.Meta.Count != 1 {
		t.Errorf("meta.count: got %d, want 1", result.Meta.Count)
	}
	if result.Meta.HasNext {
		t.Error("expected has_next=false on final page")
	}
}

func TestListUsers_Pagination_OffsetBeyondTotal(t *testing.T) {
	// Get baseline total
	baseResp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=1&offset=0", adminToken, nil)
	defer baseResp.Body.Close()
	base := decodePaginatedUsersResponse(t, baseResp)

	// Create two more users
	id1, _ := createUser(t, "viewer", "beyond-a")
	id2, _ := createUser(t, "viewer", "beyond-b")
	_ = id1
	_ = id2

	// Request offset far beyond total
	resp := doRequest(http.MethodGet, "/api/panel/v1/users?limit=1&offset=100000", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	result := decodePaginatedUsersResponse(t, resp)

	if len(result.Data) != 0 {
		t.Errorf("offset beyond total: got %d users, want 0", len(result.Data))
	}
	if result.Meta.Count != 0 {
		t.Errorf("meta.count: got %d, want 0", result.Meta.Count)
	}
	if result.Meta.Total < base.Meta.Total+2 {
		t.Errorf("meta.total (%d) should be >= baseline+2 (%d)", result.Meta.Total, base.Meta.Total+2)
	}
	if result.Meta.HasNext {
		t.Error("expected has_next=false when offset beyond total")
	}
}

func TestListUsers_DefaultLimit(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/users", adminToken, nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodePaginatedUsersResponse(t, resp)

	if !result.Success {
		t.Error("expected success=true")
	}
	if len(result.Data) == 0 {
		t.Error("expected at least one user (bootstrap admin)")
	}
	if result.Meta.Limit != 20 {
		t.Errorf("meta.limit: got %d, want 20", result.Meta.Limit)
	}
	if result.Meta.Offset != 0 {
		t.Errorf("meta.offset: got %d, want 0", result.Meta.Offset)
	}
	if result.Meta.Count != len(result.Data) {
		t.Errorf("meta.count: got %d, want %d", result.Meta.Count, len(result.Data))
	}
	if result.Meta.Total < int64(result.Meta.Count) {
		t.Errorf("meta.total (%d) < meta.count (%d)", result.Meta.Total, result.Meta.Count)
	}
	if result.Meta.HasNext != (int64(result.Meta.Offset)+int64(result.Meta.Count) < result.Meta.Total) {
		t.Errorf("meta.has_next inconsistent: offset=%d count=%d total=%d has_next=%v", result.Meta.Offset, result.Meta.Count, result.Meta.Total, result.Meta.HasNext)
	}
}

func TestCreateUser_AllValidRoles(t *testing.T) {
	roles := []string{"admin", "experimenter", "approver", "viewer"}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			id, _ := createUser(t, role, "role-"+role)
			if id == "" {
				t.Errorf("expected positive id for role %s", role)
			}
		})
	}
}

func TestGetUserByID(t *testing.T) {
	id, email := createUser(t, "viewer", "getid-user")

	resp := getUser(t, id)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeUserResponse(t, resp)

	if result.Data.ID != id {
		t.Errorf("id: got %s, want %s", result.Data.ID, id)
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
		{"nonexistent id", "/api/panel/v1/users/0198f4c0-dead-7000-8000-000000000001", http.StatusNotFound, "NOT_FOUND"},
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
	resp := updateUser(t, id, jsonBody(map[string]string{"email": newEmail}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeUserResponse(t, resp)

	if result.Data.Email != newEmail {
		t.Errorf("email: got %q, want %q", result.Data.Email, newEmail)
	}
}

func TestUpdateUserRole(t *testing.T) {
	id, _ := createUser(t, "viewer", "update-role-target")

	resp := updateUser(t, id, jsonBody(map[string]string{"role": "approver"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	result := decodeUserResponse(t, resp)

	if result.Data.Role != "approver" {
		t.Errorf("role: got %q, want %q", result.Data.Role, "approver")
	}
}

func TestUpdateUser_NonexistentID(t *testing.T) {
	resp := doRequest(http.MethodPatch, "/api/panel/v1/users/0198f4c0-dead-7000-8000-000000000001", adminToken,
		jsonBody(map[string]string{"email": "new@test.local"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNotFound)
}

func TestDeleteUser(t *testing.T) {
	id, _ := createUser(t, "viewer", "delete-user")

	resp := deleteUser(t, id)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNoContent)

	getResp := getUser(t, id)
	defer getResp.Body.Close()

	requireStatus(t, getResp, http.StatusNotFound)
}

func TestDeleteUser_NonexistentID(t *testing.T) {
	resp := deleteUser(t, "0198f4c0-dead-7000-8000-000000000001")
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNotFound)
}

func TestSelfDeleteForbidden(t *testing.T) {
	meResp := getMe(t, adminToken)
	defer meResp.Body.Close()
	me := decodeMeResponse(t, meResp)

	resp := deleteUser(t, me.Data.ID)
	defer resp.Body.Close()

	result := requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")
	if !strings.Contains(result.Error.Message, "cannot delete") {
		t.Errorf("error message should mention self-delete, got %q", result.Error.Message)
	}
}

func TestSelfRoleChangeForbidden(t *testing.T) {
	meResp := getMe(t, adminToken)
	defer meResp.Body.Close()
	me := decodeMeResponse(t, meResp)

	resp := updateUser(t, me.Data.ID, jsonBody(map[string]string{"role": "viewer"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")
}

func TestAdminCanDeleteAnotherAdmin(t *testing.T) {
	id, _ := createUser(t, "admin", "del-other-admin")

	resp := deleteUser(t, id)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusNoContent)
}

func TestAdminCanDemoteAnotherAdmin(t *testing.T) {
	id, _ := createUser(t, "admin", "demote-other-admin")

	resp := updateUser(t, id, jsonBody(map[string]string{"role": "viewer"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	email := testEmail("dup-email")
	createUser(t, "viewer", "dup-email")

	resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		jsonBody(map[string]string{
			"full_name": testFullName("dup-email-2"),
			"email":    email,
			"password": "testpass123",
			"role":     "viewer",
		}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
}

func TestCreateUser_DuplicateFullName(t *testing.T) {
	fullName := testFullName("dup-user")
	createUser(t, "viewer", "dup-user")

	resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken,
		jsonBody(map[string]string{
			"full_name": fullName,
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
			"full_name": testFullName("viewer-cant-manage"),
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
				"full_name": testFullName("should-not-exist"),
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
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%s", targetID), viewerToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("cannot update user", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%s", targetID), viewerToken,
			jsonBody(map[string]string{"email": "hacker@test.local"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("cannot delete user", func(t *testing.T) {
		resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%s", targetID), viewerToken, nil)
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
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/panel/v1/users/%s", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("POST /users", func(t *testing.T) {
		resp := doRequest(http.MethodPost, "/api/panel/v1/users", "",
			jsonBody(map[string]string{
				"full_name": testFullName("should-not-work"),
				"email":    testEmail("should-not-work"),
				"password": "testpass123",
				"role":     "viewer",
			}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("PATCH /users/:id", func(t *testing.T) {
		resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/panel/v1/users/%s", id), "",
			jsonBody(map[string]string{"email": "hacker@test.local"}))
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("DELETE /users/:id", func(t *testing.T) {
		resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/panel/v1/users/%s", id), "", nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestCreateUser_RejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]string
	}{
		{"empty body", map[string]string{}},
		{"missing email", map[string]string{"full_name": "valid-user", "password": "testpass123", "role": "viewer"}},
		{"missing password", map[string]string{"full_name": "valid-user", "email": "valid@test.local", "role": "viewer"}},
		{"missing role", map[string]string{"full_name": "valid-user", "email": "valid@test.local", "password": "testpass123"}},
		{"missing full_name", map[string]string{"email": "valid@test.local", "password": "testpass123", "role": "viewer"}},
		{"invalid email format", map[string]string{"full_name": "valid-user", "email": "not-an-email", "password": "testpass123", "role": "viewer"}},
		{"empty role", map[string]string{"full_name": testFullName("bad-role"), "email": testEmail("bad-role"), "password": "testpass123", "role": ""}},
		{"unknown role", map[string]string{"full_name": testFullName("bad-role"), "email": testEmail("bad-role"), "password": "testpass123", "role": "superadmin"}},
		{"capitalized role", map[string]string{"full_name": testFullName("bad-role"), "email": testEmail("bad-role"), "password": "testpass123", "role": "Admin"}},
		{"empty full_name", map[string]string{"full_name": "", "email": testEmail("short-username"), "password": "testpass123", "role": "viewer"}},
		{"full_name too long", map[string]string{"full_name": strings.Repeat("a", 257), "email": testEmail("long-username"), "password": "testpass123", "role": "viewer"}},
		{"password too short", map[string]string{"full_name": testFullName("short-pass"), "email": testEmail("short-pass"), "password": "short", "role": "viewer"}},
		{"all fields invalid", map[string]string{"full_name": "", "email": "not-an-email", "password": "short", "role": "bogus"}},
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

	resp := getUser(t, id)
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

	for _, field := range []string{"id", "full_name", "email", "role", "created_at", "updated_at"} {
		if _, exists := userFields[field]; !exists {
			t.Errorf("response missing %s field", field)
		}
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
