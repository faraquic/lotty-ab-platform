package reviews

import (
	"context"
	"strings"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"go.uber.org/zap"
)

type UserProvider interface {
	GetByID(ctx context.Context, id string) (users.User, error)
}

type OwnerResolver func(ctx context.Context, experimentID string) (string, error)

type OutcomeApplier func(ctx context.Context, callerID, experimentID, toStatus string, version int) error

type Service struct {
	repo         *Repository
	users        UserProvider
	ownerOf      OwnerResolver
	applyOutcome OutcomeApplier
	log          *zap.Logger
}

func NewService(repo *Repository, users UserProvider, ownerOf OwnerResolver, applyOutcome OutcomeApplier, log *zap.Logger) *Service {
	return &Service{repo, users, ownerOf, applyOutcome, log}
}

func (s *Service) CreateGroup(ctx context.Context, callerID string, req CreateGroupRequest) (GroupResponse, error) {
	if err := ValidateGroup(req.Name, req.MinApprovals, -1); err != nil {
		return GroupResponse{}, err
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	id, err := s.repo.CreateGroup(ctx, strings.TrimSpace(req.Name), desc, req.MinApprovals, callerID)
	if err != nil {
		return GroupResponse{}, err
	}

	s.log.Info("approver group created",
		zap.String(logger.FieldGroupID, id),
		zap.String(logger.FieldActorID, callerID),
	)

	return s.GetGroup(ctx, id, true)
}

func (s *Service) GetGroup(ctx context.Context, id string, includeArchived bool) (GroupResponse, error) {
	g, err := s.repo.GetGroup(ctx, id, includeArchived)
	if err != nil {
		return GroupResponse{}, err
	}
	return ToGroupResponse(g), nil
}

func (s *Service) ListGroups(ctx context.Context, limit, offset int, includeArchived bool) (PaginatedGroupResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountGroups(ctx, includeArchived)
	if err != nil {
		return PaginatedGroupResponse{}, err
	}

	list, err := s.repo.ListGroups(ctx, limit, offset, includeArchived)
	if err != nil {
		return PaginatedGroupResponse{}, err
	}

	resp := make([]GroupResponse, 0, len(list))
	for _, g := range list {
		resp = append(resp, ToGroupResponse(g))
	}

	count := len(resp)
	return PaginatedGroupResponse{
		Data: resp,
		Meta: api.PaginationMeta{
			Limit:   limit,
			Offset:  offset,
			Count:   count,
			Total:   total,
			HasNext: int64(offset)+int64(count) < total,
		},
	}, nil
}

func (s *Service) UpdateGroup(ctx context.Context, callerID, id string, req UpdateGroupRequest) (GroupResponse, error) {
	var description *string
	if req.Description != nil && *req.Description != "" {
		description = req.Description
	}

	var status GroupStatus
	if req.Status != "" {
		status = GroupStatus(req.Status)
		if !status.Valid() {
			return GroupResponse{}, ErrInvalidThreshold
		}
	}

	g, err := s.repo.UpdateGroup(ctx, id, strings.TrimSpace(req.Name), description, req.MinApprovals, status, callerID)
	if err != nil {
		return GroupResponse{}, err
	}
	return ToGroupResponse(g), nil
}

func (s *Service) AddMember(ctx context.Context, groupID, userID string) (GroupResponse, error) {
	if _, err := s.repo.GetGroup(ctx, groupID, true); err != nil {
		return GroupResponse{}, err
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return GroupResponse{}, ErrUserNotFound
	}
	if err := s.repo.AddMember(ctx, groupID, userID); err != nil {
		return GroupResponse{}, err
	}
	return s.GetGroup(ctx, groupID, true)
}

func (s *Service) RemoveMember(ctx context.Context, groupID, userID string) (GroupResponse, error) {
	if err := s.repo.RemoveMember(ctx, groupID, userID); err != nil {
		return GroupResponse{}, err
	}
	return s.GetGroup(ctx, groupID, true)
}

func (s *Service) SetExperimenterGroup(ctx context.Context, experimenterID string, req SetExperimenterGroupRequest) error {
	if _, err := s.users.GetByID(ctx, experimenterID); err != nil {
		return ErrUserNotFound
	}
	if req.GroupID == nil {
		return s.repo.UnsetExperimenterGroup(ctx, experimenterID)
	}
	if _, err := s.repo.GetGroup(ctx, *req.GroupID, false); err != nil {
		return err
	}
	return s.repo.SetExperimenterGroup(ctx, experimenterID, *req.GroupID)
}

func (s *Service) CreateReview(ctx context.Context, experimentID, versionID, callerID string) (string, error) {
	return s.repo.CreateReview(ctx, experimentID, versionID, callerID)
}

func (s *Service) VersionReviewStatus(ctx context.Context, versionID string) (ReviewStatus, error) {
	rev, err := s.repo.GetVersionReview(ctx, versionID)
	if err != nil {
		return "", err
	}
	return rev.Status, nil
}

func (s *Service) GetReview(ctx context.Context, id string) (ReviewResponse, error) {
	rev, err := s.repo.GetReview(ctx, id)
	if err != nil {
		return ReviewResponse{}, err
	}
	return ToReviewResponse(rev), nil
}

func (s *Service) ListReviews(ctx context.Context, limit, offset int, status ReviewStatus) (PaginatedReviewResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if status != "" && !status.Valid() {
		return PaginatedReviewResponse{}, ErrReviewClosed
	}

	total, err := s.repo.CountReviews(ctx, status)
	if err != nil {
		return PaginatedReviewResponse{}, err
	}

	list, err := s.repo.ListReviews(ctx, limit, offset, status)
	if err != nil {
		return PaginatedReviewResponse{}, err
	}

	resp := make([]ReviewResponse, 0, len(list))
	for _, r := range list {
		resp = append(resp, ToReviewResponse(r))
	}

	count := len(resp)
	return PaginatedReviewResponse{
		Data: resp,
		Meta: api.PaginationMeta{
			Limit:   limit,
			Offset:  offset,
			Count:   count,
			Total:   total,
			HasNext: int64(offset)+int64(count) < total,
		},
	}, nil
}

func (s *Service) Act(ctx context.Context, callerID, reviewID string, req ActOnReviewRequest) (ReviewResponse, error) {
	if !req.Decision.Valid() {
		return ReviewResponse{}, ErrInvalidDecision
	}

	rev, err := s.repo.GetReview(ctx, reviewID)
	if err != nil {
		return ReviewResponse{}, err
	}
	if rev.Status != ReviewOpen {
		return ReviewResponse{}, ErrReviewClosed
	}

	threshold, err := s.checkEligible(ctx, callerID, rev.ExperimentID)
	if err != nil {
		return ReviewResponse{}, err
	}

	var comment *string
	if strings.TrimSpace(req.Comment) != "" {
		c := strings.TrimSpace(req.Comment)
		comment = &c
	}

	final, err := s.repo.ActOnReview(ctx, reviewID, callerID, req.Decision, comment, threshold)
	if err != nil {
		return ReviewResponse{}, err
	}

	if final != ReviewOpen {
		toStatus := map[ReviewStatus]string{
			ReviewApproved:         "approved",
			ReviewChangesRequested: "draft",
			ReviewRejected:         "rejected",
		}[final]
		if err := s.applyOutcome(ctx, callerID, rev.ExperimentID, toStatus, req.Version); err != nil {
			return ReviewResponse{}, err
		}

		s.log.Info("review finalized",
			zap.String(logger.FieldReviewID, reviewID),
			zap.String(logger.FieldReviewStatus, string(final)),
			zap.String(logger.FieldActorID, callerID),
		)
	}

	return s.GetReview(ctx, reviewID)
}

func (s *Service) checkEligible(ctx context.Context, callerID, experimentID string) (int, error) {
	u, err := s.users.GetByID(ctx, callerID)
	if err != nil {
		return 0, err
	}
	if u.Role == users.RoleAdmin {
		return 1, nil
	}

	ownerID, err := s.ownerOf(ctx, experimentID)
	if err != nil {
		return 0, err
	}

	rule, err := s.repo.GetRule(ctx, ownerID)
	if err != nil {
		return 0, err
	}
	if rule.GroupID == nil {
		if u.Role != users.RoleApprover {
			return 0, ErrForbidden
		}
		return 1, nil
	}

	for _, memberID := range rule.MemberIDs {
		if memberID == callerID {
			return rule.MinApprovals, nil
		}
	}
	return 0, ErrForbidden
}

func (s *Service) AddComment(ctx context.Context, callerID, reviewID string, req AddCommentRequest) (CommentResponse, error) {
	if err := ValidateComment(req.Body); err != nil {
		return CommentResponse{}, err
	}
	if _, err := s.repo.GetReview(ctx, reviewID); err != nil {
		return CommentResponse{}, err
	}

	id, err := s.repo.AddComment(ctx, reviewID, callerID, req.ParentID, strings.TrimSpace(req.Body))
	if err != nil {
		return CommentResponse{}, err
	}

	c, err := s.repo.GetComment(ctx, id)
	if err != nil {
		return CommentResponse{}, err
	}
	return toCommentResponse(c), nil
}

func (s *Service) ResolveComment(ctx context.Context, callerID, reviewID, commentID string, req ResolveCommentRequest) (CommentResponse, error) {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return CommentResponse{}, err
	}
	if c.AuthorID != callerID {
		u, err := s.users.GetByID(ctx, callerID)
		if err != nil {
			return CommentResponse{}, err
		}
		if u.Role != users.RoleAdmin && u.Role != users.RoleApprover {
			return CommentResponse{}, ErrForbidden
		}
	}
	if err := s.repo.ResolveComment(ctx, reviewID, commentID, req.Resolved); err != nil {
		return CommentResponse{}, err
	}
	c, err = s.repo.GetComment(ctx, commentID)
	if err != nil {
		return CommentResponse{}, err
	}
	return toCommentResponse(c), nil
}

func (s *Service) DeleteComment(ctx context.Context, callerID, reviewID, commentID string) error {
	if err := s.checkCommentOwner(ctx, callerID, commentID); err != nil {
		return err
	}
	return s.repo.DeleteComment(ctx, reviewID, commentID)
}

func (s *Service) ResolveCommentByID(ctx context.Context, callerID, commentID string, req ResolveCommentRequest) (CommentResponse, error) {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return CommentResponse{}, err
	}
	return s.ResolveComment(ctx, callerID, c.ReviewID, commentID, req)
}

func (s *Service) DeleteCommentByID(ctx context.Context, callerID, commentID string) error {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return err
	}
	return s.DeleteComment(ctx, callerID, c.ReviewID, commentID)
}

func (s *Service) checkCommentOwner(ctx context.Context, callerID, commentID string) error {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return err
	}
	if c.AuthorID == callerID {
		return nil
	}
	u, err := s.users.GetByID(ctx, callerID)
	if err != nil {
		return err
	}
	if u.Role != users.RoleAdmin {
		return ErrForbidden
	}
	return nil
}
