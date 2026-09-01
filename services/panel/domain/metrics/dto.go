package metrics

import (
	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

type CreateMetricRequest struct {
	Key         string       `json:"key" binding:"required,min=3,max=128"`
	Name        string       `json:"name" binding:"required,min=1,max=256"`
	Description string       `json:"description"`
	MetricType  string       `json:"metric_type" binding:"required,oneof=count sum unique_count ratio percentile average"`
	Aggregation MetricConfig `json:"aggregation" binding:"required"`
	Attribution MetricConfig `json:"attribution" binding:"required"`
}

type UpdateMetricRequest struct {
	Key         string       `json:"key" binding:"omitempty,min=3,max=128"`
	Name        string       `json:"name" binding:"omitempty,min=1,max=256"`
	Description string       `json:"description"`
	Aggregation MetricConfig `json:"aggregation" binding:"omitempty"`
	Attribution MetricConfig `json:"attribution" binding:"omitempty"`
	Status      string       `json:"status" binding:"omitempty,oneof=active archived"`
}

type MetricResponse struct {
	api.ResourceResponse
	Key         string              `json:"key"`
	Name        string              `json:"name"`
	Description *string             `json:"description"`
	MetricType  string              `json:"metric_type"`
	Aggregation MetricConfig        `json:"aggregation"`
	Attribution MetricConfig        `json:"attribution"`
	IsBuiltin   bool                `json:"is_builtin"`
	Status      string              `json:"status"`
	CreatedBy   *users.UserResponse `json:"created_by"`
	UpdatedBy   *users.UserResponse `json:"updated_by"`
}

type PaginatedMetricResponse struct {
	Data []MetricResponse   `json:"data"`
	Meta api.PaginationMeta `json:"meta"`
}

func ToResponse(m MetricWithCreatorAndUpdater) MetricResponse {
	var createdBy *users.UserResponse
	if m.CreatedBy != nil {
		resp := users.ToResponse(*m.CreatedBy)
		createdBy = &resp
	}
	var updatedBy *users.UserResponse
	if m.UpdatedBy != nil {
		resp := users.ToResponse(*m.UpdatedBy)
		updatedBy = &resp
	}
	return MetricResponse{
		ID:          m.Metric.ID,
		CreatedAt:   m.Metric.CreatedAt,
		UpdatedAt:   m.Metric.UpdatedAt,
		Key:         m.Metric.Key,
		Name:        m.Metric.Name,
		Description: m.Metric.Description,
		MetricType:  string(m.Metric.MetricType),
		Aggregation: m.Metric.Aggregation,
		Attribution: m.Metric.Attribution,
		IsBuiltin:   m.Metric.IsBuiltin,
		Status:      string(m.Metric.Status),
		CreatedBy:   createdBy,
		UpdatedBy:   updatedBy,
	}
}
