package reviews

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

var (
	ErrNotFound         = errors.New("review not found")
	ErrGroupNotFound    = errors.New("approver group not found")
	ErrCommentNotFound  = errors.New("review comment not found")
	ErrConflictName     = errors.New("approver group with this name already exists")
	ErrDuplicateMember  = errors.New("user is already a group member")
	ErrMemberNotFound   = errors.New("user is not a group member")
	ErrDuplicateAction  = errors.New("reviewer already acted on this review")
	ErrReviewClosed     = errors.New("review is closed")
	ErrForbidden        = errors.New("not allowed to review")
	ErrReviewExists     = errors.New("version already has a review")
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidThreshold = errors.New("invalid approvals threshold")
	ErrInvalidComment   = errors.New("invalid comment")
	ErrInvalidDecision  = errors.New("invalid decision")
)

type ReviewStatus string

const (
	ReviewOpen             ReviewStatus = "open"
	ReviewApproved         ReviewStatus = "approved"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewRejected         ReviewStatus = "rejected"
)

func (s ReviewStatus) Valid() bool {
	switch s {
	case ReviewOpen, ReviewApproved, ReviewChangesRequested, ReviewRejected:
		return true
	default:
		return false
	}
}

func (s ReviewStatus) Final() bool {
	return s != ReviewOpen
}

type ApprovalDecision string

const (
	DecisionApprove        ApprovalDecision = "approve"
	DecisionRequestChanges ApprovalDecision = "request_changes"
	DecisionReject         ApprovalDecision = "reject"
)

func (d ApprovalDecision) Valid() bool {
	switch d {
	case DecisionApprove, DecisionRequestChanges, DecisionReject:
		return true
	default:
		return false
	}
}

type GroupStatus string

const (
	GroupActive   GroupStatus = "active"
	GroupArchived GroupStatus = "archived"
)

func (s GroupStatus) Valid() bool {
	switch s {
	case GroupActive, GroupArchived:
		return true
	default:
		return false
	}
}

type ApproverGroup struct {
	ID           string
	Name         string
	Description  *string
	MinApprovals int
	Status       GroupStatus
	Members      []users.User
	CreatedBy    string
	UpdatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func ValidateGroup(name string, minApprovals, activeMembers int) error {
	if strings.TrimSpace(name) == "" || len(name) > 256 {
		return fmt.Errorf("%w: name must be 1..256 chars", ErrInvalidThreshold)
	}
	if minApprovals < 1 {
		return fmt.Errorf("%w: min_approvals must be >= 1", ErrInvalidThreshold)
	}
	if activeMembers >= 0 && minApprovals > activeMembers {
		return fmt.Errorf("%w: min_approvals exceeds member count", ErrInvalidThreshold)
	}
	return nil
}

type Review struct {
	ID           string
	ExperimentID string
	VersionID    string
	VersionNum   int
	Status       ReviewStatus
	Approvals    []Approval
	Comments     []Comment
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Approval struct {
	ID         string
	ReviewID   string
	ReviewerID string
	Decision   ApprovalDecision
	Comment    *string
	CreatedAt  time.Time
}

type Comment struct {
	ID        string
	ReviewID  string
	AuthorID  string
	ParentID  *string
	Body      string
	Resolved  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ValidateComment(body string) error {
	if strings.TrimSpace(body) == "" || len(body) > 4096 {
		return fmt.Errorf("%w: body must be 1..4096 chars", ErrInvalidComment)
	}
	return nil
}
