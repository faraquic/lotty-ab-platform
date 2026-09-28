package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newMetricsFiberApp(job string) *fiber.App {
	app := fiber.New()
	app.Use(MiddlewareFiber(job))
	app.Get("/decide", func(c fiber.Ctx) error {
		return c.SendString(`{"flag_key":"checkout"}`)
	})
	app.Get("/flags/:id", func(c fiber.Ctx) error {
		return c.SendString(`{"id":"` + c.Params("id") + `"}`)
	})
	app.Get("/unavailable", func(c fiber.Ctx) error {
		return c.Status(http.StatusServiceUnavailable).SendString("nope")
	})
	return app
}

func callFiber(t *testing.T, app *fiber.App, target string) {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, target, nil))
	if err != nil {
		t.Fatalf("request %s: %v", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
}

// exposition renders the current default registry, which is where the metric
// vectors of this package are registered.
func exposition(t *testing.T) string {
	t.Helper()

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	return rec.Body.String()
}

func assertSeries(t *testing.T, body, series string) {
	t.Helper()

	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, series) {
			return
		}
	}

	t.Errorf("exposition has no series %q", series)
}

func TestMiddlewareFiberRecordsJobMethodPathAndCode(t *testing.T) {
	const job = "fiber-job-success"
	app := newMetricsFiberApp(job)

	callFiber(t, app, "/decide")

	body := exposition(t)
	assertSeries(t, body, `http_requests_total{code="200",job="`+job+`",method="GET",path="/decide"} 1`)
	assertSeries(t, body, `http_request_duration_seconds_count{job="`+job+`",method="GET",path="/decide"} 1`)
	assertSeries(t, body, `http_response_size_bytes_count{job="`+job+`",method="GET",path="/decide"} 1`)
}

func TestMiddlewareFiberRecordsErrorStatus(t *testing.T) {
	const job = "fiber-job-error"
	app := newMetricsFiberApp(job)

	callFiber(t, app, "/unavailable")

	assertSeries(t, exposition(t), `http_requests_total{code="503",job="`+job+`",method="GET",path="/unavailable"} 1`)
}

func TestMiddlewareFiberRecordsRoutePatternNotConcretePath(t *testing.T) {
	const job = "fiber-job-param"
	app := newMetricsFiberApp(job)

	callFiber(t, app, "/flags/2f0a1b6c-0000-7000-8000-000000000001")

	assertSeries(t, exposition(t), `http_requests_total{code="200",job="`+job+`",method="GET",path="/flags/:id"} 1`)
}
