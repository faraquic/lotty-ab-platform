package e2e

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type decideFlag struct {
	Value             any    `json:"value"`
	Source            string `json:"source"`
	Reason            string `json:"reason"`
	ExperimentID      string `json:"experiment_id"`
	ExperimentVersion int    `json:"experiment_version"`
	VariantID         string `json:"variant_id"`
	DecisionID        string `json:"decision_id"`
}

type decideResponse struct {
	Success bool `json:"success"`
	Data    struct {
		RequestID      string                `json:"request_id"`
		ConfigRevision string                `json:"config_revision"`
		Degraded       bool                  `json:"degraded"`
		Flags          map[string]decideFlag `json:"flags"`
	} `json:"data"`
}

func decide(t *testing.T, subject string, flags ...string) decideResponse {
	t.Helper()
	resp := runtimeRequest(http.MethodPost, "/api/v1/runtime/decide", jsonBody(map[string]any{
		"subject_id": subject,
		"attributes": map[string]any{"country": "DE"},
		"flags":      flags,
	}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("decide: got %d, want 200", resp.StatusCode)
	}
	var result decideResponse
	decodeJSON(resp, &result)
	if !result.Success {
		t.Fatal("expected success=true")
	}
	return result
}

func waitForDecide(t *testing.T, subject, flag, wantSource string) decideResponse {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		result := decide(t, subject, flag)
		if result.Data.Flags[flag].Source == wantSource {
			return result
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("flag %q did not reach source %q within 25s", flag, wantSource)
	return decideResponse{}
}

func createFlag(t *testing.T, key, flagType string, defaultValue any) string {
	t.Helper()
	uniqueKey := fmt.Sprintf("%s-%s", key, runID)
	resp := panelRequest(http.MethodPost, "/api/v1/panel/flags", adminToken,
		jsonBody(map[string]any{
			"key":           uniqueKey,
			"name":          uniqueKey,
			"type":          flagType,
			"default_value": defaultValue,
		}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("createFlag: got %d, want 200", resp.StatusCode)
	}
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)
	return result.Data.ID
}

func panelAction(t *testing.T, method, path, token string, payload map[string]any) map[string]any {
	t.Helper()
	var body io.Reader
	if payload != nil {
		body = jsonBody(payload)
	}
	resp := panelRequest(method, path, token, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: got %d, want 200", method, path, resp.StatusCode)
	}
	var result struct {
		Data map[string]any `json:"data"`
	}
	decodeJSON(resp, &result)
	return result.Data
}

func driveExperimentToRunning(t *testing.T, flagID, name string) map[string]any {
	t.Helper()
	return driveExperimentToRunningWith(t, flagID, name, nil)
}

func driveExperimentToRunningWith(t *testing.T, flagID, name string, targeting map[string]any) map[string]any {
	t.Helper()
	payload := map[string]any{
		"flag_id": flagID,
		"name":    fmt.Sprintf("%s-%s", name, runID),
	}
	if targeting != nil {
		payload["targeting"] = targeting
	}
	data := panelAction(t, http.MethodPost, "/api/v1/panel/experiments", adminToken, payload)
	id := data["id"].(string)
	version := int(data["version"].(float64))

	data = panelAction(t, http.MethodPut, fmt.Sprintf("/api/v1/panel/experiments/%s/variants", id), adminToken, map[string]any{
		"version": version,
		"variants": []any{
			map[string]any{"name": "control", "value": "off", "weight_bp": 5000, "is_control": true},
			map[string]any{"name": "treatment", "value": "on", "weight_bp": 5000, "is_control": false},
		},
	})
	version = int(data["version"].(float64))

	for _, action := range []string{"submit", "start"} {
		if action == "submit" {
			data = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/%s", id, action), adminToken,
				map[string]any{"version": version})
			version = int(data["version"].(float64))
			continue
		}
		reviewID := currentReviewID(t, id)
		panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), adminToken,
			map[string]any{"decision": "approve", "version": version})
		data = panelAction(t, http.MethodGet, fmt.Sprintf("/api/v1/panel/experiments/%s", id), adminToken, nil)
		version = int(data["version"].(float64))
		data = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/%s", id, action), adminToken,
			map[string]any{"version": version})
		version = int(data["version"].(float64))
	}
	if data["status"] != "running" {
		t.Fatalf("status: got %v, want running", data["status"])
	}
	return data
}

