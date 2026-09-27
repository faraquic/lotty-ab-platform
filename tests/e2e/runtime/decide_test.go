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

func waitForDecideWithAttrs(t *testing.T, subject string, attrs map[string]any, flag, wantSource string) decideResponse {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		resp := runtimeRequest(http.MethodPost, "/api/v1/runtime/decide", jsonBody(map[string]any{
			"subject_id": subject,
			"attributes": attrs,
			"flags":      []string{flag},
		}))
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(500 * time.Millisecond)
			continue
		}
		var result decideResponse
		decodeJSON(resp, &result)
		if result.Data.Flags[flag].Source == wantSource {
			return result
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("flag %q did not reach source %q within 25s", flag, wantSource)
	return decideResponse{}
}

func waitForDecisionReason(t *testing.T, subject, flag, wantReason string) decideResponse {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		result := decide(t, subject, flag)
		if result.Data.Flags[flag].Reason == wantReason {
			return result
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("flag %q did not reach reason %q within 25s", flag, wantReason)
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

func driveExperimentToRunningWith(t *testing.T, flagID, name string, targeting *string) map[string]any {
	t.Helper()
	payload := map[string]any{
		"flag_id": flagID,
		"name":    fmt.Sprintf("%s-%s", name, runID),
	}
	if targeting != nil {
		payload["targeting"] = *targeting
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

func TestExperimentPipelineWithAndWithoutTargeting(t *testing.T) {
	flagKey := "rt-pipeline-dsl-" + runID
	flagID := createFlag(t, "rt-pipeline-dsl", "string", "off")
	flagData := panelAction(t, http.MethodGet, "/api/v1/panel/flags/"+flagID, adminToken, nil)
	if flagData["id"] != flagID || flagData["key"] != flagKey {
		t.Fatalf("created flag was not returned by GET: %+v", flagData)
	}

	targeting := `country == "DE"`
	experiment := panelAction(t, http.MethodPost, "/api/v1/panel/experiments", adminToken, map[string]any{
		"flag_id":   flagID,
		"name":      "Pipeline DSL-" + runID,
		"targeting": targeting,
	})
	experimentID := experiment["id"].(string)
	version := int(experiment["version"].(float64))
	variants := []any{
		map[string]any{"name": "control", "value": "off", "weight_bp": 5000, "is_control": true},
		map[string]any{"name": "treatment", "value": "on", "weight_bp": 5000, "is_control": false},
	}
	experiment = panelAction(t, http.MethodPut, fmt.Sprintf("/api/v1/panel/experiments/%s/variants", experimentID), adminToken, map[string]any{
		"version":  version,
		"variants": variants,
	})
	currentVersion := experiment["current_version"].(map[string]any)
	if currentVersion["targeting"] != targeting {
		t.Fatalf("targeting: got %v, want %q", currentVersion["targeting"], targeting)
	}

	experiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/submit", experimentID), adminToken,
		map[string]any{"version": experiment["version"]})
	if experiment["status"] != "review" {
		t.Fatalf("status after submit: got %v, want review", experiment["status"])
	}
	reviewID := currentReviewID(t, experimentID)
	review := panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), adminToken,
		map[string]any{"decision": "request_changes", "version": experiment["version"]})
	if review["status"] != "changes_requested" {
		t.Fatalf("review status: got %v, want changes_requested", review["status"])
	}
	experiment = panelAction(t, http.MethodGet, fmt.Sprintf("/api/v1/panel/experiments/%s", experimentID), adminToken, nil)
	if experiment["status"] != "draft" {
		t.Fatalf("experiment status after return: got %v, want draft", experiment["status"])
	}

	experiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/versions", experimentID), adminToken, map[string]any{
		"version":   experiment["version"],
		"targeting": targeting,
		"variants":  variants,
	})
	versionData := experiment["current_version"].(map[string]any)
	if versionData["version_num"] != float64(2) || versionData["targeting"] != targeting {
		t.Fatalf("revised version did not preserve DSL: %+v", versionData)
	}
	experiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/submit", experimentID), adminToken,
		map[string]any{"version": experiment["version"]})
	reviewID = currentReviewID(t, experimentID)
	panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), adminToken,
		map[string]any{"decision": "approve", "version": experiment["version"]})
	experiment = panelAction(t, http.MethodGet, fmt.Sprintf("/api/v1/panel/experiments/%s", experimentID), adminToken, nil)
	if experiment["status"] != "approved" {
		t.Fatalf("status after approval: got %v, want approved", experiment["status"])
	}
	experiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/start", experimentID), adminToken,
		map[string]any{"version": experiment["version"]})
	if experiment["status"] != "running" {
		t.Fatalf("status after start: got %v, want running", experiment["status"])
	}

	waitForDecideWithAttrs(t, "pipeline-dsl-wait", map[string]any{"country": "DE"}, flagKey, "experiment")
	result := waitForDecideWithAttrs(t, "pipeline-dsl-subject", map[string]any{"country": "US"}, flagKey, "default")
	if result.Data.Flags[flagKey].Source != "default" || result.Data.Flags[flagKey].Value != "off" {
		t.Fatalf("unmatched DSL decision: %+v", result.Data.Flags[flagKey])
	}

	plainFlagKey := "rt-pipeline-plain-" + runID
	plainFlagID := createFlag(t, "rt-pipeline-plain", "string", "off")
	plainFlag := panelAction(t, http.MethodGet, "/api/v1/panel/flags/"+plainFlagID, adminToken, nil)
	if plainFlag["id"] != plainFlagID || plainFlag["key"] != plainFlagKey {
		t.Fatalf("created flag was not returned by GET: %+v", plainFlag)
	}
	plainExperiment := driveExperimentToRunning(t, plainFlagID, "Pipeline Without Targeting")
	plainExperimentID := plainExperiment["id"].(string)
	plainVersion := panelAction(t, http.MethodGet, fmt.Sprintf("/api/v1/panel/experiments/%s", plainExperimentID), adminToken, nil)["current_version"].(map[string]any)
	if plainVersion["targeting"] != nil {
		t.Fatalf("untargeted experiment should have no targeting, got %v", plainVersion["targeting"])
	}
	plainResult := waitForDecide(t, "pipeline-plain-subject", plainFlagKey, "experiment")
	if got := plainResult.Data.Flags[plainFlagKey].ExperimentID; got != plainExperimentID {
		t.Fatalf("untargeted decide experiment_id: got %q, want %q", got, plainExperimentID)
	}
	plainExperiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/pause", plainExperimentID), adminToken,
		map[string]any{"version": plainExperiment["version"]})
	if plainExperiment["status"] != "paused" {
		t.Fatalf("status after pause: got %v, want paused", plainExperiment["status"])
	}
	plainResult = waitForDecide(t, "pipeline-plain-subject", plainFlagKey, "default")
	if plainResult.Data.Flags[plainFlagKey].Value != "off" {
		t.Fatalf("paused flag value: got %v, want off", plainResult.Data.Flags[plainFlagKey].Value)
	}
	plainExperiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/resume", plainExperimentID), adminToken,
		map[string]any{"version": plainExperiment["version"]})
	if plainExperiment["status"] != "running" {
		t.Fatalf("status after resume: got %v, want running", plainExperiment["status"])
	}
	waitForDecide(t, "pipeline-plain-subject", plainFlagKey, "experiment")
	plainExperiment = panelAction(t, http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/complete", plainExperimentID), adminToken, map[string]any{
		"version":  plainExperiment["version"],
		"decision": "no_effect",
		"reason":   "pipeline e2e complete",
	})
	if plainExperiment["status"] != "completed" {
		t.Fatalf("status after complete: got %v, want completed", plainExperiment["status"])
	}
	waitForDecide(t, "pipeline-plain-subject", plainFlagKey, "default")
}

