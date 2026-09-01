package metrics

import (
	"context"
	"errors"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrInvalidMetricType = errors.New("invalid metric type")
)

type MetricRepo interface {
	Create(ctx context.Context, m Metric) (int64, error)
	GetByID(ctx context.Context, id int64, includeArchived bool) (MetricWithCreatorAndUpdater, error)
	List(ctx context.Context, limit, offset int, includeArchived bool) ([]Metric, error)
	Update(ctx context.Context, id int64, key, name, description string, aggregation MetricConfig, attribution MetricConfig, status MetricStatus, updatedBy int64) (MetricWithCreatorAndUpdater, error)
	Count(ctx context.Context, includeArchived bool) (int64, error)
}

type Service struct {
	repo MetricRepo
	log  *zap.Logger
}

func NewService(repo MetricRepo, log *zap.Logger) *Service {
	return &Service{repo, log}
}

func (s *Service) Create(ctx context.Context, callerID int64, req CreateMetricRequest) (MetricResponse, error) {
	metricType := MetricType(req.MetricType)
	if !metricType.Valid() {
		return MetricResponse{}, ErrInvalidMetricType
	}

	aggregation := MetricConfig(req.Aggregation)
	if !aggregation.Valid() {
		return MetricResponse{}, ErrInvalidMetricConfig
	}

	attribution := MetricConfig(req.Attribution)
	if !attribution.Valid() {
		return MetricResponse{}, ErrInvalidMetricConfig
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	m := Metric{
		Key:         req.Key,
		Name:        req.Name,
		Description: desc,
		MetricType:  metricType,
		Aggregation: aggregation,
		Attribution: attribution,
		IsBuiltin:   false,
		Status:      MetricStatusActive,
		CreatedBy:   callerID,
		UpdatedBy:   callerID,
	}

	id, err := s.repo.Create(ctx, m)
	if err != nil {
		return MetricResponse{}, err
	}

	s.log.Info("metric created",
		zap.Int64(logger.FieldMetricID, id),
		zap.String(logger.FieldMetricKey, m.Key),
		zap.String(logger.FieldMetricType, string(metricType)),
		zap.Int64(logger.FieldActorID, callerID),
	)

	return s.GetByID(ctx, id, false)
}

func (s *Service) GetByID(ctx context.Context, id int64, includeArchived bool) (MetricResponse, error) {
	mwo, err := s.repo.GetByID(ctx, id, includeArchived)
	if err != nil {
		return MetricResponse{}, err
	}

	return ToResponse(mwo), nil
}

func (s *Service) List(ctx context.Context, limit, offset int, includeArchived bool) (PaginatedMetricResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.Count(ctx, includeArchived)
	if err != nil {
		return PaginatedMetricResponse{}, err
	}

	metricsList, err := s.repo.List(ctx, limit, offset, includeArchived)
	if err != nil {
		return PaginatedMetricResponse{}, err
	}

	resp := make([]MetricResponse, 0, len(metricsList))
	for _, m := range metricsList {
		resp = append(resp, ToResponse(MetricWithCreatorAndUpdater{Metric: m}))
	}

	count := len(resp)
	hasNext := int64(offset)+int64(count) < total

	return PaginatedMetricResponse{
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

func (s *Service) Update(ctx context.Context, callerID, id int64, req UpdateMetricRequest) (MetricResponse, error) {
	existing, err := s.repo.GetByID(ctx, id, true)
	if err != nil {
		return MetricResponse{}, err
	}

	if existing.Metric.IsBuiltin {
		if req.Key != "" {
			return MetricResponse{}, ErrBuiltinProtected
		}
		if len(req.Aggregation) > 0 {
			return MetricResponse{}, ErrBuiltinProtected
		}
		if len(req.Attribution) > 0 {
			return MetricResponse{}, ErrBuiltinProtected
		}
	}

	if len(req.Aggregation) > 0 {
		agg := MetricConfig(req.Aggregation)
		if !agg.Valid() {
			return MetricResponse{}, ErrInvalidMetricConfig
		}
	}

	if len(req.Attribution) > 0 {
		attr := MetricConfig(req.Attribution)
		if !attr.Valid() {
			return MetricResponse{}, ErrInvalidMetricConfig
		}
	}

	fwo, err := s.repo.Update(ctx, id, req.Key, req.Name, req.Description,
		req.Aggregation, req.Attribution, MetricStatus(req.Status), callerID)
	if err != nil {
		return MetricResponse{}, err
	}

	s.log.Debug("metric updated",
		zap.Int64(logger.FieldMetricID, id),
		zap.String(logger.FieldMetricKey, fwo.Metric.Key),
		zap.Int64(logger.FieldActorID, callerID),
	)

	return ToResponse(fwo), nil
}
