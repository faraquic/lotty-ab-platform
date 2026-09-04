package users

import "github.com/faraquic/lotty-ab-platform/pkg/api"

type CreateUserRequest struct {
	FullName   string `json:"full_name" binding:"required,min=1,max=256"`
	Email      string `json:"email" binding:"required,email,min=5,max=96"`
	Password   string `json:"password" binding:"required,min=8"`
	Role       string `json:"role" binding:"required,oneof=admin experimenter approver viewer"`
}

type UpdateUserRequest struct {
	Email *string `json:"email" binding:"omitempty,email"`
	Role  *string `json:"role" binding:"omitempty,oneof=admin experimenter approver viewer"`
}

type UserResponse struct {
	api.ResourceResponse
	FullName    string  `json:"full_name"`
	Email       string  `json:"email"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatar_url"`
}

type PaginatedUserResponse struct {
	Data []UserResponse     `json:"data"`
	Meta api.PaginationMeta `json:"meta"`
}

func ToResponse(u User) UserResponse {
	var avatarURL *string
	if u.AvatarURL != "" {
		avatarURL = &u.AvatarURL
	}
	return UserResponse{
		ID:        u.ID,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		FullName:  u.FullName,
		Email:     u.Email,
		Role:      string(u.Role),
		AvatarURL: avatarURL,
	}
}