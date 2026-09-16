package reviews

import (
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

type CreateGroupRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=256"`
	Description  string `json:"description"`
	MinApprovals int    `json:"min_approvals" binding:"required,min=1"`
}

type UpdateGroupRequest struct {
	Name         string  `json:"name" binding:"omitempty,min=1,max=256"`
	Description  *string `json:"description"`
	MinApprovals *int    `json:"min_approvals" binding:"omitempty,min=1"`
	Status       string  `json:"status" binding:"omitempty,oneof=active archived"`
}

type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
}

type SetExperimenterGroupRequest struct {
	GroupID *string `json:"group_id"`
}

type ActOnReviewRequest struct {
	Decision ApprovalDecision `json:"decision" binding:"required,oneof=approve request_changes reject"`
	Version  int              `json:"version" binding:"required,min=1"`
	Comment  string           `json:"comment" binding:"omitempty,max=4096"`
}

type AddCommentRequest struct {
	Body     string  `json:"body" binding:"required,min=1,max=4096"`
	ParentID *string `json:"parent_id"`
}

type ResolveCommentRequest struct {
	Resolved bool `json:"resolved"`
}

type MemberResponse struct {
	ID        string  `json:"id"`
	FullName  string  `json:"full_name"`
	Email     string  `json:"email"`
	Role      string  `json:"role"`
	AvatarURL *string `json:"avatar_url"`
}

type GroupResponse struct {
	api.ResourceResponse
	Name         string           `json:"name"`
	Description  *string          `json:"description"`
	MinApprovals int              `json:"min_approvals"`
	Status       string           `json:"status"`
	Members      []MemberResponse `json:"members"`
}

type PaginatedGroupResponse struct {
	Data []GroupResponse    `json:"data"`
	Meta api.PaginationMeta `json:"meta"`
}

type ApprovalResponse struct {
	ID         string    `json:"id"`
	ReviewerID string    `json:"reviewer_id"`
	Decision   string    `json:"decision"`
	Comment    *string   `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

type CommentResponse struct {
	ID        string            `json:"id"`
	AuthorID  string            `json:"author_id"`
	ParentID  *string           `json:"parent_id"`
	Body      string            `json:"body"`
	Resolved  bool              `json:"resolved"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Replies   []CommentResponse `json:"replies"`
}

type ReviewResponse struct {
	api.ResourceResponse
	ExperimentID string             `json:"experiment_id"`
	VersionID    string             `json:"version_id"`
	VersionNum   int                `json:"version_num"`
	Status       ReviewStatus       `json:"status"`
	Approvals    []ApprovalResponse `json:"approvals"`
	Comments     []CommentResponse  `json:"comments"`
	CreatedBy    string             `json:"created_by"`
}

type PaginatedReviewResponse struct {
	Data []ReviewResponse   `json:"data"`
	Meta api.PaginationMeta `json:"meta"`
}

func memberToResponse(u users.User) MemberResponse {
	var avatar *string
	if u.AvatarURL != "" {
		avatar = &u.AvatarURL
	}
	return MemberResponse{
		ID:        u.ID,
		FullName:  u.FullName,
		Email:     u.Email,
		Role:      string(u.Role),
		AvatarURL: avatar,
	}
}

func ToGroupResponse(g ApproverGroup) GroupResponse {
	members := make([]MemberResponse, 0, len(g.Members))
	for _, m := range g.Members {
		members = append(members, memberToResponse(m))
	}
	return GroupResponse{
		ResourceResponse: api.ResourceResponse{
			ID:        g.ID,
			CreatedAt: g.CreatedAt,
			UpdatedAt: g.UpdatedAt,
		},
		Name:         g.Name,
		Description:  g.Description,
		MinApprovals: g.MinApprovals,
		Status:       string(g.Status),
		Members:      members,
	}
}

func ToReviewResponse(r Review) ReviewResponse {
	approvals := make([]ApprovalResponse, 0, len(r.Approvals))
	for _, a := range r.Approvals {
		approvals = append(approvals, ApprovalResponse{
			ID:         a.ID,
			ReviewerID: a.ReviewerID,
			Decision:   string(a.Decision),
			Comment:    a.Comment,
			CreatedAt:  a.CreatedAt,
		})
	}
	return ReviewResponse{
		ResourceResponse: api.ResourceResponse{
			ID:        r.ID,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		ExperimentID: r.ExperimentID,
		VersionID:    r.VersionID,
		VersionNum:   r.VersionNum,
		Status:       r.Status,
		Approvals:    approvals,
		Comments:     buildCommentTree(r.Comments),
		CreatedBy:    r.CreatedBy,
	}
}

func toCommentResponse(c Comment) CommentResponse {
	return CommentResponse{
		ID:        c.ID,
		AuthorID:  c.AuthorID,
		ParentID:  c.ParentID,
		Body:      c.Body,
		Resolved:  c.Resolved,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Replies:   []CommentResponse{},
	}
}

func buildCommentTree(comments []Comment) []CommentResponse {
	byParent := make(map[string][]Comment)
	var roots []Comment
	for _, c := range comments {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else {
			byParent[*c.ParentID] = append(byParent[*c.ParentID], c)
		}
	}
	var convert func(c Comment) CommentResponse
	convert = func(c Comment) CommentResponse {
		replies := make([]CommentResponse, 0, len(byParent[c.ID]))
		for _, r := range byParent[c.ID] {
			replies = append(replies, CommentResponse{
				ID:        r.ID,
				AuthorID:  r.AuthorID,
				ParentID:  r.ParentID,
				Body:      r.Body,
				Resolved:  r.Resolved,
				CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			})
		}
		return CommentResponse{
			ID:        c.ID,
			AuthorID:  c.AuthorID,
			ParentID:  c.ParentID,
			Body:      c.Body,
			Resolved:  c.Resolved,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Replies:   replies,
		}
	}
	out := make([]CommentResponse, 0, len(roots))
	for _, root := range roots {
		out = append(out, convert(root))
	}
	return out
}
