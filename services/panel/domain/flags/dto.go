package flags

import (
	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

type CreateFlagRequest struct {
	Key          string    `json:"key" binding:"required,min=3,max=128"`
	Name         string    `json:"name" binding:"required,min=1,max=256"`
	Type         TypeFlag  `json:"type" binding:"required,oneof=string number bool"`
	DefaultValue ValueFlag `json:"default_value" binding:"required"`
	Description  string    `json:"description"`
}

type UpdateFlagRequest struct {
	Key          string    `json:"key" binding:"omitempty,min=3,max=128"`
	Name         string    `json:"name" binding:"omitempty,min=1,max=256"`
	DefaultValue ValueFlag `json:"default_value" binding:"omitempty"`
	Description  string    `json:"description"`
}

type FlagResponse struct {
	api.ResourceResponse
	Key          string              `json:"key"`
	Name         string              `json:"name"`
	Type         TypeFlag            `json:"type"`
	DefaultValue ValueFlag           `json:"default_value"`
	Description  *string             `json:"description"`
	CreatedBy    *users.UserResponse `json:"created_by"`
	UpdatedBy    *users.UserResponse `json:"updated_by"`
}

type PaginatedFlagResponse struct {
	Data []FlagResponse     `json:"data"`
	Meta api.PaginationMeta `json:"meta"`
}

func ToResponse(f FlagWithCreatorAndUpdater) FlagResponse {
	var createdBy *users.UserResponse
	if f.CreatedBy != nil {
		resp := users.ToResponse(*f.CreatedBy)
		createdBy = &resp
	}
	var updatedBy *users.UserResponse
	if f.UpdatedBy != nil {
		resp := users.ToResponse(*f.UpdatedBy)
		updatedBy = &resp
	}
	return FlagResponse{
		ID:           f.Flag.ID,
		CreatedAt:    f.Flag.CreatedAt,
		UpdatedAt:    f.Flag.UpdatedAt,
		Key:          f.Flag.Key,
		Name:         f.Flag.Name,
		Type:         f.Flag.Type,
		DefaultValue: f.Flag.DefaultValue,
		Description:  f.Flag.Description,
		CreatedBy:    createdBy,
		UpdatedBy:    updatedBy,
	}
}
