package flags

import (
	"context"
	"errors"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"go.uber.org/zap"
)

var (
	ErrInvalidTypeFlag = errors.New("invalid type flag")
	ErrInvalidValue    = errors.New("invalid default value")
)

type FlagRepo interface {
	Create(ctx context.Context, f Flag) (int64, error)
	GetByID(ctx context.Context, id int64) (FlagWithCreatorAndUpdater, error)
	List(ctx context.Context, limit, offset int) ([]Flag, error)
	Update(ctx context.Context, id int64, key, name string, defaultValue ValueFlag, description string, updatedBy int64) (FlagWithCreatorAndUpdater, error)
	Delete(ctx context.Context, id int64) error
	Count(ctx context.Context) (int64, error)
}

type Service struct {
	repo FlagRepo
	log  *zap.Logger
}

func NewService(repo FlagRepo, log *zap.Logger) *Service {
	return &Service{repo, log}
}

func (s *Service) Create(ctx context.Context, callerID int64, req CreateFlagRequest) (FlagResponse, error) {
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

	s.log.Debug("flag created", zap.Int64("id", id), zap.String("key", f.Key))

	return s.GetByID(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id int64) (FlagResponse, error) {
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

func (s *Service) Update(ctx context.Context, callerID, id int64, req UpdateFlagRequest) (FlagResponse, error) {
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

	return ToResponse(fwo), nil
}

func (s *Service) Delete(ctx context.Context, callerID, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	return nil
}