func TestDecideEmptyTargetingMatches(t *testing.T) {
	flagKey := "rt-empty-tg-" + runID
	flagID := createFlag(t, "rt-empty-tg", "string", "off")
	empty := ""
	driveExperimentToRunningWith(t, flagID, "Empty Targeting", &empty)

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

func TestDecideTargetingEvaluation(t *testing.T) {
	flagKey := "rt-targeting-eval-" + runID
	flagID := createFlag(t, "rt-targeting-eval", "string", "off")
	targeting := `country == "DE" AND age >= 18`
	driveExperimentToRunningWith(t, flagID, "Targeting Eval", &targeting)

	matched := waitForDecideWithAttrs(t, "rt-tg-match", map[string]any{"country": "DE", "age": 25}, flagKey, "experiment")
	if matched.Data.Flags[flagKey].VariantID == "" {
		t.Error("matched subject should get variant_id")
	}

	result := waitForDecideWithAttrs(t, "rt-tg-nomatch", map[string]any{"country": "US", "age": 25}, flagKey, "default")
	if result.Data.Flags[flagKey].Reason != "targeting mismatch" {
		t.Errorf("non-matching subject: reason=%q, want targeting mismatch", result.Data.Flags[flagKey].Reason)
	}
}

func TestDecideTargetingInNotIn(t *testing.T) {
	flagKey := "rt-targeting-in-" + runID
	flagID := createFlag(t, "rt-targeting-in", "string", "off")
	targeting := `plan IN ["free", "pro"]`
	driveExperimentToRunningWith(t, flagID, "Targeting IN", &targeting)

	matched := waitForDecideWithAttrs(t, "rt-tg-in-match", map[string]any{"plan": "free"}, flagKey, "experiment")
	if matched.Data.Flags[flagKey].VariantID == "" {
		t.Error("IN match should get variant_id")
	}

	result := waitForDecideWithAttrs(t, "rt-tg-in-nomatch", map[string]any{"plan": "enterprise"}, flagKey, "default")
	if result.Data.Flags[flagKey].Source != "default" {
		t.Errorf("IN non-match: source=%q, want default", result.Data.Flags[flagKey].Source)
	}
}

func TestDecideTargetingNotIn(t *testing.T) {
	flagKey := "rt-targeting-notin-" + runID
	flagID := createFlag(t, "rt-targeting-notin", "string", "off")
	targeting := `plan NOT IN ["free"]`
	driveExperimentToRunningWith(t, flagID, "Targeting NOT IN", &targeting)

	matched := waitForDecideWithAttrs(t, "rt-tg-notin-match", map[string]any{"plan": "pro"}, flagKey, "experiment")
	if matched.Data.Flags[flagKey].VariantID == "" {
		t.Error("NOT IN match should get variant_id")
	}

	result := waitForDecideWithAttrs(t, "rt-tg-notin-nomatch", map[string]any{"plan": "free"}, flagKey, "default")
	if result.Data.Flags[flagKey].Source != "default" {
		t.Errorf("NOT IN non-match: source=%q, want default", result.Data.Flags[flagKey].Source)
	}
}

func TestDecideTargetingLogicalOperators(t *testing.T) {
	flagKey := "rt-targeting-logic-" + runID
	flagID := createFlag(t, "rt-targeting-logic", "string", "off")
	targeting := `country == "DE" OR country == "US"`
	driveExperimentToRunningWith(t, flagID, "Targeting Logic", &targeting)

	matched := waitForDecideWithAttrs(t, "rt-tg-logic-match", map[string]any{"country": "US"}, flagKey, "experiment")
	if matched.Data.Flags[flagKey].VariantID == "" {
		t.Error("OR match should get variant_id")
	}

	result := waitForDecideWithAttrs(t, "rt-tg-logic-nomatch", map[string]any{"country": "FR"}, flagKey, "default")
	if result.Data.Flags[flagKey].Source != "default" {
		t.Errorf("OR non-match: source=%q, want default", result.Data.Flags[flagKey].Source)
	}
}

func TestDecideTargetingNotOperator(t *testing.T) {
	flagKey := "rt-targeting-not-" + runID
	flagID := createFlag(t, "rt-targeting-not", "string", "off")
	targeting := `NOT country == "DE"`
	driveExperimentToRunningWith(t, flagID, "Targeting NOT", &targeting)

	matched := waitForDecideWithAttrs(t, "rt-tg-not-match", map[string]any{"country": "US"}, flagKey, "experiment")
	if matched.Data.Flags[flagKey].VariantID == "" {
		t.Error("NOT match should get variant_id")
	}

	result := waitForDecideWithAttrs(t, "rt-tg-not-nomatch", map[string]any{"country": "DE"}, flagKey, "default")
	if result.Data.Flags[flagKey].Source != "default" {
		t.Errorf("NOT non-match: source=%q, want default", result.Data.Flags[flagKey].Source)
	}
}

func TestDecideTargetingMissingAttribute(t *testing.T) {
	flagKey := "rt-targeting-missing-" + runID
	flagID := createFlag(t, "rt-targeting-missing", "string", "off")
	targeting := `country == "DE"`
	driveExperimentToRunningWith(t, flagID, "Targeting Missing", &targeting)

	waitForDecideWithAttrs(t, "rt-tg-missing-wait", map[string]any{"country": "DE"}, flagKey, "experiment")
	result := waitForDecideWithAttrs(t, "rt-tg-missing-attr", map[string]any{}, flagKey, "default")
	if result.Data.Flags[flagKey].Source != "default" {
		t.Errorf("missing attribute: source=%q, want default", result.Data.Flags[flagKey].Source)
	}
	if result.Data.Flags[flagKey].Reason != "targeting mismatch" {
		t.Errorf("missing attribute: reason=%q, want targeting mismatch", result.Data.Flags[flagKey].Reason)
	}
}

func TestDecideTargetingTypeCoercion(t *testing.T) {
	flagKey := "rt-targeting-coerce-" + runID
	flagID := createFlag(t, "rt-targeting-coerce", "string", "off")
	targeting := `age >= 18`
	driveExperimentToRunningWith(t, flagID, "Targeting Coerce", &targeting)

	result := waitForDecideWithAttrs(t, "rt-tg-coerce", map[string]any{"age": "25"}, flagKey, "experiment")
	if result.Data.Flags[flagKey].Source != "experiment" {
		t.Errorf("type coercion: source=%q, want experiment", result.Data.Flags[flagKey].Source)
	}
}
