package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
)

// decoded mirrors the JSON envelope so we can assert on unexported fields
// without reaching into the api package internals.
type decoded struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) decoded {
	t.Helper()

	var d decoded
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("response body is not valid json: %v\nbody: %s", err, rec.Body.String())
	}

	return d
}

func TestOK(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{"nil data", nil},
		{"map", map[string]int{"count": 3}},
		{"struct", struct{ ID int64 }{ID: 7}},
		{"slice", []string{"a", "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			api.OK(rec, tc.data)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type = %q, want application/json", ct)
			}

			d := decode(t, rec)
			if !d.Success {
				t.Errorf("success = false, want true")
			}
			if d.Error != nil {
				t.Errorf("error = %+v, want nil", d.Error)
			}

			// data must be present and non-empty unless it was nil
			if tc.data == nil {
				if len(d.Data) != 0 {
					t.Errorf("data = %s, want omitted for nil", d.Data)
				}
			} else if len(d.Data) == 0 {
				t.Errorf("data omitted, want payload for %#v", tc.data)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		code    string
		message string
	}{
		{"bad request", http.StatusBadRequest, api.BadRequest, "field required"},
		{"not found", http.StatusNotFound, api.NotFound, "user not found"},
		{"conflict", http.StatusConflict, "CONFLICT", "user already exists"},
		{"custom 418", http.StatusTeapot, "IM_A_TEAPOT", "short and stout"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			api.Error(rec, tc.status, tc.code, tc.message)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type = %q, want application/json", ct)
			}

			d := decode(t, rec)
			if d.Success {
				t.Errorf("success = true, want false")
			}
			if d.Error == nil {
				t.Fatalf("error = nil, want %q/%q", tc.code, tc.message)
			}
			if d.Error.Code != tc.code {
				t.Errorf("error.code = %q, want %q", d.Error.Code, tc.code)
			}
			if d.Error.Message != tc.message {
				t.Errorf("error.message = %q, want %q", d.Error.Message, tc.message)
			}
		})
	}
}

func TestInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	api.InternalError(rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	d := decode(t, rec)
	if d.Success {
		t.Errorf("success = true, want false")
	}
	if d.Error == nil {
		t.Fatalf("error = nil, want internal error")
	}
	if d.Error.Code != api.InternalServerError {
		t.Errorf("error.code = %q, want %q", d.Error.Code, api.InternalServerError)
	}
	// internal errors must never leak the underlying cause
	if d.Error.Message != "internal server error" {
		t.Errorf("error.message = %q, want %q", d.Error.Message, "internal server error")
	}
}

// sampleDTO is a request body shape used to exercise ValidateRequest.
type sampleDTO struct {
	Name  string `json:"name" binding:"required,min=3"`
	Email string `json:"email" binding:"required,email"`
}

func TestValidateRequest(t *testing.T) {
	// oversizedBody exceeds api.maxBodyBytes (1 MiB) so the size guard fires.
	const maxBodyBytes = 1 << 20
	oversizedBody := strings.Repeat("x", maxBodyBytes+10)

	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantStatus int
		wantCode   string
	}{
		{
			name:    "valid payload",
			body:    `{"name":"abc","email":"a@b.com"}`,
			wantErr: false,
		},
		{
			name:    "valid with unknown fields ignored",
			body:    `{"name":"abc","email":"a@b.com","extra":1,"nested":{"x":true}}`,
			wantErr: false,
		},
		{
			name:       "malformed json",
			body:       `{"name":`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
			wantCode:   api.BadRequest,
		},
		{
			name:       "missing required field",
			body:       `{"email":"a@b.com"}`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
			wantCode:   api.BadRequest,
		},
		{
			name:       "field too short",
			body:       `{"name":"ab","email":"a@b.com"}`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
			wantCode:   api.BadRequest,
		},
		{
			name:       "invalid email format",
			body:       `{"name":"abc","email":"not-an-email"}`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
			wantCode:   api.BadRequest,
		},
		{
			name:       "wrong field type",
			body:       `{"name":123,"email":"a@b.com"}`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
			wantCode:   api.BadRequest,
		},
		{
			name:       "empty body",
			body:       ``,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
			wantCode:   api.BadRequest,
		},
		{
			name:       "oversized body",
			body:       oversizedBody,
			wantErr:    true,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   api.PayloadTooLarge,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))

			rec := httptest.NewRecorder()
			var dst sampleDTO
			err := api.ValidateRequest(rec, req, &dst)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want error (status %d, code %s)", tc.wantStatus, tc.wantCode)
				}
				if rec.Code != tc.wantStatus {
					t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
				}
				d := decode(t, rec)
				if d.Error == nil || d.Error.Code != tc.wantCode {
					t.Errorf("error code = %v, want %q", d.Error, tc.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			// on success the handler must not have written anything
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want default 200 (handler writes later)", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty on success", rec.Body.String())
			}
			if dst.Name != "abc" || dst.Email != "a@b.com" {
				t.Errorf("decoded dst = %+v, want name=abc email=a@b.com", dst)
			}
		})
	}
}
