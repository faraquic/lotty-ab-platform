package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func getExperiment(t *testing.T, id string) experimentResponseData {
	t.Helper()
	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/experiments/%s", id), adminToken, nil)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeExperimentResponse(t, resp).Data
}

func actOnReview(t *testing.T, token, reviewID, decision string, version int) reviewResponseData {
	t.Helper()
	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), token,
		jsonBody(map[string]any{"decision": decision, "version": version}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeReviewResponse(t, resp).Data
}

func approveViaReview(t *testing.T, token, expID string) experimentResponseData {
	t.Helper()
	exp := getExperiment(t, expID)
	if exp.CurrentVersion == nil || exp.CurrentVersion.ReviewID == nil {
		t.Fatal("experiment has no linked review")
	}
	actOnReview(t, token, *exp.CurrentVersion.ReviewID, "approve", exp.Version)
	return getExperiment(t, expID)
}

func createGroup(t *testing.T, name string, minApprovals int) groupResponseData {
	t.Helper()
	resp := doRequest(http.MethodPost, "/api/v1/panel/approver-groups", adminToken,
		jsonBody(map[string]any{"name": fmt.Sprintf("%s-%s", name, runID), "min_approvals": minApprovals}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeGroupResponse(t, resp).Data
}

func createApproverUser(t *testing.T, prefix string) (string, string) {
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

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(createResp, &result)

	token := login(email, "testpass123")
	if token == "" {
		t.Fatal("could not login as approver")
	}
	return result.Data.ID, token
}

func addGroupMember(t *testing.T, groupID, userID string) {
	t.Helper()
	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/approver-groups/%s/members", groupID), adminToken,
		jsonBody(map[string]any{"user_id": userID}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
}

func assignGroup(t *testing.T, experimenterID, groupID string) {
	t.Helper()
	resp := doRequest(http.MethodPut, fmt.Sprintf("/api/v1/panel/experimenters/%s/group", experimenterID), adminToken,
		jsonBody(map[string]any{"group_id": groupID}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
}

func submitExperiment(t *testing.T, token, flagID, name string) experimentResponseData {
	t.Helper()
	exp := createExperiment(t, token, flagID, name)
	exp = setExperimentVariants(t, token, exp.ID, exp.Version)
	return experimentTransition(t, token, exp.ID, "submit", exp.Version)
}

func createExperimenterID(t *testing.T, prefix string) string {
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

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(createResp, &result)
	return result.Data.ID
}

func loginAs(t *testing.T, prefix string) string {
	t.Helper()
	token := login(testEmail(prefix), "testpass123")
	if token == "" {
		t.Fatalf("could not login as %s", prefix)
	}
	return token
}

func TestReviewThreshold(t *testing.T) {
	ownerID := createExperimenterID(t, "rev-th-owner")
	ownerToken := loginAs(t, "rev-th-owner")
	approver1ID, approver1 := createApproverUser(t, "rev-th-a1")
	approver2ID, approver2 := createApproverUser(t, "rev-th-a2")
	group := createGroup(t, "threshold-group", 2)
	addGroupMember(t, group.ID, approver1ID)
	addGroupMember(t, group.ID, approver2ID)
	assignGroup(t, ownerID, group.ID)

	flagID, _ := createFlag(t, "rev-threshold", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Threshold Review")
	reviewID := *exp.CurrentVersion.ReviewID

	review := actOnReview(t, approver1, reviewID, "approve", exp.Version)
	if review.Status != "open" {
		t.Fatalf("status: got %q, want open after first approval", review.Status)
	}
	if len(review.Approvals) != 1 {
		t.Fatalf("approvals: got %d, want 1", len(review.Approvals))
	}

	review = actOnReview(t, approver2, reviewID, "approve", exp.Version)
	if review.Status != "approved" {
		t.Fatalf("status: got %q, want approved", review.Status)
	}

	exp = getExperiment(t, exp.ID)
	if exp.Status != "approved" {
		t.Fatalf("experiment status: got %q, want approved", exp.Status)
	}

	dup := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), approver1,
		jsonBody(map[string]any{"decision": "approve", "version": exp.Version}))
	defer dup.Body.Close()
	requireErrorResponse(t, dup, http.StatusConflict, "CONFLICT")
}

func TestReviewNonMemberForbidden(t *testing.T) {
	ownerID := createExperimenterID(t, "rev-nm-owner")
	ownerToken := loginAs(t, "rev-nm-owner")
	memberID, _ := createApproverUser(t, "rev-nm-member")
	_, outside := createApproverUser(t, "rev-nm-outside")
	group := createGroup(t, "closed-group", 1)
	addGroupMember(t, group.ID, memberID)
	assignGroup(t, ownerID, group.ID)

	flagID, _ := createFlag(t, "rev-nonmember", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Non Member")
	reviewID := *exp.CurrentVersion.ReviewID

	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), outside,
		jsonBody(map[string]any{"decision": "approve", "version": exp.Version}))
	defer resp.Body.Close()
	requireErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN")

	review := actOnReview(t, loginAs(t, "rev-nm-member"), reviewID, "approve", exp.Version)
	if review.Status != "approved" {
		t.Fatalf("status: got %q, want approved", review.Status)
	}
}

func TestReviewRejectFlow(t *testing.T) {
	createExperimenterID(t, "rev-rej-owner")
	ownerToken := loginAs(t, "rev-rej-owner")
	_, approver := createApproverUser(t, "rev-rej-approver")

	flagID, _ := createFlag(t, "rev-reject", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Reject Flow")
	reviewID := *exp.CurrentVersion.ReviewID

	review := actOnReview(t, approver, reviewID, "reject", exp.Version)
	if review.Status != "rejected" {
		t.Fatalf("status: got %q, want rejected", review.Status)
	}

	exp = getExperiment(t, exp.ID)
	if exp.Status != "rejected" {
		t.Fatalf("experiment status: got %q, want rejected", exp.Status)
	}

	late := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), approver,
		jsonBody(map[string]any{"decision": "approve", "version": exp.Version}))
	defer late.Body.Close()
	requireErrorResponse(t, late, http.StatusConflict, "CONFLICT")
}

func TestReviewRequestChangesFlow(t *testing.T) {
	createExperimenterID(t, "rev-rc-owner")
	ownerToken := loginAs(t, "rev-rc-owner")
	_, approver := createApproverUser(t, "rev-rc-approver")

	flagID, _ := createFlag(t, "rev-req-changes", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Request Changes")
	reviewID := *exp.CurrentVersion.ReviewID

	review := actOnReview(t, approver, reviewID, "request_changes", exp.Version)
	if review.Status != "changes_requested" {
		t.Fatalf("status: got %q, want changes_requested", review.Status)
	}

	exp = getExperiment(t, exp.ID)
	if exp.Status != "draft" {
		t.Fatalf("experiment status: got %q, want draft", exp.Status)
	}
}

func TestReviewComments(t *testing.T) {
	createExperimenterID(t, "rev-com-owner")
	ownerToken := loginAs(t, "rev-com-owner")
	_, approver := createApproverUser(t, "rev-com-approver")

	flagID, _ := createFlag(t, "rev-comments", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Comment Flow")
	reviewID := *exp.CurrentVersion.ReviewID

	comment := addComment(t, approver, reviewID, "Looks good overall", nil)
	reply := addComment(t, ownerToken, reviewID, "Thanks, addressed", &comment.ID)

	resolved := resolveComment(t, approver, reply.ID, true)
	if !resolved.Resolved {
		t.Error("reply should be resolved")
	}

	review := getReview(t, reviewID)
	if len(review.Comments) != 1 {
		t.Fatalf("top-level comments: got %d, want 1", len(review.Comments))
	}
	if len(review.Comments[0].Replies) != 1 || !review.Comments[0].Replies[0].Resolved {
		t.Errorf("unexpected replies: %+v", review.Comments[0].Replies)
	}

	deleteComment(t, ownerToken, reply.ID)

	review = getReview(t, reviewID)
	if len(review.Comments[0].Replies) != 0 {
		t.Errorf("reply should be gone: %+v", review.Comments[0].Replies)
	}
}

func TestReviewConcurrentApprovals(t *testing.T) {
	ownerID := createExperimenterID(t, "rev-cc-owner")
	ownerToken := loginAs(t, "rev-cc-owner")
	approver1ID, approver1 := createApproverUser(t, "rev-cc-a1")
	approver2ID, approver2 := createApproverUser(t, "rev-cc-a2")
	group := createGroup(t, "race-group", 2)
	addGroupMember(t, group.ID, approver1ID)
	addGroupMember(t, group.ID, approver2ID)
	assignGroup(t, ownerID, group.ID)

	flagID, _ := createFlag(t, "rev-race", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Race Approvals")
	reviewID := *exp.CurrentVersion.ReviewID

	type outcome struct {
		status int
	}
	results := make(chan outcome, 2)
	approve := func(token string) {
		resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/approvals", reviewID), token,
			jsonBody(map[string]any{"decision": "approve", "version": exp.Version}))
		defer resp.Body.Close()
		results <- outcome{status: resp.StatusCode}
	}
	go approve(approver1)
	go approve(approver2)

	statuses := map[int]int{}
	for i := 0; i < 2; i++ {
		statuses[(<-results).status]++
	}
	if statuses[http.StatusOK] != 2 {
		t.Fatalf("both approvals should succeed, got %v", statuses)
	}

	review := getReview(t, reviewID)
	if review.Status != "approved" {
		t.Errorf("status: got %q, want approved", review.Status)
	}
	if len(review.Approvals) != 2 {
		t.Errorf("approvals: got %d, want 2", len(review.Approvals))
	}
	exp = getExperiment(t, exp.ID)
	if exp.Status != "approved" {
		t.Errorf("experiment status: got %q, want approved", exp.Status)
	}
}

func TestApproverGroupCRUD(t *testing.T) {
	group := createGroup(t, "crud-group", 2)
	if len(group.Members) != 0 {
		t.Fatalf("members: got %d, want 0", len(group.Members))
	}

	dup := doRequest(http.MethodPost, "/api/v1/panel/approver-groups", adminToken,
		jsonBody(map[string]any{"name": group.Name, "min_approvals": 1}))
	defer dup.Body.Close()
	requireErrorResponse(t, dup, http.StatusConflict, "CONFLICT")

	memberID, _ := createApproverUser(t, "rev-crud-member")
	addGroupMember(t, group.ID, memberID)

	dupMember := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/approver-groups/%s/members", group.ID), adminToken,
		jsonBody(map[string]any{"user_id": memberID}))
	defer dupMember.Body.Close()
	requireErrorResponse(t, dupMember, http.StatusConflict, "CONFLICT")

	secondID, _ := createApproverUser(t, "rev-crud-member2")
	addGroupMember(t, group.ID, secondID)

	thirdID, _ := createApproverUser(t, "rev-crud-member3")
	addGroupMember(t, group.ID, thirdID)

	remove := doRequest(http.MethodDelete, fmt.Sprintf("/api/v1/panel/approver-groups/%s/members/%s", group.ID, memberID), adminToken, nil)
	defer remove.Body.Close()
	requireStatus(t, remove, http.StatusOK)

	overRemove := doRequest(http.MethodDelete, fmt.Sprintf("/api/v1/panel/approver-groups/%s/members/%s", group.ID, secondID), adminToken, nil)
	defer overRemove.Body.Close()
	requireErrorResponse(t, overRemove, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY")

	lower := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/approver-groups/%s", group.ID), adminToken,
		jsonBody(map[string]any{"min_approvals": 1}))
	defer lower.Body.Close()
	requireStatus(t, lower, http.StatusOK)

	archive := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/approver-groups/%s", group.ID), adminToken,
		jsonBody(map[string]any{"status": "archived"}))
	defer archive.Body.Close()
	requireStatus(t, archive, http.StatusOK)
}

func TestReviewQueue(t *testing.T) {
	createExperimenterID(t, "rev-q-owner")
	ownerToken := loginAs(t, "rev-q-owner")
	flagID, _ := createFlag(t, "rev-queue", "string", "off")
	exp := submitExperiment(t, ownerToken, flagID, "Queue Item")

	resp := doRequest(http.MethodGet, "/api/v1/panel/reviews?status=open&limit=100", adminToken, nil)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)

	var result struct {
		Success bool                 `json:"success"`
		Data    []reviewResponseData `json:"data"`
	}
	decodeJSON(resp, &result)

	found := false
	for _, r := range result.Data {
		if r.Status != "open" {
			t.Errorf("queue contains non-open review: %q", r.Status)
		}
		if r.ID == *exp.CurrentVersion.ReviewID {
			found = true
		}
	}
	if !found {
		t.Error("submitted review not found in open queue")
	}
}

func addComment(t *testing.T, token, reviewID, body string, parentID *string) reviewCommentData {
	t.Helper()
	payload := map[string]any{"body": body}
	if parentID != nil {
		payload["parent_id"] = *parentID
	}
	resp := doRequest(http.MethodPost, fmt.Sprintf("/api/v1/panel/reviews/%s/comments", reviewID), token, jsonBody(payload))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	var result struct {
		Data reviewCommentData `json:"data"`
	}
	decodeJSON(resp, &result)
	return result.Data
}

func resolveComment(t *testing.T, token, commentID string, resolved bool) reviewCommentData {
	t.Helper()
	resp := doRequest(http.MethodPatch, fmt.Sprintf("/api/v1/panel/reviews/comments/%s/resolve", commentID), token,
		jsonBody(map[string]any{"resolved": resolved}))
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	var result struct {
		Data reviewCommentData `json:"data"`
	}
	decodeJSON(resp, &result)
	return result.Data
}

func deleteComment(t *testing.T, token, commentID string) {
	t.Helper()
	resp := doRequest(http.MethodDelete, fmt.Sprintf("/api/v1/panel/reviews/comments/%s", commentID), token, nil)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusNoContent)
}

func getReview(t *testing.T, reviewID string) reviewResponseData {
	t.Helper()
	resp := doRequest(http.MethodGet, fmt.Sprintf("/api/v1/panel/reviews/%s", reviewID), adminToken, nil)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
	return decodeReviewResponse(t, resp).Data
}
