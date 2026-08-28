package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
)

func TestAuth_Login_ValidCredentialsReturnsToken(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
		} `json:"data"`
	}
	if err := unmarshalJSON(body, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Data.Token == "" {
		t.Error("expected non-empty token")
	}
	if result.Data.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
}

func TestAuth_Login_TokenUsableOnProtectedEndpoint(t *testing.T) {
	token := login("root@labp.net", "root!@#$")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

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
	if result.Data.Email != "root@labp.net" {
		t.Errorf("email: got %q, want %q", result.Data.Email, "root@labp.net")
	}
	if result.Data.Role != "admin" {
		t.Errorf("role: got %q, want %q", result.Data.Role, "admin")
	}
}

func TestAuth_Login_ResponseEnvelope(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var raw struct {
		Success bool `json:"success"`
		Data    struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
		} `json:"data"`
	}
	if err := unmarshalJSON(body, &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !raw.Success {
		t.Error("expected success=true")
	}
	if raw.Data.Token == "" {
		t.Error("response envelope missing data.token")
	}
	if raw.Data.ExpiresAt == "" {
		t.Error("response envelope missing data.expires_at")
	}
}

func TestAuth_Login_InvalidPassword(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "wrongpassword"}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeJSON(resp, &result)

	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error.Code != "UNAUTHORIZED" {
		t.Errorf("error code: got %q, want %q", result.Error.Code, "UNAUTHORIZED")
	}
	if result.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestAuth_Login_NonexistentUser(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "nobody@test.local", "password": "whatever"}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(resp, &result)

	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error.Code != "UNAUTHORIZED" {
		t.Errorf("error code: got %q, want %q", result.Error.Code, "UNAUTHORIZED")
	}
}

func TestAuth_Login_NoUserEnumeration(t *testing.T) {
	bodyWrong := readBody(t, doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "wrong"})))
	bodyMissing := readBody(t, doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "nobody@test.local", "password": "wrong"})))

	var errWrong, errMissing struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	unmarshalJSON(bodyWrong, &errWrong)
	unmarshalJSON(bodyMissing, &errMissing)

	if errWrong.Error.Message != errMissing.Error.Message {
		t.Errorf("error messages differ: wrong=%q missing=%q (user enumeration possible)", errWrong.Error.Message, errMissing.Error.Message)
	}
}

func TestAuth_Login_MalformedJSON(t *testing.T) {
	resp := doRequestWithRawBody(http.MethodPost, "/api/panel/v1/login", "",
		strings.NewReader("{not json}"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(resp, &result)

	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error.Code != "BAD_REQUEST" {
		t.Errorf("error code: got %q, want %q", result.Error.Code, "BAD_REQUEST")
	}
}

func TestAuth_Login_EmptyBody(t *testing.T) {
	resp := doRequestWithRawBody(http.MethodPost, "/api/panel/v1/login", "",
		strings.NewReader(""))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestAuth_Login_MissingEmail(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"password": "root!@#$"}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(resp, &result)

	if result.Error.Code != "BAD_REQUEST" {
		t.Errorf("error code: got %q, want %q", result.Error.Code, "BAD_REQUEST")
	}
}

func TestAuth_Login_MissingPassword(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net"}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(resp, &result)

	if result.Error.Code != "BAD_REQUEST" {
		t.Errorf("error code: got %q, want %q", result.Error.Code, "BAD_REQUEST")
	}
}

func TestAuth_Login_InvalidEmailFormat(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "not-an-email", "password": "root!@#$"}))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestAuth_Middleware_NoAuthHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/panel/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Middleware_EmptyAuthHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/panel/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Middleware_WrongScheme(t *testing.T) {
	token := login("root@labp.net", "root!@#$")
	if token == "" {
		t.Fatal("login failed")
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/panel/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Basic "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Middleware_BearerWithNoToken(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/panel/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer ")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Middleware_BearerOnly(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/panel/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Middleware_GarbageToken(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", "totally.invalid.token", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Middleware_WrongSignature(t *testing.T) {
	// Generate a valid-looking JWT with a different secret
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxIiwicm9sZSI6ImFkbWluIiwiaWF0IjoxNjAwMDAwMDAwLCJleHAiOjQxMDI0NDQ4MDB9." +
		"bWFsb21lZHNpZ25hdHVyZXZhbGlkZm9ybWF0"

	resp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_Session_RevokedTokenRejected(t *testing.T) {
	// Create a user and log in
	_, email := createUser(t, "viewer", "revoke-test")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	// Verify token works
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-revoke check: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete the session key from Redis directly
	if err := deleteSessionFromRedis(email, token); err != nil {
		t.Fatalf("delete session from redis: %v", err)
	}

	// Verify token is now rejected
	resp = doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-revoke: got %d, want 401", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeJSON(resp, &result)

	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error.Code != "UNAUTHORIZED" {
		t.Errorf("error code: got %q, want %q", result.Error.Code, "UNAUTHORIZED")
	}
}

func TestAuth_Session_RevokedTokenNoUserLeakage(t *testing.T) {
	_, email := createUser(t, "viewer", "leak-test")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	deleteSessionFromRedis(email, token)

	resp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if strings.Contains(s, email) {
		t.Error("revoked token response must not contain user email")
	}
	if strings.Contains(s, "leak-test") {
		t.Error("revoked token response must not contain username")
	}
}

func TestAuth_Security_ContentTypeJSON(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

func TestAuth_Security_ErrorContentTypeJSON(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "wrong"}))
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

func TestAuth_Security_NoStackTracesInError(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "wrong"}))
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	sensitive := []string{"goroutine", "stack trace", "panic", ".go:", "pkg/", "internal/"}
	for _, pattern := range sensitive {
		if strings.Contains(s, pattern) {
			t.Errorf("error response contains %q: possible info leak", pattern)
		}
	}
}

