package flags

import (
	"context"
	"errors"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	panelsnapshot "github.com/faraquic/lotty-ab-platform/services/panel/snapshot"
	"go.uber.org/zap"
)

var (
	ErrInvalidTypeFlag = errors.New("invalid type flag")
	ErrInvalidValue    = errors.New("invalid default value")
)

type FlagRepo interface {
	Create(ctx context.Context, f Flag) (string, error)
	GetByID(ctx context.Context, id string) (FlagWithCreatorAndUpdater, error)
	List(ctx context.Context, limit, offset int) ([]Flag, error)
	Update(ctx context.Context, id string, key, name string, defaultValue ValueFlag, description string, updatedBy string) (FlagWithCreatorAndUpdater, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

type Service struct {
	repo      FlagRepo
	refresher panelsnapshot.Refresher
	log       *zap.Logger
}

func NewService(repo FlagRepo, refresher panelsnapshot.Refresher, log *zap.Logger) *Service {
	return &Service{repo, refresher, log}
}

func (s *Service) Create(ctx context.Context, callerID string, req CreateFlagRequest) (FlagResponse, error) {
	typeFlag := TypeFlag(req.Type)
	if !typeFlag.Valid() {
		return FlagResponse{}, ErrInvalidTypeFlag
	}

	dv := ValueFlag(req.DefaultValue)
	if !dv.Valid(typeFlag) {
		return FlagResponse{}, ErrInvalidValue
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	f := Flag{
		Key:          req.Key,
		Name:         req.Name,
		Type:         typeFlag,
		DefaultValue: dv,
		Description:  desc,
		CreatedBy:    callerID,
		UpdatedBy:    callerID,
	}

	id, err := s.repo.Create(ctx, f)
	if err != nil {
		return FlagResponse{}, err
	}

	s.log.Info("flag created",
		zap.String(logger.FieldFlagID, id),
		zap.String(logger.FieldFlagKey, f.Key),
		zap.String(logger.FieldFlagType, string(typeFlag)),
		zap.String(logger.FieldActorID, callerID),
	)

	s.notifyRefresh()

	return s.GetByID(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id string) (FlagResponse, error) {
	fwo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return FlagResponse{}, err
	}

	return ToResponse(fwo), nil
}

func (s *Service) List(ctx context.Context, limit, offset int) (PaginatedFlagResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		return PaginatedFlagResponse{}, err
	}

	flagsList, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return PaginatedFlagResponse{}, err
	}

	resp := make([]FlagResponse, 0, len(flagsList))
	for _, f := range flagsList {
		resp = append(resp, ToResponse(FlagWithCreatorAndUpdater{Flag: f}))
	}

	count := len(resp)
	hasNext := int64(offset)+int64(count) < total

	return PaginatedFlagResponse{
		Data: resp,
		Meta: api.PaginationMeta{
			Limit:   limit,
			Offset:  offset,
			Count:   count,
			Total:   total,
			HasNext: hasNext,
		},
	}, nil
}

func (s *Service) Update(ctx context.Context, callerID, id string, req UpdateFlagRequest) (FlagResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return FlagResponse{}, err
	}

	if len(req.DefaultValue) > 0 {
		dv := ValueFlag(req.DefaultValue)
		if !dv.Valid(existing.Flag.Type) {
			return FlagResponse{}, ErrInvalidValue
		}
	}

	fwo, err := s.repo.Update(ctx, id, req.Key, req.Name, req.DefaultValue, req.Description, callerID)
	if err != nil {
		return FlagResponse{}, err
	}

	s.log.Debug("flag updated",
		zap.String(logger.FieldFlagID, id),
		zap.String(logger.FieldFlagKey, fwo.Flag.Key),
		zap.String(logger.FieldActorID, callerID),
	)

	s.notifyRefresh()

	return ToResponse(fwo), nil
}

func (s *Service) Delete(ctx context.Context, callerID, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.log.Info("flag deleted",
		zap.String(logger.FieldFlagID, id),
		zap.String(logger.FieldActorID, callerID),
	)

	s.notifyRefresh()

	return nil
}

func (s *Service) notifyRefresh() {
	s.refresher.Refresh()
}
