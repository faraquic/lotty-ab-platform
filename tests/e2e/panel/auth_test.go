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

func TestAuth_Login_ValidCredentials(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	requireJSONContentType(t, resp)

	result := decodeLoginResponse(t, resp)

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

	resp := getMe(t, token)
	defer resp.Body.Close()

	result := decodeMeResponse(t, resp)

	requireStatus(t, resp, http.StatusOK)
	if result.Data.ID == "" {
		t.Error("expected positive user id")
	}
	if result.Data.Email != "root@labp.net" {
		t.Errorf("email: got %q, want %q", result.Data.Email, "root@labp.net")
	}
	if result.Data.Role != "admin" {
		t.Errorf("role: got %q, want %q", result.Data.Role, "admin")
	}
}

func TestAuth_Login_InvalidPassword(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "wrongpassword"}))
	defer resp.Body.Close()

	result := requireErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED")
	if result.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestAuth_Login_NonexistentUser(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": "nobody@test.local", "password": "whatever"}))
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestAuth_Login_NoUserEnumeration(t *testing.T) {
	bodyWrong := readBody(t, doRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "wrong"})))
	bodyMissing := readBody(t, doRequest(http.MethodPost, "/api/v1/panel/login", "",
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

func TestAuth_Login_RejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name string
		send func(t *testing.T) *http.Response
	}{
		{"empty body", func(t *testing.T) *http.Response {
			return doRequestWithRawBody(http.MethodPost, "/api/v1/panel/login", "", strings.NewReader(""))
		}},
		{"malformed json", func(t *testing.T) *http.Response {
			return doRequestWithRawBody(http.MethodPost, "/api/v1/panel/login", "", strings.NewReader("{not json}"))
		}},
		{"missing email", func(t *testing.T) *http.Response {
			return doRequest(http.MethodPost, "/api/v1/panel/login", "",
				jsonBody(map[string]string{"password": "root!@#$"}))
		}},
		{"missing password", func(t *testing.T) *http.Response {
			return doRequest(http.MethodPost, "/api/v1/panel/login", "",
				jsonBody(map[string]string{"email": "root@labp.net"}))
		}},
		{"invalid email format", func(t *testing.T) *http.Response {
			return doRequest(http.MethodPost, "/api/v1/panel/login", "",
				jsonBody(map[string]string{"email": "not-an-email", "password": "root!@#$"}))
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := tc.send(t)
			defer resp.Body.Close()
			requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
		})
	}
}

func TestAuth_Middleware_RejectsInvalidAuthorization(t *testing.T) {
	cases := []struct {
		name      string
		setHeader bool
		value     string
	}{
		{"no header", false, ""},
		{"empty header", true, ""},
		{"wrong scheme", true, "Basic some-token"},
		{"bearer without token", true, "Bearer "},
		{"bearer only", true, "Bearer"},
		{"garbage token", true, "Bearer totally.invalid.token"},
		{
			"invalid signature",
			true,
			"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
				"eyJzdWIiOiIxIiwicm9sZSI6ImFkbWluIiwiaWF0IjoxNjAwMDAwMDAwLCJleHAiOjQxMDI0NDQ4MDB9." +
				"bWFsb21lZHNpZ25hdHVyZXZhbGlkZm9ybWF0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequestWithAuth(t, http.MethodGet, "/api/v1/panel/me", tc.setHeader, tc.value, nil)
			defer resp.Body.Close()
			requireErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED")
		})
	}
}

func TestAuth_Session_RevokedTokenRejected(t *testing.T) {
	_, email := createUser(t, "viewer", "revoke-test")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	resp := getMe(t, token)
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	if err := deleteSessionFromRedis(email, token); err != nil {
		t.Fatalf("delete session from redis: %v", err)
	}

	resp = getMe(t, token)
	defer resp.Body.Close()

	requireErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestAuth_Session_RevokedTokenNoUserLeakage(t *testing.T) {
	_, email := createUser(t, "viewer", "leak-test")
	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("login failed")
	}

	deleteSessionFromRedis(email, token)

	resp := getMe(t, token)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if strings.Contains(s, email) {
		t.Error("revoked token response must not contain user email")
	}
	if strings.Contains(s, "leak-test") {
		t.Error("revoked token response must not contain full_name")
	}
}

func TestAuth_Security_ContentTypeJSON(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	requireJSONContentType(t, resp)
}

func TestAuth_Security_NoStackTracesInError(t *testing.T) {
	resp := doRequest(http.MethodPost, "/api/v1/panel/login", "",
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
	resp := doRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": "root@labp.net", "password": "root!@#$"}))
	defer resp.Body.Close()

	requireNoSensitiveFields(t, resp, "root!@#$", "root\\!@\\#$", "password_hash", "hashed_password")
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
		{"login bad credentials", http.MethodPost, "/api/v1/panel/login", "", jsonBody(map[string]string{"email": "x@x.com", "password": "x"}), http.StatusUnauthorized},
		{"login missing fields", http.MethodPost, "/api/v1/panel/login", "", jsonBody(map[string]string{"email": "x@x.com"}), http.StatusBadRequest},
		{"login malformed json", http.MethodPost, "/api/v1/panel/login", "", strings.NewReader("{bad"), http.StatusBadRequest},
		{"me no token", http.MethodGet, "/api/v1/panel/me", "", nil, http.StatusUnauthorized},
		{"me bad token", http.MethodGet, "/api/v1/panel/me", "garbage", nil, http.StatusUnauthorized},
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

			requireStatus(t, resp, tc.wantCode)
			requireJSONContentType(t, resp)
		})
	}
}

func TestAuth_RoleBasedAccessControl(t *testing.T) {
	t.Run("admin can access admin endpoints", func(t *testing.T) {
		token := login("root@labp.net", "root!@#$")
		if token == "" {
			t.Fatal("login failed")
		}

		resp := getMe(t, token)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)

		resp2 := doRequest(http.MethodGet, "/api/v1/panel/users", token, nil)
		defer resp2.Body.Close()
		requireStatus(t, resp2, http.StatusOK)
	})

	t.Run("non-admin rejected from admin endpoints", func(t *testing.T) {
		_, email := createUser(t, "viewer", "non-admin-test")
		token := login(email, "testpass123")
		if token == "" {
			t.Fatal("login failed")
		}

		resp := getMe(t, token)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)

		resp2 := doRequest(http.MethodGet, "/api/v1/panel/users", token, nil)
		defer resp2.Body.Close()
		requireStatus(t, resp2, http.StatusForbidden)
	})
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

func deleteSessionFromRedis(email, token string) error {
	ctx := tCtx()
	pool, err := pgxpool.New(ctx, e2ePostgresDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	var userID string
	err = pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}

	sum := sha256.Sum256([]byte(token))
	key := fmt.Sprintf("labp:panel:auth:jwt:%s:%s", userID, hex.EncodeToString(sum[:]))

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
