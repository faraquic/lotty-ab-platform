package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
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

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	panelBase   string
	runtimeBase string
	adminToken  string
	runID       string
	tmpDir      string
)

const (
	panelPort   = "18081"
	runtimePort = "18083"
)

const (
	e2ePostgresDSN = "postgres://lotty:lottypassword@localhost:5433/labp_e2e?sslmode=disable"
	e2eRedisURL    = "redis://localhost:6379/14"
	e2eS3Bucket    = "labp-e2e"
	e2eS3Endpoint  = "http://localhost:9000"
)

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	flag.Parse()

	if testing.Short() {
		fmt.Println("skipping e2e tests in short mode")
		return 0
	}

	if !isReachable("localhost", 5433) {
		fmt.Println("skipping e2e tests: postgres not reachable on :5433")
		return 0
	}
	if !isReachable("localhost", 6379) {
		fmt.Println("skipping e2e tests: redis not reachable on :6379")
		return 0
	}

	if err := requireE2EGuard(); err != nil {
		fmt.Printf("FATAL: %v\n", err)
		return 1
	}

	runID = fmt.Sprintf("%d", time.Now().UnixNano())

	var err error
	tmpDir, err = os.MkdirTemp("", "runtime-e2e-*")
	if err != nil {
		fmt.Printf("failed to create temp dir: %v\n", err)
		return 1
	}

	root := mustProjectRoot()

	panelCfg := filepath.Join(tmpDir, "panel.config.json")
	if err := os.WriteFile(panelCfg, fmt.Appendf(nil, `{
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
	}`, e2ePostgresDSN, e2eRedisURL, e2eS3Bucket, e2eS3Endpoint, panelPort), 0o644); err != nil {
		fmt.Printf("failed to write panel config: %v\n", err)
		os.RemoveAll(tmpDir)
		return 1
	}

	runtimeCfg := filepath.Join(tmpDir, "runtime.config.json")
	if err := os.WriteFile(runtimeCfg, fmt.Appendf(nil, `{
		"environment": "local",
		"database": {
			"redis": {"address": %q}
		},
		"runtime": {"http": {"address": "0.0.0.0:%s"}, "max_stale_age": "1s"}
	}`, e2eRedisURL, runtimePort), 0o644); err != nil {
		fmt.Printf("failed to write runtime config: %v\n", err)
		os.RemoveAll(tmpDir)
		return 1
	}

	panelBin := filepath.Join(tmpDir, "panel")
	panelBuild := exec.Command("go", "build", "-o", panelBin, "./services/panel")
	panelBuild.Dir = root
	if out, err := panelBuild.CombinedOutput(); err != nil {
		fmt.Printf("panel build failed: %v\n%s", err, out)
		os.RemoveAll(tmpDir)
		return 1
	}

	runtimeBin := filepath.Join(tmpDir, "runtime")
	buildRt := exec.Command("go", "build", "-o", runtimeBin, "./services/runtime")
	buildRt.Dir = root
	if out, err := buildRt.CombinedOutput(); err != nil {
		fmt.Printf("runtime build failed: %v\n%s", err, out)
		os.RemoveAll(tmpDir)
		return 1
	}

	panelBase = fmt.Sprintf("http://localhost:%s", panelPort)
	runtimeBase = fmt.Sprintf("http://localhost:%s", runtimePort)

	panelProc := exec.Command(panelBin)
	panelProc.Dir = filepath.Dir(panelCfg)
	panelProc.Env = append(os.Environ(), "CONFIG_NAME="+panelCfg)
	panelProc.Stdout = os.Stdout
	panelProc.Stderr = os.Stderr
	if err := panelProc.Start(); err != nil {
		fmt.Printf("failed to start panel: %v\n", err)
		os.RemoveAll(tmpDir)
		return 1
	}

	runtimeProc := exec.Command(runtimeBin)
	runtimeProc.Dir = filepath.Dir(runtimeCfg)
	runtimeProc.Env = append(os.Environ(), "CONFIG_NAME="+runtimeCfg)
	runtimeProc.Stdout = os.Stdout
	runtimeProc.Stderr = os.Stderr
	if err := runtimeProc.Start(); err != nil {
		fmt.Printf("failed to start runtime: %v\n", err)
		panelProc.Process.Kill()
		panelProc.Wait()
		os.RemoveAll(tmpDir)
		return 1
	}

	waitForServer(panelBase + "/api/v1/panel/health")
	waitForServer(runtimeBase + "/api/v1/runtime/health")

	adminToken = login("root@labp.net", "root!@#$")
	if adminToken == "" {
		fmt.Println("failed to login as bootstrap admin")
		runtimeProc.Process.Kill()
		panelProc.Process.Kill()
		runtimeProc.Wait()
		panelProc.Wait()
		os.RemoveAll(tmpDir)
		return 1
	}

	testCode := m.Run()

	runtimeProc.Process.Kill()
	panelProc.Process.Kill()
	runtimeProc.Wait()
	panelProc.Wait()

	if err := cleanupOwnRows(); err != nil {
		fmt.Printf("cleanup: %v\n", err)
		if testCode == 0 {
			return 1
		}
	}

	os.RemoveAll(tmpDir)

	return testCode
}

func requireE2EGuard() error {
	if os.Getenv("E2E_TEST") != "1" {
		return fmt.Errorf("E2E_TEST=1 must be set to run destructive cleanup. Refusing to proceed.")
	}

	dbName := e2ePostgresDSN[strings.LastIndex(e2ePostgresDSN, "/")+1:]
	if idx := strings.IndexByte(dbName, '?'); idx != -1 {
		dbName = dbName[:idx]
	}
	if !strings.HasSuffix(dbName, "_e2e") {
		return fmt.Errorf("Postgres database name %q does not end with _e2e. Refusing to proceed.", dbName)
	}

	return nil
}

func cleanupOwnRows() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, e2ePostgresDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	pattern := "%" + runID
	if _, err := pool.Exec(ctx, `DELETE FROM experiments WHERE name LIKE $1`, pattern); err != nil {
		return fmt.Errorf("delete experiments: %w", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM flags WHERE key LIKE $1`, pattern); err != nil {
		return fmt.Errorf("delete flags: %w", err)
	}
	return nil
}

func jsonBody(v interface{}) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func panelRequest(method, path, token string, body io.Reader) *http.Response {
	return doRequest(panelBase, method, path, token, body)
}

func runtimeRequest(method, path string, body io.Reader) *http.Response {
	return doRequest(runtimeBase, method, path, "", body)
}

func doRequest(base, method, path, token string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		panic(fmt.Sprintf("create request: %v", err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("do request %s %s: %v", method, path, err))
	}
	return resp
}

func decodeJSON(resp *http.Response, v interface{}) {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, v); err != nil {
		panic(fmt.Sprintf("decode response: %v", err))
	}
}

func requireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want %d (body: %s)", resp.StatusCode, want, body)
	}
}

func login(email, password string) string {
	resp := panelRequest(http.MethodPost, "/api/v1/panel/login", "",
		jsonBody(map[string]string{"email": email, "password": password}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	return result.Data.Token
}

func waitForServer(url string) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	panic(fmt.Sprintf("server did not become ready within 30s at %s", url))
}

func isReachable(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func mustProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "..", "..", "..")
}
