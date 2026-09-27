package e2e

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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

func requireExperimentAuditRecord(t *testing.T, experimentID, action, beforeStatus, afterStatus, reason string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, e2ePostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var before, after, requestID string
	var gotReason *string
	err = pool.QueryRow(ctx, `
SELECT
    COALESCE(before_state->>'status', ''),
    after_state->>'status',
    request_id::text,
    reason
FROM
    audit_records
WHERE
    resource_type = 'experiment'
    AND resource_id = $1
    AND action = $2
ORDER BY
    created_at DESC,
    id DESC
LIMIT 1`, experimentID, action).Scan(&before, &after, &requestID, &gotReason)
	if err != nil {
		t.Fatalf("read audit record %q: %v", action, err)
	}
	if before != beforeStatus || after != afterStatus {
		t.Errorf("audit %q: got %q -> %q, want %q -> %q", action, before, after, beforeStatus, afterStatus)
	}
	if requestID == "" {
		t.Errorf("audit %q has no request_id", action)
	}
	if reason == "" && gotReason != nil || reason != "" && (gotReason == nil || *gotReason != reason) {
		t.Errorf("audit %q reason: got %v, want %q", action, gotReason, reason)
	}
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
	if value := readFlagDefault(t, flagID); value != "off" {
		t.Errorf("flag default after no_effect completion: got %s, want off", value)
	}

	exp = experimentTransition(t, adminToken, exp.ID, "archive", exp.Version)
	if exp.Status != "archived" {
		t.Fatalf("status: got %q, want archived", exp.Status)
	}
}

