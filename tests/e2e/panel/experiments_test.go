package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func standardVariants() []any {
	return []any{
		map[string]any{"name": "control", "value": "off", "weight_bp": 5000, "is_control": true},
		map[string]any{"name": "treatment", "value": "on", "weight_bp": 5000, "is_control": false},
	}
}

func createExperiment(t *testing.T, token, flagID, name string) experimentResponseData {
	t.Helper()
	resp := doRequest(http.MethodPost, "/api/v1/panel/experiments", token,
		jsonBody(map[string]any{
			"flag_id": flagID,
			"name":    name + "-" + runID,
		}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeExperimentResponse(t, resp).Data
}

func setExperimentVariants(t *testing.T, token, id string, version int) experimentResponseData {
	t.Helper()
	resp := doRequest(http.MethodPut, fmt.Sprintf("/api/v1/panel/experiments/%s/variants", id), token,
		jsonBody(map[string]any{"version": version, "variants": standardVariants()}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeExperimentResponse(t, resp).Data
}

func experimentAction(t *testing.T, token, id, action string, payload map[string]any) experimentResponseData {
	t.Helper()
	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/%s", id, action), token, jsonBody(payload))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeExperimentResponse(t, resp).Data
}

func experimentTransition(t *testing.T, token, id, action string, version int) experimentResponseData {
	t.Helper()
	return experimentAction(t, token, id, action, map[string]any{"version": version})
}

func driveToRunning(t *testing.T, token, flagID, name string) experimentResponseData {
	t.Helper()
	exp := createExperiment(t, token, flagID, name)
	exp = setExperimentVariants(t, token, exp.ID, exp.Version)
	exp = experimentTransition(t, token, exp.ID, "submit", exp.Version)
	exp = approveViaReview(t, adminToken, exp.ID)
	exp = experimentTransition(t, token, exp.ID, "start", exp.Version)
	if exp.Status != "running" {
		t.Fatalf("expected running, got %q", exp.Status)
	}
	return exp
}

func createExperimenter(t *testing.T, prefix string) string {
	t.Helper()
	email := testEmail(prefix)
	createResp := doRequest(http.MethodPost, "/api/v1/panel/users", adminToken,
		jsonBody(map[string]string{
			"full_name": testFullName(prefix),
			"email":     email,
			"password":  "testpass123",
			"role":      "experimenter",
		}))
	defer createResp.Body.Close()
	requireStatus(t, createResp, http.StatusOK)

	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("could not login as experimenter")
	}
	return token
}

func createApprover(t *testing.T, prefix string) string {
	t.Helper()
	email := testEmail(prefix)
	createResp := doRequest(http.MethodPost, "/api/v1/panel/users", adminToken,
		jsonBody(map[string]string{
			"full_name": testFullName(prefix),
			"email":     email,
			"password":  "testpass123",
			"role":      "approver",
		}))
	defer createResp.Body.Close()
	requireStatus(t, createResp, http.StatusOK)

	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("could not login as approver")
	}
	return token
}

func TestExperiment_ApproverReviewsOthers(t *testing.T) {
	owner := createExperimenter(t, "exp-rev-owner")
	approver := createApprover(t, "exp-reviewer")
	other := createExperimenter(t, "exp-rev-other")
	flagID, _ := createFlag(t, "exp-review", "string", "off")

	exp := createExperiment(t, owner, flagID, "Review Others")
	exp = setExperimentVariants(t, owner, exp.ID, exp.Version)
	exp = experimentTransition(t, owner, exp.ID, "submit", exp.Version)
	if exp.Status != "review" {
		t.Fatalf("status: got %q, want review", exp.Status)
	}
	if exp.CurrentVersion == nil || exp.CurrentVersion.ReviewID == nil {
		t.Fatal("submitted experiment should link a review")
	}

	reviewID := *exp.CurrentVersion.ReviewID
	denied := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), other,
		jsonBody(map[string]any{"decision": "approve", "version": exp.Version}))
	defer denied.Body.Close()
	requireErrorResponse(t, denied, http.StatusForbidden, "FORBIDDEN")

	exp = approveViaReview(t, approver, exp.ID)
	if exp.Status != "approved" {
		t.Fatalf("status: got %q, want approved", exp.Status)
	}
}

