package flags

import (
	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
)

type CreateFlagRequest struct {
	Key          string    `json:"key" binding:"required,min=3,max=128"`
	Type         TypeFlag  `json:"type" binding:"required,oneof=string number bool"`
	DefaultValue ValueFlag `json:"default_value" binding:"required"`
	Description  string    `json:"description" binding:"omitempty,min=1,max=4096"`
}

type UpdateFlagRequest struct {
	Key         string `json:"key" binding:"omitempty,min=3,max=128"`
	Description string `json:"description" binding:"omitempty,min=1,max=4096"`
}

type FlagResponse struct {
	api.ResourceResponse
	Key          string             `json:"key"`
	Type         TypeFlag           `json:"type"`
	DefaultValue ValueFlag          `json:"default_value"`
	Description  string             `json:"description"`
	Owner        users.UserResponse `json:"owner"`
}

type PaginatedFlagResponse struct {
	Data []FlagResponse     `json:"data"`
	Meta api.PaginationMeta `json:"meta"`
}

func ToResponse(fwo FlagWithOwner) FlagResponse {
	return FlagResponse{
		ID:           fwo.Flag.ID,
		CreatedAt:    fwo.Flag.CreatedAt,
		UpdatedAt:    fwo.Flag.UpdatedAt,
		Key:          fwo.Flag.Key,
		Type:         fwo.Flag.Type,
		DefaultValue: fwo.Flag.DefaultValue,
		Description:  fwo.Flag.Description,
		Owner:        users.ToResponse(fwo.Owner),
	}
}

func toFlagResponseList(list []FlagWithOwner) []FlagResponse {
	resp := make([]FlagResponse, 0, len(list))
	for _, fwo := range list {
		resp = append(resp, ToResponse(fwo))
	}
	return resp
}
