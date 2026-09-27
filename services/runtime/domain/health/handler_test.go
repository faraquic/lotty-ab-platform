package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHealthReturnsOnlyOK(t *testing.T) {
	app := fiber.New()
	NewHandler().RegisterRoutes(app.Group("/api/v1/runtime"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/health", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "OK" {
		t.Fatalf("body: got %q, want %q", body, "OK")
	}
}