func TestExperimentLifecycle(t *testing.T) {
	flagID, _ := createFlag(t, "exp-flag", "string", "off")

	exp := createExperiment(t, adminToken, flagID, "Lifecycle Experiment")
	if exp.Status != "draft" {
		t.Fatalf("status: got %q, want draft", exp.Status)
	}
	if exp.Version != 1 {
		t.Fatalf("version: got %d, want 1", exp.Version)
	}
	if exp.CurrentVersionID == nil {
		t.Fatal("current_version_id should be set")
	}
	if exp.CurrentVersion == nil || exp.CurrentVersion.VersionNum != 1 {
		t.Fatal("current version should be version 1")
	}
	saltV1 := exp.CurrentVersion.DistributionSalt
	if saltV1 == "" {
		t.Fatal("distribution salt should be set")
	}

	exp = setExperimentVariants(t, adminToken, exp.ID, exp.Version)
	if len(exp.Variants) != 2 {
		t.Fatalf("variants: got %d, want 2", len(exp.Variants))
	}

	exp = experimentTransition(t, adminToken, exp.ID, "submit", exp.Version)
	if exp.Status != "review" {
		t.Fatalf("status: got %q, want review", exp.Status)
	}

	exp = approveViaReview(t, adminToken, exp.ID)
	if exp.Status != "approved" {
		t.Fatalf("status: got %q, want approved", exp.Status)
	}

	exp = experimentTransition(t, adminToken, exp.ID, "start", exp.Version)
	if exp.Status != "running" {
		t.Fatalf("status: got %q, want running", exp.Status)
	}

	exp = experimentTransition(t, adminToken, exp.ID, "pause", exp.Version)
	if exp.Status != "paused" {
		t.Fatalf("status: got %q, want paused", exp.Status)
	}

	exp = experimentTransition(t, adminToken, exp.ID, "resume", exp.Version)
	if exp.Status != "running" {
		t.Fatalf("status: got %q, want running", exp.Status)
	}

	exp = experimentAction(t, adminToken, exp.ID, "complete", map[string]any{
		"version": exp.Version, "decision": "no_effect", "reason": "no meaningful impact",
	})
	if exp.Status != "completed" {
		t.Fatalf("status: got %q, want completed", exp.Status)
	}
	if exp.CompletionDecision == nil || *exp.CompletionDecision != "no_effect" {
		t.Errorf("completion decision not recorded: %+v", exp.CompletionDecision)
	}

	exp = experimentTransition(t, adminToken, exp.ID, "archive", exp.Version)
	if exp.Status != "archived" {
		t.Fatalf("status: got %q, want archived", exp.Status)
	}
}

func TestExperiment_InvalidTransitions(t *testing.T) {
	flagID, _ := createFlag(t, "exp-bad-trans", "string", "off")
	exp := createExperiment(t, adminToken, flagID, "Bad Transitions")

	for _, action := range []string{"start", "pause", "resume", "archive"} {
		resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/%s", exp.ID, action), adminToken,
			jsonBody(map[string]any{"version": exp.Version}))
		defer resp.Body.Close()
		requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
	}

	completeResp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/complete", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version, "decision": "no_effect", "reason": "x"}))
	defer completeResp.Body.Close()
	requireErrorResponse(t, completeResp, http.StatusConflict, "CONFLICT")
}

func TestExperiment_SubmitRequiresVariants(t *testing.T) {
	flagID, _ := createFlag(t, "exp-no-var", "string", "off")
	exp := createExperiment(t, adminToken, flagID, "No Variants")

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/submit", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY")
}

