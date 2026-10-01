package e2e

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

const (
	loadConnections = 256
	loadDuration    = 3 * time.Second
	loadWarmup      = 1 * time.Second
	loadDelay       = 100 * time.Millisecond
	maxResponseSize = 32 * 1024
)

const defaultSlaP99 = 10 * time.Millisecond

func loadBase() string {
	if v := os.Getenv("LOAD_BASE_URL"); v != "" {
		return v
	}
	return runtimeBase
}

// decideP99Threshold возвращает порог SLA для p99 latency /decide.
// Настраивается через DECIDE_P99_THRESHOLD_MS (целые миллисекунды), по умолчанию 10ms.
func decideP99Threshold(t *testing.T) time.Duration {
	t.Helper()
	v := strings.TrimSpace(os.Getenv("DECIDE_P99_THRESHOLD_MS"))
	if v == "" {
		return defaultSlaP99
	}
	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		t.Fatalf("invalid DECIDE_P99_THRESHOLD_MS=%q: must be positive integer milliseconds", os.Getenv("DECIDE_P99_THRESHOLD_MS"))
	}
	return time.Duration(ms) * time.Millisecond
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

type workerResult struct {
	latencies []time.Duration
	total     int
	failed    int
}

func mustBody(t *testing.T, subject string, flags []string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"subject_id": subject,
		"attributes": map[string]any{"country": "DE"},
		"flags":      flags,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func runLoadTest(t *testing.T, subject string, flags []string) {
	t.Helper()

	slaP99 := decideP99Threshold(t)

	body := mustBody(t, subject, flags)
	url := loadBase() + "/api/v1/runtime/decide"

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        loadConnections,
			MaxIdleConnsPerHost: loadConnections,
			MaxConnsPerHost:     loadConnections,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Проверка, что сервер вообще жив, до старта нагрузки.
	if resp, err := client.Post(url, "application/json", bytes.NewReader(body)); err != nil {
		t.Fatalf("server unreachable: %v", err)
	} else {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("preflight status %d", resp.StatusCode)
		}
	}

	startAt := time.Now()
	warmupEnd := startAt.Add(loadWarmup)
	deadline := warmupEnd.Add(loadDuration)

	results := make([]workerResult, loadConnections)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(r *workerResult) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
				if err != nil {
					t.Errorf("new request: %v", err)
					return
				}
				req.Header.Set("Content-Type", "application/json")

				reqStart := time.Now()
				failed := false
				resp, err := client.Do(req)
				if err != nil {
					failed = true
				} else {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					failed = resp.StatusCode != http.StatusOK
				}
				latency := time.Since(reqStart)

				// Прогрев не учитываем.
				if reqStart.After(warmupEnd) {
					r.total++
					r.latencies = append(r.latencies, latency) // включая таймауты
					if failed {
						r.failed++
					}
				}
				time.Sleep(loadDelay)
			}
		}(&results[i])
	}
	wg.Wait()

	var (
		all    []time.Duration
		total  int
		failed int
	)
	for _, r := range results {
		all = append(all, r.latencies...)
		total += r.total
		failed += r.failed
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	p50, p95, p99 := percentile(all, 50), percentile(all, 95), percentile(all, 99)
	t.Logf("total=%d failed=%d p50=%v p95=%v p99=%v slaP99=%v", total, failed, p50, p95, p99, slaP99)

	if total == 0 {
		t.Fatal("no requests were measured")
	}
	if failed > 0 {
		t.Errorf("%d/%d requests failed", failed, total)
	}
	if p99 > slaP99 {
		t.Errorf("p99 latency %v exceeds %v SLA", p99, slaP99)
	}
}

func TestDecideLoad256Connections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}
	flagKey := "load-checkout-" + runID
	createFlag(t, "load-checkout", "string", "off")
	waitForKnownFlag(t, flagKey)
	runLoadTest(t, "load-test-subject", []string{flagKey})
}

func TestDecideLoadMultiFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}
	checkoutKey := "load-multi-checkout-" + runID
	plainKey := "load-multi-plain-" + runID
	createFlag(t, "load-multi-checkout", "string", "off")
	createFlag(t, "load-multi-plain", "string", "off")
	waitForKnownFlag(t, checkoutKey)
	waitForKnownFlag(t, plainKey)
	runLoadTest(t, "load-test-subject", []string{checkoutKey, plainKey})
}

func TestDecideLoadResponseSize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	flagKey := "load-size-" + runID
	createFlag(t, "load-size", "string", "off")
	waitForKnownFlag(t, flagKey)

	body := mustBody(t, "load-test-size", []string{flagKey})
	resp, err := http.Post(loadBase()+"/api/v1/runtime/decide", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	t.Logf("response size: %d bytes", len(respBody))
	if len(respBody) > maxResponseSize {
		t.Errorf("response size %d exceeds %d limit", len(respBody), maxResponseSize)
	}
}
