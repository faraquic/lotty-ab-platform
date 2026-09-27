package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthReturnsOnlyOK(t *testing.T) {
	router := gin.New()
	NewHandler().RegisterRoutes(router.Group("/api/v1/panel"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/panel/health", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "OK" {
		t.Fatalf("body: got %q, want %q", response.Body.String(), "OK")
	}
}