func TestExperiment_OneActivePerFlag(t *testing.T) {
	flagID, _ := createFlag(t, "exp-busy", "string", "off")

	first := driveToRunning(t, adminToken, flagID, "First Active")
	second := createExperiment(t, adminToken, flagID, "Second Active")
	second = setExperimentVariants(t, adminToken, second.ID, second.Version)
	second = experimentTransition(t, adminToken, second.ID, "submit", second.Version)
	second = approveViaReview(t, adminToken, second.ID)

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/start", second.ID), adminToken,
		jsonBody(map[string]any{"version": second.Version}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")

	_ = first
}

func TestExperiment_VersionConflict(t *testing.T) {
	flagID, _ := createFlag(t, "exp-stale", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Stale Version")
	stale := exp.Version

	exp = experimentTransition(t, adminToken, exp.ID, "pause", exp.Version)

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/resume", exp.ID), adminToken,
		jsonBody(map[string]any{"version": stale}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")
}

func TestExperiment_OwnerProtection(t *testing.T) {
	owner := createExperimenter(t, "exp-owner")
	other := createExperimenter(t, "exp-other")
	flagID, _ := createFlag(t, "exp-owned", "string", "off")

	exp := createExperiment(t, owner, flagID, "Owned Experiment")
	if exp.OwnerID == "" {
		t.Fatal("owner_id should be set")
	}

	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/experiments/%s", exp.ID), other,
		jsonBody(map[string]any{"version": exp.Version, "name": "Hijacked"}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")
}

func TestExperiment_NewVersionResetsApproval(t *testing.T) {
	flagID, _ := createFlag(t, "exp-newver", "string", "off")
	exp := createExperiment(t, adminToken, flagID, "New Version")
	exp = setExperimentVariants(t, adminToken, exp.ID, exp.Version)
	exp = experimentTransition(t, adminToken, exp.ID, "submit", exp.Version)
	exp = approveViaReview(t, adminToken, exp.ID)
	saltV1 := exp.CurrentVersion.DistributionSalt

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/versions", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version, "variants": standardVariants()}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	exp = decodeExperimentResponse(t, resp).Data

	if exp.Status != "draft" {
		t.Errorf("status: got %q, want draft after new version", exp.Status)
	}
	if exp.CurrentVersion == nil || exp.CurrentVersion.VersionNum != 2 {
		t.Fatalf("expected version 2, got %+v", exp.CurrentVersion)
	}
	if exp.CurrentVersion.DistributionSalt == saltV1 {
		t.Error("new version must have a fresh salt")
	}
	if len(exp.Variants) != 2 {
		t.Errorf("variants: got %d, want 2", len(exp.Variants))
	}

	runningResp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/versions", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version}))
	defer runningResp.Body.Close()
	requireStatus(t, runningResp, http.StatusOK)
}

func TestExperiment_RolloutWinner(t *testing.T) {
	flagID, _ := createFlag(t, "exp-rollout", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Rollout Winner")

	var winnerID string
	for _, v := range exp.Variants {
		if v.Name == "treatment" {
			winnerID = v.ID
		}
	}
	if winnerID == "" {
		t.Fatal("treatment variant not found")
	}

	exp = experimentAction(t, adminToken, exp.ID, "rollout", map[string]any{
		"version": exp.Version, "reason": "treatment wins", "winner_variant_id": winnerID,
	})
	if exp.Status != "completed" {
		t.Fatalf("status: got %q, want completed", exp.Status)
	}

	flagResp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/flags/%s", flagID), adminToken, nil)
	defer flagResp.Body.Close()
	requireStatus(t, flagResp, http.StatusOK)
	var flagResult struct {
		Data struct {
			DefaultValue string `json:"default_value"`
		} `json:"data"`
	}
	decodeJSON(flagResp, &flagResult)
	if flagResult.Data.DefaultValue != `on` {
		t.Errorf("flag default: got %s, want on", flagResult.Data.DefaultValue)
	}
}

func TestExperiment_GuardrailPauseAndAdminOverride(t *testing.T) {
	owner := createExperimenter(t, "exp-guard")
	flagID, _ := createFlag(t, "exp-guard", "string", "off")
	exp := driveToRunning(t, owner, flagID, "Guardrail Pause")

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/internal/experiments/%s/pause", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	exp = decodeExperimentResponse(t, resp).Data
	if exp.Status != "paused" {
		t.Fatalf("status: got %q, want paused", exp.Status)
	}
	if !exp.GuardrailPaused {
		t.Error("guardrail_paused should be true")
	}

	ownerResume := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/resume", exp.ID), owner,
		jsonBody(map[string]any{"version": exp.Version}))
	defer ownerResume.Body.Close()
	requireErrorResponse(t, ownerResume, http.StatusForbidden, "FORBIDDEN")

	exp = experimentTransition(t, adminToken, exp.ID, "resume", exp.Version)
	if exp.Status != "running" {
		t.Fatalf("status: got %q, want running", exp.Status)
	}
	if exp.GuardrailPaused {
		t.Error("guardrail_paused should be cleared after admin resume")
	}
}

func TestExperiment_ListFilter(t *testing.T) {
	flagID, _ := createFlag(t, "exp-list", "string", "off")
	exp := createExperiment(t, adminToken, flagID, "List Filter")

	resp := doRequest(http.MethodGet, "/api/v1/panel/experiments?limit=100&status=draft", adminToken, nil)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	result := decodePaginatedExperimentsResponse(t, resp)

	found := false
	for _, e := range result.Data {
		if e.ID == exp.ID {
			found = true
		}
		if e.Status != "draft" {
			t.Errorf("status filter violated: got %q", e.Status)
		}
	}
	if !found {
		t.Error("created experiment not found in filtered list")
	}
}
