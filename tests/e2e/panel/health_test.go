package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type healthReadyResponse struct {
	Service     string                      `json:"service"`
	Version     string                      `json:"version"`
	Environment string                      `json:"environment"`
	Timestamp   string                      `json:"timestamp"`
	Components  map[string]healthCompStatus `json:"components"`
}

type healthCompStatus struct {
	Status      string `json:"status"`
	Criticality string `json:"criticality"`
	Message     string `json:"message,omitempty"`
}

func TestHealth_Liveness(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/health", "", nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Fatalf("body: got %q, want %q", string(body), "OK")
	}
}

func TestHealth_LivenessNoAuthRequired(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/panel/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
}

func TestHealth_Readiness(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/ready", "", nil)
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
	requireJSONContentType(t, resp)

	var result healthReadyResponse
	decodeJSON(resp, &result)

	if result.Service != "labp-panel" {
		t.Errorf("service: got %q, want %q", result.Service, "labp-panel")
	}
	if result.Version == "" {
		t.Error("version must not be empty")
	}
	if result.Environment == "" {
		t.Error("environment must not be empty")
	}
	if result.Timestamp == "" {
		t.Error("timestamp must not be empty")
	}

	for _, name := range []string{"service", "database", "cache", "snapshot", "storage"} {
		comp, ok := result.Components[name]
		if !ok {
			t.Errorf("components missing %q", name)
			continue
		}
		if comp.Status != "ok" {
			t.Errorf("component %q status: got %q, want %q", name, comp.Status, "ok")
		}
	}

	wantCriticality := map[string]string{
		"service":  "required",
		"database": "required",
		"cache":    "required",
		"snapshot": "required",
		"storage":  "optional",
	}
	for name, want := range wantCriticality {
		comp, ok := result.Components[name]
		if !ok {
			t.Errorf("components missing %q", name)
			continue
		}
		if comp.Criticality != want {
			t.Errorf("component %q criticality: got %q, want %q", name, comp.Criticality, want)
		}
	}
}

func TestHealth_ReadinessNoAuthRequired(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/panel/ready", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	requireStatus(t, resp, http.StatusOK)
}

func TestHealth_ReadinessNoSecretsInResponse(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/ready", "", nil)
	defer resp.Body.Close()

	requireNoSensitiveFields(t, resp,
		"postgres://", "lottypassword", "minioadmin", "secret_key",
		"access_key", "e2e-test-secret", "localhost:5433", "localhost:6379", "localhost:9000",
	)
}

func TestHealth_ReadinessRedisUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.local.json")

	cfg := fmt.Sprintf(`{
		"environment": "local",
		"auth": {
			"jwt": {"secret_key": "e2e-test-secret-not-for-prod", "ttl": "1h"},
			"bootstrap": {"full_name": "root", "email": "root@labp.net", "password_hash": "$argon2id$v=19$m=65536,t=1,p=4$6lGItC3BN+vvxDF42RRw2g$svLMZu6udCWh8/bjOlpP1S3syNanEwiQO17cbUaDvck"}
		},
		"database": {
			"postgres": {"dsn": %q},
			"redis":    {"address": "localhost:19999"},
			"s3":       {"bucket": %q, "region": "us-east-1", "endpoint": %q, "access_key": "minioadmin", "secret_key": "minioadmin"}
		},
		"panel": {"http": {"address": "0.0.0.0:18082"}}
	}`, e2ePostgresDSN, e2eS3Bucket, e2eS3Endpoint)

	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(cfgDir, "panel-redis-unavail")
	build := exec.Command("go", "build", "-o", binPath, "./services/panel")
	build.Dir = mustProjectRoot()
	build.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Dir = cfgDir
	cmd.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err := cmd.Run()

	if ctx.Err() == nil && err == nil {
		t.Error("server should not start successfully with unreachable redis")
	}
}

func TestHealth_ReadinessS3Unavailable(t *testing.T) {
	proc, port := startIsolatedPanel(t, "s3-unavail", map[string]string{
		"s3_endpoint": "http://localhost:19998",
	})
	defer stopIsolatedPanel(t, proc)

	url := fmt.Sprintf("http://localhost:%s/api/v1/panel/ready", port)
	resp := waitForHealth(t, url, http.StatusOK)

	var result healthReadyResponse
	decodeJSON(resp, &result)

	dbComp, ok := result.Components["database"]
	if !ok {
		t.Fatal("missing database component")
	}
	if dbComp.Status != "ok" {
		t.Errorf("database status: got %q, want %q", dbComp.Status, "ok")
	}

	cacheComp, ok := result.Components["cache"]
	if !ok {
		t.Fatal("missing cache component")
	}
	if cacheComp.Status != "ok" {
		t.Errorf("cache status: got %q, want %q", cacheComp.Status, "ok")
	}

	snapComp, ok := result.Components["snapshot"]
	if !ok {
		t.Fatal("missing snapshot component")
	}
	if snapComp.Status != "ok" {
		t.Errorf("snapshot status: got %q, want %q", snapComp.Status, "ok")
	}

	storageComp, ok := result.Components["storage"]
	if !ok {
		t.Fatal("missing storage component")
	}
	if storageComp.Status != "unavailable" {
		t.Errorf("storage status: got %q, want %q", storageComp.Status, "unavailable")
	}
}

func TestHealth_ReadinessBothOptionalDepsUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.local.json")

	cfg := fmt.Sprintf(`{
		"environment": "local",
		"auth": {
			"jwt": {"secret_key": "e2e-test-secret-not-for-prod", "ttl": "1h"},
			"bootstrap": {"full_name": "root", "email": "root@labp.net", "password_hash": "$argon2id$v=19$m=65536,t=1,p=4$6lGItC3BN+vvxDF42RRw2g$svLMZu6udCWh8/bjOlpP1S3syNanEwiQO17cbUaDvck"}
		},
		"database": {
			"postgres": {"dsn": %q},
			"redis":    {"address": "localhost:19999"},
			"s3":       {"bucket": %q, "region": "us-east-1", "endpoint": %q, "access_key": "minioadmin", "secret_key": "minioadmin"}
		},
		"panel": {"http": {"address": "0.0.0.0:18082"}}
	}`, e2ePostgresDSN, e2eS3Bucket, e2eS3Endpoint)

	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(cfgDir, "panel-both-unavail")
	build := exec.Command("go", "build", "-o", binPath, "./services/panel")
	build.Dir = mustProjectRoot()
	build.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Dir = cfgDir
	cmd.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err := cmd.Run()

	if ctx.Err() == nil && err == nil {
		t.Error("server should not start successfully with unreachable redis")
	}
}

func TestHealth_PostgresUnavailablePreventsStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.local.json")

	cfg := fmt.Sprintf(`{
		"environment": "local",
		"auth": {
			"jwt": {"secret_key": "test-secret-not-for-prod", "ttl": "1h"},
			"bootstrap": {"full_name": "root", "email": "root@labp.net", "password_hash": "$argon2id$v=19$m=65536,t=1,p=4$6lGItC3BN+vvxDF42RRw2g$svLMZu6udCWh8/bjOlpP1S3syNanEwiQO17cbUaDvck"}
		},
		"database": {
			"postgres": {"dsn": "postgres://lotty:lottypassword@localhost:19997/labp_e2e?sslmode=disable"},
			"redis":    {"address": "localhost:6379/15"},
			"s3":       {"bucket": "labp-e2e", "region": "us-east-1", "endpoint": %q, "access_key": "minioadmin", "secret_key": "minioadmin"}
		},
		"panel": {"http": {"address": "0.0.0.0:18082"}}
	}`, e2eS3Endpoint)

	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(cfgDir, "panel")
	build := exec.Command("go", "build", "-o", binPath, "./services/panel")
	build.Dir = mustProjectRoot()
	build.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Dir = cfgDir
	cmd.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err := cmd.Run()

	if ctx.Err() == nil && err == nil {
		t.Error("server should not start successfully with unreachable postgres")
	}
}

// --- helpers ---

func startIsolatedPanel(t *testing.T, label string, overrides map[string]string) (*exec.Cmd, string) {
	t.Helper()

	port := findFreePort(t)
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.local.json")

	redisAddr := e2eRedisURL
	if v, ok := overrides["redis_address"]; ok {
		redisAddr = v
	}
	s3Endpoint := e2eS3Endpoint
	if v, ok := overrides["s3_endpoint"]; ok {
		s3Endpoint = v
	}

	cfg := fmt.Sprintf(`{
		"environment": "local",
		"auth": {
			"jwt": {"secret_key": "e2e-test-secret-not-for-prod", "ttl": "1h"},
			"bootstrap": {"full_name": "root", "email": "root@labp.net", "password_hash": "$argon2id$v=19$m=65536,t=1,p=4$6lGItC3BN+vvxDF42RRw2g$svLMZu6udCWh8/bjOlpP1S3syNanEwiQO17cbUaDvck"}
		},
		"database": {
			"postgres": {"dsn": %q},
			"redis":    {"address": %q},
			"s3":       {"bucket": %q, "region": "us-east-1", "endpoint": %q, "access_key": "minioadmin", "secret_key": "minioadmin"}
		},
		"panel": {"http": {"address": "0.0.0.0:%s"}}
	}`, e2ePostgresDSN, redisAddr, e2eS3Bucket, s3Endpoint, port)

	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(cfgDir, "panel-"+label)
	build := exec.Command("go", "build", "-o", binPath, "./services/panel")
	build.Dir = mustProjectRoot()
	build.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", label, err, out)
	}

	cmd := exec.Command(binPath)
	cmd.Dir = cfgDir
	cmd.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", label, err)
	}

	healthURL := fmt.Sprintf("http://localhost:%s/api/v1/panel/health", port)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return cmd, port
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	cmd.Process.Kill()
	cmd.Wait()
	t.Fatalf("isolated panel %s did not become ready within 15s", label)
	return nil, ""
}

func stopIsolatedPanel(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Kill()
	cmd.Wait()
}

func findFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return fmt.Sprintf("%d", port)
}

func waitForHealth(t *testing.T, url string, wantStatus int) *http.Response {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			if resp.StatusCode == wantStatus {
				return resp
			}
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("health endpoint %s did not return %d within 10s", url, wantStatus)
	return nil
}