func TestExperiment_StateTransitionsAreAudited(t *testing.T) {
	flagID, _ := createFlag(t, "exp-audit", "string", "off")
	exp := createExperiment(t, adminToken, flagID, "Audit Lifecycle")
	exp = setExperimentVariants(t, adminToken, exp.ID, exp.Version)
	exp = experimentTransition(t, adminToken, exp.ID, "submit", exp.Version)
	exp = approveViaReview(t, adminToken, exp.ID)
	exp = experimentTransition(t, adminToken, exp.ID, "start", exp.Version)
	exp = experimentTransition(t, adminToken, exp.ID, "pause", exp.Version)
	exp = experimentTransition(t, adminToken, exp.ID, "resume", exp.Version)
	exp = experimentAction(t, adminToken, exp.ID, "complete", map[string]any{
		"version": exp.Version, "decision": "no_effect", "reason": "no meaningful impact",
	})
	exp = experimentTransition(t, adminToken, exp.ID, "archive", exp.Version)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, e2ePostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
SELECT
    COALESCE(before_state->>'status', ''),
    after_state->>'status',
    request_id::text,
    action,
    reason
FROM
    audit_records
WHERE
    resource_type = 'experiment'
    AND resource_id = $1
ORDER BY
    created_at, id`, exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	want := [][2]string{
		{"", "draft"},
		{"draft", "review"},
		{"review", "approved"},
		{"approved", "running"},
		{"running", "paused"},
		{"paused", "running"},
		{"running", "completed"},
		{"completed", "archived"},
	}
	index := 0
	for rows.Next() {
		var before, after, requestID, action string
		var reason *string
		if err := rows.Scan(&before, &after, &requestID, &action, &reason); err != nil {
			t.Fatal(err)
		}
		if index >= len(want) {
			t.Fatalf("unexpected extra audit record for experiment %s", exp.ID)
		}
		if before != want[index][0] || after != want[index][1] {
			t.Errorf("audit transition %d: got %q -> %q, want %q -> %q", index, before, after, want[index][0], want[index][1])
		}
		if requestID == "" {
			t.Errorf("audit transition %d has no request_id", index)
		}
		if index == 6 && (action != "experiment.completed.no_effect" || reason == nil || *reason != "no meaningful impact") {
			t.Errorf("completion audit: action=%q reason=%v", action, reason)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(want) {
		t.Fatalf("audit records: got %d, want %d", index, len(want))
	}
	if _, err := pool.Exec(ctx, `UPDATE audit_records SET action = action WHERE resource_type = 'experiment' AND resource_id = $1`, exp.ID); err == nil {
		t.Fatal("audit records must reject updates")
	}
}

func TestExperiment_CompletionRequiresReason(t *testing.T) {
	flagID, _ := createFlag(t, "exp-completion-reason", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Completion Reason")

	requests := []map[string]any{
		{"version": exp.Version, "decision": "no_effect"},
		{"version": exp.Version, "decision": "no_effect", "reason": ""},
		{"version": exp.Version, "decision": "no_effect", "reason": " \t\n "},
	}
	for _, payload := range requests {
		resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/complete", exp.ID), adminToken, jsonBody(payload))
		requireErrorResponse(t, resp, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY")
		resp.Body.Close()
	}

	current := getExperiment(t, exp.ID)
	if current.Status != "running" {
		t.Fatalf("status after rejected completion: got %q, want running", current.Status)
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
	requireExperimentAuditRecord(t, exp.ID, "experiment.version_created_status_reset", "approved", "draft", "")
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

func readFlagDefault(t *testing.T, flagID string) string {
	t.Helper()
	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/flags/%s", flagID), adminToken, nil)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	var result struct {
		Data struct {
			DefaultValue string `json:"default_value"`
		} `json:"data"`
	}
	decodeJSON(resp, &result)
	return result.Data.DefaultValue
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
	if exp.CompletionDecision == nil || *exp.CompletionDecision != "rollout_winner" {
		t.Errorf("completion decision: got %v, want rollout_winner", exp.CompletionDecision)
	}
	if exp.CompletionReason == nil || *exp.CompletionReason != "treatment wins" {
		t.Errorf("completion reason: got %v, want %q", exp.CompletionReason, "treatment wins")
	}
	requireExperimentAuditRecord(t, exp.ID, "experiment.completed.rollout_winner", "running", "completed", "treatment wins")

	if value := readFlagDefault(t, flagID); value != "on" {
		t.Errorf("flag default: got %s, want on", value)
	}
}

func TestExperiment_CompleteRejectsRolloutWinnerDecision(t *testing.T) {
	flagID, _ := createFlag(t, "exp-complete-no-rollout", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Complete Rejects Rollout")

	var winnerID string
	for _, v := range exp.Variants {
		if v.Name == "treatment" {
			winnerID = v.ID
		}
	}

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/complete", exp.ID), adminToken,
		jsonBody(map[string]any{
			"version": exp.Version, "decision": "rollout_winner", "reason": "should be rejected",
			"winner_variant_id": winnerID,
		}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")

	current := getExperiment(t, exp.ID)
	if current.Status != "running" {
		t.Fatalf("status after rejected completion: got %q, want running", current.Status)
	}
	if value := readFlagDefault(t, flagID); value != "off" {
		t.Errorf("flag default after rejected completion: got %s, want off", value)
	}
}

func TestExperiment_RolloutWinnerRejectsBadRequests(t *testing.T) {
	flagID, _ := createFlag(t, "exp-rollout-bad", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Rollout Winner Bad Requests")

	invalidRequests := []map[string]any{
		{"version": exp.Version, "reason": "missing winner"},
		{"version": exp.Version, "reason": "empty winner", "winner_variant_id": ""},
	}
	for _, payload := range invalidRequests {
		resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/rollout", exp.ID), adminToken, jsonBody(payload))
		requireErrorResponse(t, resp, http.StatusBadRequest, "BAD_REQUEST")
		resp.Body.Close()
	}

	unknownWinner := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/rollout", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version, "reason": "unknown winner", "winner_variant_id": "00000000-0000-7000-8000-000000000000"}))
	defer unknownWinner.Body.Close()
	requireErrorResponse(t, unknownWinner, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY")

	current := getExperiment(t, exp.ID)
	if current.Status != "running" {
		t.Fatalf("status after rejected rollouts: got %q, want running", current.Status)
	}
	if value := readFlagDefault(t, flagID); value != "off" {
		t.Errorf("flag default after rejected rollouts: got %s, want off", value)
	}
}

func TestExperiment_RolloutWinnerStaleVersion(t *testing.T) {
	flagID, _ := createFlag(t, "exp-rollout-stale", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Rollout Winner Stale Version")
	stale := exp.Version

	exp = experimentTransition(t, adminToken, exp.ID, "pause", exp.Version)

	var winnerID string
	for _, v := range exp.Variants {
		if v.Name == "treatment" {
			winnerID = v.ID
		}
	}

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/rollout", exp.ID), adminToken,
		jsonBody(map[string]any{"version": stale, "reason": "stale", "winner_variant_id": winnerID}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")

	current := getExperiment(t, exp.ID)
	if current.Status != "paused" {
		t.Fatalf("status after stale rollout: got %q, want paused", current.Status)
	}
	if value := readFlagDefault(t, flagID); value != "off" {
		t.Errorf("flag default after stale rollout: got %s, want off", value)
	}
}

func TestExperiment_CompleteStaleVersion(t *testing.T) {
	for _, decision := range []string{"rollback", "no_effect"} {
		flagID, _ := createFlag(t, "exp-complete-stale-"+decision, "string", "off")
		exp := driveToRunning(t, adminToken, flagID, "Complete Stale "+decision)
		stale := exp.Version

		exp = experimentTransition(t, adminToken, exp.ID, "pause", exp.Version)

		resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/complete", exp.ID), adminToken,
			jsonBody(map[string]any{"version": stale, "decision": decision, "reason": "stale"}))
		defer resp.Body.Close()
		requireErrorResponse(t, resp, http.StatusConflict, "CONFLICT")

		current := getExperiment(t, exp.ID)
		if current.Status != "paused" {
			t.Fatalf("%s: status after stale completion: got %q, want paused", decision, current.Status)
		}
		if value := readFlagDefault(t, flagID); value != "off" {
			t.Errorf("%s: flag default after stale completion: got %s, want off", decision, value)
		}
	}
}

func TestExperiment_NoEffectKeepsDefault(t *testing.T) {
	flagID, _ := createFlag(t, "exp-no-effect", "string", "fallback")
	exp := driveToRunning(t, adminToken, flagID, "No Effect Keeps Default")

	exp = experimentAction(t, adminToken, exp.ID, "complete", map[string]any{
		"version": exp.Version, "decision": "no_effect", "reason": "no meaningful impact",
	})
	if exp.Status != "completed" {
		t.Fatalf("status: got %q, want completed", exp.Status)
	}
	if exp.CompletionDecision == nil || *exp.CompletionDecision != "no_effect" {
		t.Errorf("completion decision: got %v, want no_effect", exp.CompletionDecision)
	}
	if value := readFlagDefault(t, flagID); value != "fallback" {
		t.Errorf("flag default after no_effect: got %s, want fallback", value)
	}
}

func TestExperiment_ConcurrentRollout(t *testing.T) {
	flagID, _ := createFlag(t, "exp-rollout-race", "string", "off")
	exp := driveToRunning(t, adminToken, flagID, "Concurrent Rollout")

	var winnerID string
	for _, v := range exp.Variants {
		if v.Name == "treatment" {
			winnerID = v.ID
		}
	}

	const attempts = 8
	var wg sync.WaitGroup
	statuses := make(chan int, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/rollout", exp.ID), adminToken,
				jsonBody(map[string]any{"version": exp.Version, "reason": "race", "winner_variant_id": winnerID}))
			defer resp.Body.Close()
			statuses <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)

	ok, conflict := 0, 0
	for code := range statuses {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Errorf("unexpected status: %d", code)
		}
	}
	if ok != 1 {
		t.Errorf("successful rollouts: got %d, want exactly 1", ok)
	}
	if conflict != attempts-1 {
		t.Errorf("conflicting rollouts: got %d, want %d", conflict, attempts-1)
	}

	final := getExperiment(t, exp.ID)
	if final.Status != "completed" {
		t.Fatalf("final status: got %q, want completed", final.Status)
	}
	if value := readFlagDefault(t, flagID); value != "on" {
		t.Errorf("flag default after concurrent rollout: got %s, want on", value)
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

func TestExperiment_PauseIsDifferentFromGuardrailRollback(t *testing.T) {
	flagID, _ := createFlag(t, "exp-pause-vs-rollback", "string", "fallback")
	exp := driveToRunning(t, adminToken, flagID, "Pause Versus Rollback")
	readFlagDefault := func(id string) string {
		resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/flags/%s", id), adminToken, nil)
		defer resp.Body.Close()
		requireStatus(t, resp, http.StatusOK)
		var result struct {
			Data struct {
				DefaultValue string `json:"default_value"`
			} `json:"data"`
		}
		decodeJSON(resp, &result)
		return result.Data.DefaultValue
	}

	exp = experimentTransition(t, adminToken, exp.ID, "pause", exp.Version)
	if exp.Status != "paused" || exp.GuardrailPaused {
		t.Fatalf("ordinary pause: status=%q guardrail_paused=%t, want paused/false", exp.Status, exp.GuardrailPaused)
	}
	if value := readFlagDefault(flagID); value != "fallback" {
		t.Fatalf("default after ordinary pause: got %s, want fallback", value)
	}

	exp = experimentTransition(t, adminToken, exp.ID, "resume", exp.Version)
	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/experiments/%s/complete", exp.ID), adminToken,
		jsonBody(map[string]any{"version": exp.Version, "decision": "rollback", "reason": "return to control"}))
	requireStatus(t, resp, http.StatusOK)
	exp = decodeExperimentResponse(t, resp).Data
	resp.Body.Close()
	if exp.Status != "completed" || exp.CompletionDecision == nil || *exp.CompletionDecision != "rollback" {
		t.Fatalf("rollback completion: status=%q decision=%v, want completed/rollback", exp.Status, exp.CompletionDecision)
	}
	if value := readFlagDefault(flagID); value != "off" {
		t.Fatalf("default after rollback completion: got %s, want control value off", value)
	}
	requireExperimentAuditRecord(t, exp.ID, "experiment.completed.rollback", "running", "completed", "return to control")

	guardrailFlagID, _ := createFlag(t, "exp-guardrail-rollback", "string", "fallback")
	guardrailExp := driveToRunning(t, adminToken, guardrailFlagID, "Guardrail Rollback")
	resp = doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/internal/experiments/%s/rollback", guardrailExp.ID), adminToken,
		jsonBody(map[string]any{"version": guardrailExp.Version}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	guardrailExp = decodeExperimentResponse(t, resp).Data
	if guardrailExp.Status != "paused" || !guardrailExp.GuardrailPaused {
		t.Fatalf("guardrail rollback: status=%q guardrail_paused=%t, want paused/true", guardrailExp.Status, guardrailExp.GuardrailPaused)
	}
	requireExperimentAuditRecord(t, guardrailExp.ID, "experiment.guardrail_rollback", "running", "paused", "")
	if value := readFlagDefault(guardrailFlagID); value != "off" {
		t.Fatalf("default after guardrail rollback: got %s, want control value off", value)
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
