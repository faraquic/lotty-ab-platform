package reviews

import (
	"testing"
)

func TestReviewStatusFlow(t *testing.T) {
	for _, s := range []ReviewStatus{ReviewOpen, ReviewApproved, ReviewChangesRequested, ReviewRejected} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if ReviewStatus("draft").Valid() {
		t.Error("draft should not be a review status")
	}
	if ReviewOpen.Final() {
		t.Error("open should not be final")
	}
	for _, s := range []ReviewStatus{ReviewApproved, ReviewChangesRequested, ReviewRejected} {
		if !s.Final() {
			t.Errorf("%q should be final", s)
		}
	}
}

func TestApprovalDecisionValid(t *testing.T) {
	for _, d := range []ApprovalDecision{DecisionApprove, DecisionRequestChanges, DecisionReject} {
		if !d.Valid() {
			t.Errorf("%q should be valid", d)
		}
	}
	if ApprovalDecision("comment").Valid() {
		t.Error("comment should not be a decision")
	}
}

func TestValidateGroup(t *testing.T) {
	if err := ValidateGroup("core", 2, 3); err != nil {
		t.Errorf("valid group rejected: %v", err)
	}
	if err := ValidateGroup("core", 2, -1); err != nil {
		t.Errorf("group without member context rejected: %v", err)
	}
	for name, args := range map[string]struct {
		name    string
		min     int
		members int
	}{
		"empty name":     {"", 1, 1},
		"zero threshold": {"g", 0, 1},
		"over members":   {"g", 3, 2},
	} {
		if err := ValidateGroup(args.name, args.min, args.members); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestValidateComment(t *testing.T) {
	if err := ValidateComment("looks good"); err != nil {
		t.Errorf("valid comment rejected: %v", err)
	}
	for _, bad := range []string{"", "   "} {
		if err := ValidateComment(bad); err == nil {
			t.Errorf("%q: expected error, got nil", bad)
		}
	}
}
