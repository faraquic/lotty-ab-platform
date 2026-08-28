package e2e

import (
	"bytes"
	"context"
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

	"github.com/goccy/go-json"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
)

var (
	baseURL    string
	adminToken string
	binaryPath string
	binaryProc *exec.Cmd
	runID      string
	tmpDir     string
)

const testPort = "18080"

const (
	e2ePostgresDSN = "postgres://lotty:lottypassword@localhost:5433/labp_e2e?sslmode=disable"
	e2eRedisURL    = "redis://localhost:6379/15"
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
	if err := cleanupTestState(); err != nil {
		fmt.Printf("cleanup: %v\n", err)
		return 1
	}

	runID = fmt.Sprintf("%d", time.Now().UnixNano())

	var err error
	tmpDir, err = os.MkdirTemp("", "panel-e2e-*")
	if err != nil {
		fmt.Printf("failed to create temp dir: %v\n", err)
		return 1
	}

	cfgPath := filepath.Join(tmpDir, "config.local.json")
	if err := os.WriteFile(cfgPath, fmt.Appendf(nil, `{
		"environment": "local",
		"auth": {
			"jwt": {"secret_key": "e2e-test-secret-not-for-prod", "ttl": "1h"},
			"bootstrap": {"username": "root", "email": "root@labp.net", "password": "root!@#$"}
		},
		"database": {
			"postgres": {"dsn": %q},
			"redis":    {"address": %q},
			"s3":       {"bucket": %q, "region": "us-east-1", "endpoint": %q, "access_key": "minioadmin", "secret_key": "minioadmin"}
		},
		"panel": {"http": {"address": "0.0.0.0:%s"}}
	}`, e2ePostgresDSN, e2eRedisURL, e2eS3Bucket, e2eS3Endpoint, testPort), 0o644); err != nil {
		fmt.Printf("failed to write config: %v\n", err)
		os.RemoveAll(tmpDir)
		return 1
	}

	binaryPath = filepath.Join(tmpDir, "panel")
	build := exec.Command("go", "build", "-o", binaryPath, "./services/panel/cmd")
	build.Dir = mustProjectRoot()
	build.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Printf("build failed: %v\n%s", err, out)
		os.RemoveAll(tmpDir)
		return 1
	}

	binaryProc = exec.Command(binaryPath)
	binaryProc.Dir = filepath.Dir(cfgPath)
	binaryProc.Env = append(os.Environ(), "CONFIG_NAME="+cfgPath)
	binaryProc.Stdout = os.Stdout
	binaryProc.Stderr = os.Stderr

	if err := binaryProc.Start(); err != nil {
		fmt.Printf("failed to start binary: %v\n", err)
		os.RemoveAll(tmpDir)
		return 1
	}

	baseURL = fmt.Sprintf("http://localhost:%s", testPort)

	waitForServer(baseURL + "/api/panel/v1/health")

	adminToken = login("root@labp.net", "root!@#$")
	if adminToken == "" {
		fmt.Println("failed to login as bootstrap admin")
		binaryProc.Process.Kill()
		binaryProc.Wait()
		os.RemoveAll(tmpDir)
		return 1
	}

	testCode := m.Run()

	binaryProc.Process.Kill()
	binaryProc.Wait()

	if err := cleanupTestState(); err != nil {
		fmt.Printf("post-suite cleanup: %v\n", err)
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
		return fmt.Errorf("Postgres database name %q does not end with _e2e. Refusing to truncate.", dbName)
	}

	return nil
}

func cleanupTestState() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, e2ePostgresDSN)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, "TRUNCATE users RESTART IDENTITY")
	if err != nil {
		return fmt.Errorf("truncate users: %w", err)
	}

	redisClient, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{"localhost:6379"},
		SelectDB:    15,
	})
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer redisClient.Close()

	err = redisClient.Do(ctx, redisClient.B().Flushdb().Build()).Error()
	if err != nil {
		return fmt.Errorf("flush redis db 15: %w", err)
	}

	return nil
}

func testEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@test.local", prefix, runID)
}

func testUsername(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, runID)
}

func createUser(t *testing.T, role, prefix string) (int64, string) {
	t.Helper()
	email := testEmail(prefix)
	body := jsonBody(map[string]string{
		"username": testUsername(prefix),
		"email":    email,
		"password": "testpass123",
		"role":     role,
	})
	resp := doRequest(http.MethodPost, "/api/panel/v1/users", adminToken, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("createUser(%s): expected 200, got %d", prefix, resp.StatusCode)
	}

	var result struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)

	return result.Data.ID, email
}

func login(email, password string) string {
	body := jsonBody(map[string]string{"email": email, "password": password})
	resp := doRequest(http.MethodPost, "/api/panel/v1/login", "", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Data.Token
}

func jsonBody(v interface{}) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func doRequest(method, path, token string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, baseURL+path, body)
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

func doMultipartRequest(path, token, fieldName, filename string, fileData []byte) *http.Response {
	body := new(bytes.Buffer)
	boundary := "----E2EBoundary"
	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldName, filename))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	body.Write(fileData)
	body.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	req, err := http.NewRequest(http.MethodPost, baseURL+path, body)
	if err != nil {
		panic(fmt.Sprintf("create request: %v", err))
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("do request: %v", err))
	}
	return resp
}

func decodeJSON(resp *http.Response, v interface{}) {
	json.NewDecoder(resp.Body).Decode(v)
}

type errorResponse struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func requireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, want)
	}
}

func requireJSONContentType(t *testing.T, resp *http.Response) {
	t.Helper()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

func requireErrorResponse(t *testing.T, resp *http.Response, wantStatus int, wantCode string) errorResponse {
	t.Helper()
	requireStatus(t, resp, wantStatus)
	requireJSONContentType(t, resp)

	var result errorResponse
	decodeJSON(resp, &result)

	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error.Code != wantCode {
		t.Errorf("error code: got %q, want %q", result.Error.Code, wantCode)
	}
	return result
}

func doRequestWithAuth(t *testing.T, method, path string, setAuth bool, authValue string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if setAuth {
		req.Header.Set("Authorization", authValue)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	return resp
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
