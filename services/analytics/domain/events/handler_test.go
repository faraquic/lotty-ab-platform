package events

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func testHandlerApp(svc *Service) *fiber.App {
	app := fiber.New()
	NewHandler(svc, zap.NewNop()).RegisterRoutes(app.Group("/api/v1/analytics"))

	return app
}

func postBatch(t *testing.T, app *fiber.App, path string, body any) (int, api.Response) {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var out api.Response
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}

	return resp.StatusCode, out
}

func TestHandlerEventsAcceptedStatus(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	item := testEventItem(now)

	status, _ := postBatch(t, app, "/api/v1/analytics/events/batch", EventBatchRequest{Events: []EventItem{item}})

	if status != http.StatusAccepted {
		t.Errorf("status = %d, want 202", status)
	}
}

func TestHandlerEventsMixedStatus(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	good := testEventItem(now)
	bad := testEventItem(now)
	bad.EventType = "nope"

	status, _ := postBatch(t, app, "/api/v1/analytics/events/batch", EventBatchRequest{Events: []EventItem{good, bad}})

	if status != http.StatusMultiStatus {
		t.Errorf("status = %d, want 207", status)
	}
}

func TestHandlerEventsAllRejectedStatus(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	bad := testEventItem(now)
	bad.EventType = "nope"

	status, _ := postBatch(t, app, "/api/v1/analytics/events/batch", EventBatchRequest{Events: []EventItem{bad}})

	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestHandlerEventsBrokerUnavailable(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{err: ErrBrokerUnavailable}, testPII())
	app := testHandlerApp(svc)

	status, out := postBatch(t, app, "/api/v1/analytics/events/batch", EventBatchRequest{Events: []EventItem{testEventItem(now)}})

	if status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", status)
	}

	if out.Error == nil || out.Error.Code != api.ServiceUnavailable {
		t.Errorf("error = %+v, want SERVICE_UNAVAILABLE", out.Error)
	}
}

func TestHandlerExposuresAcceptedStatus(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	status, _ := postBatch(t, app, "/api/v1/analytics/exposures/batch", ExposureBatchRequest{Exposures: []ExposureItem{testExposureItem(now)}})

	if status != http.StatusAccepted {
		t.Errorf("status = %d, want 202", status)
	}
}

func TestHandlerListEventTypes(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/event-types", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var out api.Response
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !out.Success {
		t.Fatalf("success = false: %+v", out)
	}
}

func TestHandlerMalformedBody(t *testing.T) {
	now := time.Now()
	svc := testService(now, testTypes(), newFakeClaims(), &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/events/batch", bytes.NewReader([]byte("{nope")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHandlerDuplicateReplay(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	svc := testService(now, testTypes(), claims, &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	item := testEventItem(now)
	req := EventBatchRequest{Events: []EventItem{item}}

	first, _ := postBatch(t, app, "/api/v1/analytics/events/batch", req)
	second, _ := postBatch(t, app, "/api/v1/analytics/events/batch", req)

	if first != http.StatusAccepted || second != http.StatusAccepted {
		t.Errorf("statuses = %d, %d, want 202, 202", first, second)
	}
}

func TestHandlerConflictStatus(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	svc := testService(now, testTypes(), claims, &fakePublisher{}, testPII())
	app := testHandlerApp(svc)

	item := testEventItem(now)

	if _, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{item}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	changed := item
	changed.SubjectID = "intruder"

	status, _ := postBatch(t, app, "/api/v1/analytics/events/batch", EventBatchRequest{Events: []EventItem{changed}})

	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (all rejected on conflict)", status)
	}
}
