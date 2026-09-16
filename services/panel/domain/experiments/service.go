package experiments

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	flagsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/flags"
	reviewsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/reviews"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	panelsnapshot "github.com/faraquic/lotty-ab-platform/services/panel/snapshot"
	"go.uber.org/zap"
)

const defaultWeightsTotal = 10000

type ExperimentRepo interface {
	CreateExperiment(ctx context.Context, exp Experiment, weightsTotal int, targeting *Targeting, salt string) (string, string, error)
	GetState(ctx context.Context, id string) (Experiment, error)
	GetDetail(ctx context.Context, id string) (ExperimentDetail, error)
	CreateVersion(ctx context.Context, experimentID string, weightsTotal int, targeting *Targeting, salt, callerID string, resetToDraft bool) (string, int, error)
	SetVariants(ctx context.Context, experimentID, versionID string, inputs []VariantInput, expectedVersion int, callerID string) error
	ListVariants(ctx context.Context, versionID string) ([]Variant, error)
	Transition(ctx context.Context, id string, from, to Status, expectedVersion int, callerID string, guardrailPaused *bool) (Experiment, error)
	UpdateDraft(ctx context.Context, id, name, description string, expectedVersion int, callerID string) (Experiment, error)
	Complete(ctx context.Context, id string, from Status, expectedVersion int, callerID string, decision CompletionDecision, reason string) (Experiment, error)
	CountActiveByFlag(ctx context.Context, flagID, excludeID string) (int64, error)
	List(ctx context.Context, limit, offset int, status Status) ([]Experiment, error)
	Count(ctx context.Context, status Status) (int64, error)
}

type UserProvider interface {
	GetByID(ctx context.Context, id string) (users.User, error)
}

type FlagsService interface {
	Update(ctx context.Context, callerID, id string, req flagsdomain.UpdateFlagRequest) (flagsdomain.FlagResponse, error)
}

type ReviewService interface {
	CreateReview(ctx context.Context, experimentID, versionID, callerID string) (string, error)
	VersionReviewStatus(ctx context.Context, versionID string) (reviewsdomain.ReviewStatus, error)
}

type Service struct {
	repo      ExperimentRepo
	users     UserProvider
	flags     FlagsService
	reviews   ReviewService
	refresher panelsnapshot.Refresher
	log       *zap.Logger
}

func NewService(repo ExperimentRepo, users UserProvider, flags FlagsService, reviews ReviewService, refresher panelsnapshot.Refresher, log *zap.Logger) *Service {
	return &Service{repo, users, flags, reviews, refresher, log}
}

func (s *Service) notifyRefresh() {
	s.refresher.Refresh()
}

func resolveWeightsTotal(wt int) (int, error) {
	if wt == 0 {
		return defaultWeightsTotal, nil
	}
	if wt < 1 || wt > defaultWeightsTotal {
		return 0, ErrInvalidWeights
	}
	return wt, nil
}

func checkTargeting(t *Targeting) error {
	if t == nil {
		return nil
	}
	if !t.Valid() {
		return ErrInvalidTargeting
	}
	return nil
}

func normalizeTargeting(t *Targeting) *Targeting {
	if t == nil {
		return nil
	}
	trimmed := bytes.TrimSpace([]byte(*t))
	if len(trimmed) == 0 || string(trimmed) == "null" || string(trimmed) == "{}" {
		return nil
	}
	return t
}

func (s *Service) isAdmin(ctx context.Context, callerID string) (bool, error) {
	u, err := s.users.GetByID(ctx, callerID)
	if err != nil {
		return false, err
	}
	return u.Role == users.RoleAdmin, nil
}

func checkOwner(state Experiment, callerID string, admin bool) error {
	if admin {
		return nil
	}
	if state.OwnerID != callerID {
		return ErrForbidden
	}
	return nil
}

