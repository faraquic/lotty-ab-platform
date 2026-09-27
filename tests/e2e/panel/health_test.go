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
	"testing"
	"time"
)

func requireHealthOK(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "OK" {
		t.Fatalf("body: got %q, want %q", body, "OK")
	}
}

func TestHealth_Liveness(t *testing.T) {
	resp := doRequest(http.MethodGet, "/api/v1/panel/health", "", nil)
	requireHealthOK(t, resp)
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
	requireHealthOK(t, resp)
}

func TestPanelStartupFailsWhenRedisUnavailable(t *testing.T) {
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

func TestHealth_ReturnsOKWhenS3Unavailable(t *testing.T) {
	proc, port := startIsolatedPanel(t, "s3-unavail", map[string]string{
		"s3_endpoint": "http://localhost:19998",
	})
	defer stopIsolatedPanel(t, proc)

	url := fmt.Sprintf("http://localhost:%s/api/v1/panel/health", port)
	resp := waitForHealth(t, url, http.StatusOK)
	requireHealthOK(t, resp)
}

func TestPanelStartupFailsWhenPostgresUnavailable(t *testing.T) {
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