func currentReviewID(t *testing.T, expID string) string {
	t.Helper()
	resp := runtimePanelGet(t, fmt.Sprintf("/api/v1/panel/experiments/%s", expID))
	var result struct {
		Data struct {
			CurrentVersion struct {
				ReviewID *string `json:"review_id"`
			} `json:"current_version"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)
	if result.Data.CurrentVersion.ReviewID == nil {
		t.Fatal("experiment has no linked review")
	}
	return *result.Data.CurrentVersion.ReviewID
}

func runtimePanelGet(t *testing.T, path string) *http.Response {
	t.Helper()
	resp := panelRequest(http.MethodGet, path, adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("GET %s: got %d, want 200", path, resp.StatusCode)
	}
	return resp
}

func TestDecideExperimentFlow(t *testing.T) {
	flagID := createFlag(t, "rt-exp", "string", "off")
	exp := driveExperimentToRunning(t, flagID, "Runtime Flow")
	expID := exp["id"].(string)

	result := waitForDecide(t, "rt-user-1", "rt-exp-"+runID, "experiment")
	d := result.Data.Flags["rt-exp-"+runID]
	if d.ExperimentID != expID {
		t.Errorf("experiment_id: got %q, want %q", d.ExperimentID, expID)
	}
	if d.ExperimentVersion != 1 {
		t.Errorf("experiment_version: got %d, want 1", d.ExperimentVersion)
	}
	if d.VariantID == "" {
		t.Error("variant_id should be set")
	}
	if _, err := uuid.Parse(d.DecisionID); err != nil {
		t.Errorf("decision_id should be a uuid: %q", d.DecisionID)
	}
	if d.Value != "off" && d.Value != "on" {
		t.Errorf("value: got %v, want on/off", d.Value)
	}
	if result.Data.ConfigRevision == "" {
		t.Error("config_revision should be set")
	}

	again := decide(t, "rt-user-1", "rt-exp-"+runID)
	againFlag := again.Data.Flags["rt-exp-"+runID]
	if againFlag.VariantID != d.VariantID || againFlag.Value != d.Value {
		t.Errorf("not sticky: %+v vs %+v", d, againFlag)
	}
	if againFlag.DecisionID == d.DecisionID {
		t.Error("decision_id should be unique per decision")
	}
}

func TestDecidePauseReturnsDefault(t *testing.T) {
	flagKey := "rt-pause-" + runID
	flagID := createFlag(t, "rt-pause", "string", "off")
	exp := driveExperimentToRunning(t, flagID, "Runtime Pause")
	expID := exp["id"].(string)
	version := int(exp["version"].(float64))

	waitForDecide(t, "rt-user-2", flagKey, "experiment")

	panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/pause", expID), adminToken,
		map[string]any{"version": version})

	result := waitForDecide(t, "rt-user-2", flagKey, "default")
	if result.Data.Flags[flagKey].Value != "off" {
		t.Errorf("value: got %v, want off", result.Data.Flags[flagKey].Value)
	}
}

func TestDecideEmptyTargetingMatches(t *testing.T) {
	flagKey := "rt-empty-tg-" + runID
	flagID := createFlag(t, "rt-empty-tg", "string", "off")
	driveExperimentToRunningWith(t, flagID, "Empty Targeting", map[string]any{})

	result := waitForDecide(t, "rt-user-5", flagKey, "experiment")
	if result.Data.Flags[flagKey].VariantID == "" {
		t.Error("variant_id should be set for empty targeting")
	}
}

func TestDecideUnknownFlag(t *testing.T) {
	resp := runtimeRequest(http.MethodPost, "/api/v1/runtime/decide", jsonBody(map[string]any{
		"subject_id": "rt-user-3",
		"flags":      []string{"rt-missing-" + runID},
	}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestDecideDegraded(t *testing.T) {
	flagKey := "rt-degraded-" + runID
	createFlag(t, "rt-degraded", "string", "off")
	waitForKnownFlag(t, flagKey)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		result := decide(t, "rt-user-4", flagKey)
		if result.Data.Degraded {
			if result.Data.Flags[flagKey].Source != "default" {
				t.Fatalf("source: got %q, want default", result.Data.Flags[flagKey].Source)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("degraded did not become true within 15s (max_stale_age=1s)")
}

func waitForKnownFlag(t *testing.T, flag string) {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		resp := runtimeRequest(http.MethodPost, "/api/v1/runtime/decide", jsonBody(map[string]any{
			"subject_id": "rt-waiter",
			"flags":      []string{flag},
		}))
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("flag %q did not propagate within 25s", flag)
}
