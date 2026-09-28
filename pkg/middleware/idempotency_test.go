package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

func newIdempotencyTestEngine(backend idempotencyBackend, method, path string, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(method, path, newIdempotencyHandler(backend, zap.NewNop()), handler)
	return r
}

func echoJSONHandler(calls *int) gin.HandlerFunc {
	return func(c *gin.Context) {
		*calls++
		raw, _ := io.ReadAll(c.Request.Body)
		c.Header("Content-Type", "application/json")
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = c.Writer.Write(raw)
	}
}

func doIdempotencyRequest(t *testing.T, r *gin.Engine, method, path, key, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if key != "" {
		req.Header.Set(IdempotencyKeyHeader, key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIdempotencyPassthroughWithoutKey(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	r := newIdempotencyTestEngine(backend, http.MethodPost, "/flags", echoJSONHandler(&calls))

	for i := 0; i < 2; i++ {
		w := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "", "application/json", `{"key":"a"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
		}
		if w.Body.String() != `{"key":"a"}` {
			t.Fatalf("body not echoed: %q", w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("handler calls: got %d, want 2", calls)
	}
}

func TestIdempotencyReplaySameBody(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	r := newIdempotencyTestEngine(backend, http.MethodPost, "/flags", echoJSONHandler(&calls))

	first := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "key-1", "application/json", `{"key":"a"}`)
	second := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "key-1", "application/json", `{"key":"a"}`)

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status: got %d and %d, want 200 and 200", first.Code, second.Code)
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("replay body mismatch: %q vs %q", second.Body.String(), first.Body.String())
	}
	if second.Header().Get(IdempotentReplayedHeader) != "true" {
		t.Fatalf("missing %s header on replay", IdempotentReplayedHeader)
	}
	if first.Header().Get(IdempotentReplayedHeader) == "true" {
		t.Fatalf("original response must not carry %s header", IdempotentReplayedHeader)
	}
	if calls != 1 {
		t.Fatalf("handler calls: got %d, want 1", calls)
	}
}

func TestIdempotencyConflictDifferentBody(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	r := newIdempotencyTestEngine(backend, http.MethodPost, "/flags", echoJSONHandler(&calls))

	first := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "key-2", "application/json", `{"key":"a"}`)
	second := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "key-2", "application/json", `{"key":"b"}`)

	if first.Code != http.StatusOK {
		t.Fatalf("first status: got %d, want %d", first.Code, http.StatusOK)
	}
	if second.Code != http.StatusConflict {
		t.Fatalf("second status: got %d, want %d", second.Code, http.StatusConflict)
	}
	var payload struct {
		Success bool `json:"success"`
		Error   *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
		t.Fatalf("conflict body is not JSON: %v", err)
	}
	if payload.Success || payload.Error == nil || payload.Error.Code != "CONFLICT" {
		t.Fatalf("unexpected conflict payload: %q", second.Body.String())
	}
	if calls != 1 {
		t.Fatalf("handler calls: got %d, want 1", calls)
	}
}

func TestIdempotencySkipsGetAndDelete(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	getCalls, deleteCalls := 0, 0
	r.Handle(http.MethodGet, "/flags", newIdempotencyHandler(backend, zap.NewNop()), echoJSONHandler(&getCalls))
	r.Handle(http.MethodDelete, "/flags/1", newIdempotencyHandler(backend, zap.NewNop()), echoJSONHandler(&deleteCalls))

	for i := 0; i < 2; i++ {
		if w := doIdempotencyRequest(t, r, http.MethodGet, "/flags", "key-get", "", ""); w.Code != http.StatusOK {
			t.Fatalf("get status: got %d", w.Code)
		}
		if w := doIdempotencyRequest(t, r, http.MethodDelete, "/flags/1", "key-del", "", ""); w.Code != http.StatusOK {
			t.Fatalf("delete status: got %d", w.Code)
		}
	}
	if getCalls != 2 || deleteCalls != 2 {
		t.Fatalf("handler calls: got get=%d delete=%d, want 2 and 2", getCalls, deleteCalls)
	}
}

func TestIdempotencyKeyTooLong(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	r := newIdempotencyTestEngine(backend, http.MethodPost, "/flags", echoJSONHandler(&calls))

	w := doIdempotencyRequest(t, r, http.MethodPost, "/flags", strings.Repeat("k", idempotencyMaxKeyLen+1), "application/json", `{"key":"a"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
	if calls != 0 {
		t.Fatalf("handler calls: got %d, want 0", calls)
	}
}

func TestIdempotencySkipsNonJSON(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	r := newIdempotencyTestEngine(backend, http.MethodPost, "/users/1/avatar", echoJSONHandler(&calls))

	body := "--boundary\r\nContent-Disposition: form-data; name=\"avatar\"; filename=\"a.png\"\r\n\r\nbytes\r\n--boundary--\r\n"
	first := doIdempotencyRequest(t, r, http.MethodPost, "/users/1/avatar", "key-avatar", "multipart/form-data; boundary=boundary", body)
	second := doIdempotencyRequest(t, r, http.MethodPost, "/users/1/avatar", "key-avatar", "multipart/form-data; boundary=boundary", body)

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status: got %d and %d, want 200 and 200", first.Code, second.Code)
	}
	if calls != 2 {
		t.Fatalf("handler calls: got %d, want 2", calls)
	}
	if second.Header().Get(IdempotentReplayedHeader) == "true" {
		t.Fatalf("multipart responses must not be replayed")
	}
}

func TestIdempotencyDoesNotCacheErrors(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(http.MethodPost, "/flags", newIdempotencyHandler(backend, zap.NewNop()), func(c *gin.Context) {
		calls++
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
	})

	for i := 0; i < 2; i++ {
		w := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "key-err", "application/json", `{"key":"a"}`)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status: got %d, want 500", w.Code)
		}
		if w.Header().Get(IdempotentReplayedHeader) == "true" {
			t.Fatalf("error responses must not be replayed")
		}
	}
	if calls != 2 {
		t.Fatalf("handler calls: got %d, want 2", calls)
	}
}

func TestIdempotencyIsolatesRoutesAndMethods(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	postCalls, patchCalls := 0, 0
	r.Handle(http.MethodPost, "/flags", newIdempotencyHandler(backend, zap.NewNop()), echoJSONHandler(&postCalls))
	r.Handle(http.MethodPatch, "/flags/1", newIdempotencyHandler(backend, zap.NewNop()), echoJSONHandler(&patchCalls))

	body := `{"key":"a"}`
	if w := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "shared", "application/json", body); w.Code != http.StatusOK {
		t.Fatalf("post status: got %d", w.Code)
	}
	if w := doIdempotencyRequest(t, r, http.MethodPatch, "/flags/1", "shared", "application/json", body); w.Code != http.StatusOK {
		t.Fatalf("patch status: got %d", w.Code)
	}
	if w := doIdempotencyRequest(t, r, http.MethodPatch, "/flags/1", "shared", "application/json", body); w.Header().Get(IdempotentReplayedHeader) != "true" {
		t.Fatalf("same route and body must replay")
	}
	if postCalls != 1 || patchCalls != 1 {
		t.Fatalf("handler calls: got post=%d patch=%d, want 1 and 1", postCalls, patchCalls)
	}
}

func TestIdempotencyEmptyBody(t *testing.T) {
	backend := newMemoryIdempotencyBackend()
	calls := 0
	r := newIdempotencyTestEngine(backend, http.MethodPost, "/logout", echoJSONHandler(&calls))

	first := doIdempotencyRequest(t, r, http.MethodPost, "/logout", "key-empty", "", "")
	second := doIdempotencyRequest(t, r, http.MethodPost, "/logout", "key-empty", "", "")

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status: got %d and %d, want 200 and 200", first.Code, second.Code)
	}
	if second.Header().Get(IdempotentReplayedHeader) != "true" {
		t.Fatalf("empty-body response must replay")
	}
	if calls != 1 {
		t.Fatalf("handler calls: got %d, want 1", calls)
	}
}

func TestIdempotencyNilBackendPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	calls := 0
	r.Handle(http.MethodPost, "/flags", IdempotencyGin(nil, zap.NewNop()), echoJSONHandler(&calls))

	for i := 0; i < 2; i++ {
		w := doIdempotencyRequest(t, r, http.MethodPost, "/flags", "key-nil", "application/json", `{"key":"a"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
		}
	}
	if calls != 2 {
		t.Fatalf("handler calls: got %d, want 2", calls)
	}
}

func TestIdempotencyRedisKeyScopesCaller(t *testing.T) {
	withUser := idempotencyRedisKey("user-1", http.MethodPost, "/api/v1/panel/flags", "k")
	anon := idempotencyRedisKey("", http.MethodPost, "/api/v1/panel/flags", "k")
	otherUser := idempotencyRedisKey("user-2", http.MethodPost, "/api/v1/panel/flags", "k")
	if withUser == anon || withUser == otherUser {
		t.Fatalf("redis keys must be scoped by caller: %q %q %q", withUser, anon, otherUser)
	}
}