func TestAuth_Security_NoPasswordInLoginResponse(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	leaked := []string{"root!@#$", "root\\!@\\#$", "password_hash", "hashed_password"}
	for _, pattern := range leaked {
		if strings.Contains(s, pattern) {
			t.Errorf("login response leaks sensitive data: contains %q", pattern)
		}
	}
}

func TestAuth_Security_AllErrorResponseCodes(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		token    string
		body     io.Reader
		wantCode int
	}{
		{"login bad credentials", http.MethodPost, "/api/panel/v1/login", "", jsonBody(map[string]string{"email": "x@x.com", "password": "x"}), http.StatusUnauthorized},
		{"login missing fields", http.MethodPost, "/api/panel/v1/login", "", jsonBody(map[string]string{"email": "x@x.com"}), http.StatusBadRequest},
		{"login malformed json", http.MethodPost, "/api/panel/v1/login", "", strings.NewReader("{bad"), http.StatusBadRequest},
		{"me no token", http.MethodGet, "/api/panel/v1/me", "", nil, http.StatusUnauthorized},
		{"me bad token", http.MethodGet, "/api/panel/v1/me", "garbage", nil, http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var resp *http.Response
			if tc.body != nil {
				resp = doRequestWithRawBody(tc.method, tc.path, tc.token, tc.body)
			} else {
				resp = doRequest(tc.method, tc.path, tc.token, nil)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantCode {
				t.Errorf("status: got %d, want %d", resp.StatusCode, tc.wantCode)
			}

			if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
				t.Errorf("Content-Type: got %q, want application/json", resp.Header.Get("Content-Type"))
			}
		})
	}
}

func TestAuth_Login_AdminTokenGrantsAdminAccess(t *testing.T) {
	token := login("root@labp.net", "root!@#$")
	if token == "" {
		t.Fatal("login failed")
	}

	// /me works for any authenticated role
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d", resp.StatusCode)
	}

	// admin-only /users endpoint works
	resp2 := doRequest(http.MethodGet, "/api/panel/v1/users", token, nil)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /users (admin), got %d", resp2.StatusCode)
	}
}

func TestAuth_Login_NonAdminTokenRejectedByAdminEndpoint(t *testing.T) {
	_, email := createUser(t, "viewer", "non-admin-test")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	// /me works for any authenticated role
	resp := doRequest(http.MethodGet, "/api/panel/v1/me", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d", resp.StatusCode)
	}

	// admin-only /users endpoint rejected for viewer
	resp2 := doRequest(http.MethodGet, "/api/panel/v1/users", token, nil)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for /users (viewer), got %d", resp2.StatusCode)
	}
}

// --- helpers ---

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

func doRequestWithRawBody(method, path, token string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		panic(fmt.Sprintf("create request: %v", err))
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("do request %s %s: %v", method, path, err))
	}
	return resp
}

func deleteSessionFromRedis(email, token string) error {
	// We need the user ID to compute the session key.
	// Login returned a token but didn't give us the ID directly,
	// so we query the DB for it.
	ctx := tCtx()
	pool, err := pgxpool.New(ctx, e2ePostgresDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	var userID int64
	err = pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}

	sum := sha256.Sum256([]byte(token))
	key := fmt.Sprintf("labp:panel:auth:jwt:%d:%s", userID, hex.EncodeToString(sum[:]))

	redisClient, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{"localhost:6379"},
		SelectDB:    15,
	})
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer redisClient.Close()

	return redisClient.Do(ctx, redisClient.B().Del().Key(key).Build()).Error()
}

func tCtx() context.Context {
	return context.Background()
}