func (s *Service) Create(ctx context.Context, callerID string, req CreateExperimentRequest) (ExperimentResponse, error) {
	weights, err := resolveWeightsTotal(req.WeightsTotal)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkTargeting(req.Targeting); err != nil {
		return ExperimentResponse{}, err
	}
	if len(req.Variants) > 0 {
		if err := ValidateVariants(req.Variants, weights); err != nil {
			return ExperimentResponse{}, err
		}
	}

	salt, err := NewSalt()
	if err != nil {
		return ExperimentResponse{}, err
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	expID, versionID, err := s.repo.CreateExperiment(ctx, Experiment{
		FlagID:      req.FlagID,
		Name:        req.Name,
		Description: desc,
		OwnerID:     callerID,
		CreatedBy:   callerID,
		UpdatedBy:   callerID,
	}, weights, normalizeTargeting(req.Targeting), salt)
	if err != nil {
		return ExperimentResponse{}, err
	}

	if len(req.Variants) > 0 {
		if err := s.repo.SetVariants(ctx, expID, versionID, req.Variants, 1, callerID); err != nil {
			return ExperimentResponse{}, err
		}
	}

	s.log.Info("experiment created",
		zap.String(logger.FieldExperimentID, expID),
		zap.String(logger.FieldExperimentName, req.Name),
		zap.String(logger.FieldActorID, callerID),
	)

	return s.getDetail(ctx, expID)
}

func (s *Service) getDetail(ctx context.Context, id string) (ExperimentResponse, error) {
	d, err := s.repo.GetDetail(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	return ToResponse(d), nil
}

func (s *Service) GetByID(ctx context.Context, id string) (ExperimentResponse, error) {
	return s.getDetail(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int, status Status) (PaginatedExperimentResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if status != "" && !status.Valid() {
		return PaginatedExperimentResponse{}, ErrInvalidTransition
	}

	total, err := s.repo.Count(ctx, status)
	if err != nil {
		return PaginatedExperimentResponse{}, err
	}

	list, err := s.repo.List(ctx, limit, offset, status)
	if err != nil {
		return PaginatedExperimentResponse{}, err
	}

	resp := make([]ExperimentResponse, 0, len(list))
	for _, m := range list {
		resp = append(resp, ToResponse(ExperimentDetail{Experiment: m}))
	}

	count := len(resp)
	return PaginatedExperimentResponse{
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

func (s *Service) UpdateDraft(ctx context.Context, callerID, id string, req UpdateDraftRequest) (ExperimentResponse, error) {
	admin, err := s.isAdmin(ctx, callerID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkOwner(state, callerID, admin); err != nil {
		return ExperimentResponse{}, err
	}

	var desc string
	if req.Description != nil {
		desc = *req.Description
	}
	if _, err := s.repo.UpdateDraft(ctx, id, req.Name, desc, req.Version, callerID); err != nil {
		return ExperimentResponse{}, err
	}
	return s.getDetail(ctx, id)
}

func (s *Service) CreateVersion(ctx context.Context, callerID, id string, req CreateVersionRequest) (ExperimentResponse, error) {
	admin, err := s.isAdmin(ctx, callerID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkOwner(state, callerID, admin); err != nil {
		return ExperimentResponse{}, err
	}
	switch state.Status {
	case StatusDraft, StatusReview, StatusApproved, StatusPaused, StatusRejected:
	default:
		return ExperimentResponse{}, ErrVersionImmutable
	}

	weights, err := resolveWeightsTotal(req.WeightsTotal)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkTargeting(req.Targeting); err != nil {
		return ExperimentResponse{}, err
	}
	if len(req.Variants) > 0 {
		if err := ValidateVariants(req.Variants, weights); err != nil {
			return ExperimentResponse{}, err
		}
	}

	if state.Version != req.Version {
		return ExperimentResponse{}, ErrVersionConflict
	}

	salt, err := NewSalt()
	if err != nil {
		return ExperimentResponse{}, err
	}

	resetToDraft := state.Status == StatusApproved || state.Status == StatusRejected
	versionID, versionNum, err := s.repo.CreateVersion(ctx, id, weights, normalizeTargeting(req.Targeting), salt, callerID, resetToDraft)
	if err != nil {
		return ExperimentResponse{}, err
	}

	if state.Status == StatusReview {
		if _, err := s.reviews.CreateReview(ctx, id, versionID, callerID); err != nil {
			return ExperimentResponse{}, err
		}
	}

	if len(req.Variants) > 0 {
		if err := s.repo.SetVariants(ctx, id, versionID, req.Variants, state.Version+1, callerID); err != nil {
			return ExperimentResponse{}, err
		}
	}

	s.log.Info("experiment version created",
		zap.String(logger.FieldExperimentID, id),
		zap.Int(logger.FieldExperimentVersion, versionNum),
		zap.String(logger.FieldActorID, callerID),
	)

	return s.getDetail(ctx, id)
}

func (s *Service) SetVariants(ctx context.Context, callerID, id string, req SetVariantsRequest) (ExperimentResponse, error) {
	admin, err := s.isAdmin(ctx, callerID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkOwner(state, callerID, admin); err != nil {
		return ExperimentResponse{}, err
	}
	if state.Status != StatusDraft {
		return ExperimentResponse{}, ErrVersionImmutable
	}
	if state.CurrentVersionID == nil {
		return ExperimentResponse{}, ErrVersionNotFound
	}
	if state.Version != req.Version {
		return ExperimentResponse{}, ErrVersionConflict
	}

	ver, err := s.currentVersion(ctx, state)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := ValidateVariants(req.Variants, ver.WeightsTotal); err != nil {
		return ExperimentResponse{}, err
	}

	if err := s.repo.SetVariants(ctx, id, ver.ID, req.Variants, req.Version, callerID); err != nil {
		return ExperimentResponse{}, err
	}
	return s.getDetail(ctx, id)
}

func (s *Service) currentVersion(ctx context.Context, state Experiment) (ExperimentVersion, error) {
	if state.CurrentVersionID == nil {
		return ExperimentVersion{}, ErrVersionNotFound
	}
	d, err := s.repo.GetDetail(ctx, state.ID)
	if err != nil {
		return ExperimentVersion{}, err
	}
	if d.CurrentVersion == nil {
		return ExperimentVersion{}, ErrVersionNotFound
	}
	return *d.CurrentVersion, nil
}

func (s *Service) transition(ctx context.Context, callerID, id string, to Status, version int, allowPausedOverride bool) (ExperimentResponse, error) {
	admin, err := s.isAdmin(ctx, callerID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkOwner(state, callerID, admin); err != nil {
		return ExperimentResponse{}, err
	}
	if err := ValidateTransition(state.Status, to); err != nil {
		return ExperimentResponse{}, err
	}

	var gp *bool
	switch {
	case state.Status == StatusPaused && to == StatusRunning:
		if state.GuardrailPaused && !admin && !allowPausedOverride {
			return ExperimentResponse{}, ErrForbidden
		}
		if n, err := s.repo.CountActiveByFlag(ctx, state.FlagID, id); err != nil {
			return ExperimentResponse{}, err
		} else if n > 0 {
			return ExperimentResponse{}, ErrFlagBusy
		}
		if state.GuardrailPaused {
			cleared := false
			gp = &cleared
		}
	case state.Status == StatusApproved && to == StatusRunning:
		if n, err := s.repo.CountActiveByFlag(ctx, state.FlagID, id); err != nil {
			return ExperimentResponse{}, err
		} else if n > 0 {
			return ExperimentResponse{}, ErrFlagBusy
		}
	}

	if _, err := s.repo.Transition(ctx, id, state.Status, to, version, callerID, gp); err != nil {
		return ExperimentResponse{}, err
	}

	s.log.Info("experiment transition",
		zap.String(logger.FieldExperimentID, id),
		zap.String(logger.FieldExperimentStatus, string(to)),
		zap.String(logger.FieldActorID, callerID),
	)

	return s.getDetail(ctx, id)
}

func (s *Service) Submit(ctx context.Context, callerID, id string, req TransitionRequest) (ExperimentResponse, error) {
	admin, err := s.isAdmin(ctx, callerID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkOwner(state, callerID, admin); err != nil {
		return ExperimentResponse{}, err
	}
	if err := s.checkReady(ctx, state); err != nil {
		return ExperimentResponse{}, err
	}
	ver, err := s.currentVersion(ctx, state)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if _, err := s.reviews.CreateReview(ctx, id, ver.ID, callerID); err != nil {
		if !errors.Is(err, reviewsdomain.ErrReviewExists) {
			return ExperimentResponse{}, err
		}
	}
	return s.transition(ctx, callerID, id, StatusReview, req.Version, false)
}

func (s *Service) checkReady(ctx context.Context, state Experiment) error {
	ver, err := s.currentVersion(ctx, state)
	if err != nil {
		return err
	}
	variants, err := s.repo.ListVariants(ctx, ver.ID)
	if err != nil {
		return err
	}
	inputs := make([]VariantInput, 0, len(variants))
	for _, v := range variants {
		inputs = append(inputs, VariantInput{Name: v.Name, Value: v.Value, WeightBP: v.WeightBP, IsControl: v.IsControl})
	}
	if err := ValidateVariants(inputs, ver.WeightsTotal); err != nil {
		return ErrVersionNotReady
	}
	return nil
}

func (s *Service) ApplyReviewOutcome(ctx context.Context, callerID, experimentID string, to Status, version int) (ExperimentResponse, error) {
	state, err := s.repo.GetState(ctx, experimentID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := ValidateTransition(state.Status, to); err != nil {
		return ExperimentResponse{}, err
	}
	if _, err := s.repo.Transition(ctx, experimentID, state.Status, to, version, callerID, nil); err != nil {
		return ExperimentResponse{}, err
	}

	s.log.Info("experiment review outcome",
		zap.String(logger.FieldExperimentID, experimentID),
		zap.String(logger.FieldExperimentStatus, string(to)),
		zap.String(logger.FieldActorID, callerID),
	)

	return s.getDetail(ctx, experimentID)
}

func (s *Service) Start(ctx context.Context, callerID, id string, req TransitionRequest) (ExperimentResponse, error) {
	resp, err := s.transition(ctx, callerID, id, StatusRunning, req.Version, false)
	if err != nil {
		return ExperimentResponse{}, err
	}
	s.notifyRefresh()
	return resp, nil
}

func (s *Service) Pause(ctx context.Context, callerID, id string, req TransitionRequest) (ExperimentResponse, error) {
	resp, err := s.transition(ctx, callerID, id, StatusPaused, req.Version, false)
	if err != nil {
		return ExperimentResponse{}, err
	}
	s.notifyRefresh()
	return resp, nil
}

func (s *Service) Resume(ctx context.Context, callerID, id string, req TransitionRequest) (ExperimentResponse, error) {
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if state.Status == StatusPaused {
		if state.CurrentVersionID == nil {
			return ExperimentResponse{}, ErrVersionNotFound
		}
		approved, err := s.reviews.VersionReviewStatus(ctx, *state.CurrentVersionID)
		if err != nil && !errors.Is(err, reviewsdomain.ErrNotFound) {
			return ExperimentResponse{}, err
		}
		if err == nil && approved != reviewsdomain.ReviewApproved {
			return ExperimentResponse{}, ErrVersionNotApproved
		}
	}
	resp, err := s.transition(ctx, callerID, id, StatusRunning, req.Version, false)
	if err != nil {
		return ExperimentResponse{}, err
	}
	s.notifyRefresh()
	return resp, nil
}

func (s *Service) Archive(ctx context.Context, callerID, id string, req TransitionRequest) (ExperimentResponse, error) {
	return s.transition(ctx, callerID, id, StatusArchived, req.Version, false)
}

func (s *Service) InternalPause(ctx context.Context, callerID, id string, version int) (ExperimentResponse, error) {
	return s.guardrailTransition(ctx, callerID, id, version)
}

func (s *Service) InternalRollback(ctx context.Context, callerID, id string, version int) (ExperimentResponse, error) {
	return s.guardrailTransition(ctx, callerID, id, version)
}

func (s *Service) guardrailTransition(ctx context.Context, callerID, id string, version int) (ExperimentResponse, error) {
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := ValidateTransition(state.Status, StatusPaused); err != nil {
		return ExperimentResponse{}, err
	}
	set := true
	if _, err := s.repo.Transition(ctx, id, state.Status, StatusPaused, version, callerID, &set); err != nil {
		return ExperimentResponse{}, err
	}

	s.log.Info("experiment guardrail pause",
		zap.String(logger.FieldExperimentID, id),
		zap.String(logger.FieldActorID, callerID),
	)

	s.notifyRefresh()

	return s.getDetail(ctx, id)
}

func (s *Service) Complete(ctx context.Context, callerID, id string, req CompleteRequest) (ExperimentResponse, error) {
	admin, err := s.isAdmin(ctx, callerID)
	if err != nil {
		return ExperimentResponse{}, err
	}
	state, err := s.repo.GetState(ctx, id)
	if err != nil {
		return ExperimentResponse{}, err
	}
	if err := checkOwner(state, callerID, admin); err != nil {
		return ExperimentResponse{}, err
	}
	if state.Status != StatusRunning && state.Status != StatusPaused {
		return ExperimentResponse{}, ErrInvalidTransition
	}
	if !req.Decision.Valid() {
		return ExperimentResponse{}, ErrInvalidTransition
	}
	if strings.TrimSpace(req.Reason) == "" {
		return ExperimentResponse{}, ErrReasonRequired
	}

	if req.Decision == DecisionRolloutWinner {
		if err := s.rolloutWinner(ctx, callerID, state, req.WinnerVariantID); err != nil {
			return ExperimentResponse{}, err
		}
	}

	if _, err := s.repo.Complete(ctx, id, state.Status, req.Version, callerID, req.Decision, strings.TrimSpace(req.Reason)); err != nil {
		return ExperimentResponse{}, err
	}

	s.log.Info("experiment completed",
		zap.String(logger.FieldExperimentID, id),
		zap.String(logger.FieldExperimentStatus, string(req.Decision)),
		zap.String(logger.FieldActorID, callerID),
	)

	s.notifyRefresh()

	return s.getDetail(ctx, id)
}

func (s *Service) Rollout(ctx context.Context, callerID, id string, req RolloutRequest) (ExperimentResponse, error) {
	return s.Complete(ctx, callerID, id, CompleteRequest{
		Version:         req.Version,
		Decision:        DecisionRolloutWinner,
		Reason:          req.Reason,
		WinnerVariantID: req.WinnerVariantID,
	})
}

func (s *Service) rolloutWinner(ctx context.Context, callerID string, state Experiment, winnerVariantID *string) error {
	if winnerVariantID == nil || *winnerVariantID == "" {
		return ErrWinnerRequired
	}
	ver, err := s.currentVersion(ctx, state)
	if err != nil {
		return err
	}
	variants, err := s.repo.ListVariants(ctx, ver.ID)
	if err != nil {
		return err
	}
	var winner *Variant
	for i := range variants {
		if variants[i].ID == *winnerVariantID {
			winner = &variants[i]
			break
		}
	}
	if winner == nil {
		return ErrWinnerRequired
	}

	_, err = s.flags.Update(ctx, callerID, state.FlagID, flagsdomain.UpdateFlagRequest{
		DefaultValue: flagsdomain.ValueFlag(winner.Value),
	})
	return err
}
