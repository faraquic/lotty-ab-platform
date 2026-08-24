package users

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidRole = errors.New("invalid role")

type UserRepo interface {
	Create(ctx context.Context, u User) (int64, error)
	GetByID(ctx context.Context, id int64) (User, error)
	List(ctx context.Context, limit, offset int) ([]User, error)
	Update(ctx context.Context, id int64, email *string, role *Role) (User, error)
	Delete(ctx context.Context, id int64) error
}

type Service struct {
	repo UserRepo
	log  *zap.Logger
}

func NewService(repo UserRepo, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) Create(ctx context.Context, req CreateUserRequest) (UserResponse, error) {
	role := Role(req.Role)
	if !role.Valid() {
		return UserResponse{}, ErrInvalidRole
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserResponse{}, fmt.Errorf("hash password: %w", err)
	}

	u := User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         role,
	}

	id, err := s.repo.Create(ctx, u)
	if err != nil {
		return UserResponse{}, err
	}

	s.log.Debug("user created", zap.Int64("id", id), zap.String("username", u.Username))

	return s.GetByID(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id int64) (UserResponse, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return UserResponse{}, err
	}

	return toResponse(u), nil
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]UserResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	usersList, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	resp := make([]UserResponse, 0, len(usersList))
	for _, u := range usersList {
		resp = append(resp, toResponse(u))
	}

	return resp, nil
}

func (s *Service) Update(ctx context.Context, id int64, req UpdateUserRequest) (UserResponse, error) {
	var role *Role
	if req.Role != nil {
		r := Role(*req.Role)
		if !r.Valid() {
			return UserResponse{}, ErrInvalidRole
		}
		role = &r
	}

	u, err := s.repo.Update(ctx, id, req.Email, role)
	if err != nil {
		return UserResponse{}, err
	}

	return toResponse(u), nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
