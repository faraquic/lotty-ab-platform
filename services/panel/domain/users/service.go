package users

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"go.uber.org/zap"
	"github.com/alexedwards/argon2id"
)

var (
	ErrInvalidRole        = errors.New("invalid role")
	ErrSelfRoleChange     = errors.New("cannot change your own role")
	ErrSelfDelete         = errors.New("cannot delete your own account")
	ErrLastAdmin          = errors.New("cannot remove the last admin")
	ErrInvalidFileType    = errors.New("unsupported file type; allowed: jpeg, png, webp")
	ErrFileTooLarge       = errors.New("file too large; maximum 5 MB")
	ErrStorageUnavailable = errors.New("storage unavailable")
)

type UserRepo interface {
	Create(ctx context.Context, u User) (string, error)
	GetByID(ctx context.Context, id string) (User, error)
	List(ctx context.Context, limit, offset int) ([]User, error)
	Update(ctx context.Context, id string, email *string, role *Role) (User, error)
	Delete(ctx context.Context, id string) error
	UpdateAvatarURL(ctx context.Context, id string, avatarURL string) error
	Count(ctx context.Context) (int64, error)
	CountAdmins(ctx context.Context) (int64, error)
}

// SessionRevoker invalidates a user's auth sessions (implemented by
// the auth domain). Optional; nil means tokens stay valid until expiry.
type SessionRevoker interface {
	RevokeUserSessions(ctx context.Context, userID string) error
}

type Service struct {
	repo    UserRepo
	revoker SessionRevoker
	storage Storage
	log     *zap.Logger
}

func NewService(repo UserRepo, revoker SessionRevoker, storage Storage, log *zap.Logger) *Service {
	return &Service{repo, revoker, storage, log}
}

func (s *Service) Create(ctx context.Context, req CreateUserRequest) (UserResponse, error) {
	role := Role(req.Role)
	if !role.Valid() {
		return UserResponse{}, ErrInvalidRole
	}

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		return UserResponse{}, fmt.Errorf("hash password: %w", err)
	}

	u := User{
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         role,
	}

	id, err := s.repo.Create(ctx, u)
	if err != nil {
		return UserResponse{}, err
	}

	s.log.Info("user created",
		zap.String(logger.FieldUserID, id),
		zap.String(logger.FieldUserRole, string(role)),
	)

	return s.GetByID(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id string) (UserResponse, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return UserResponse{}, err
	}

	return ToResponse(u), nil
}

func (s *Service) List(ctx context.Context, limit, offset int) (PaginatedUserResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		return PaginatedUserResponse{}, err
	}

	usersList, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return PaginatedUserResponse{}, err
	}

	resp := make([]UserResponse, 0, len(usersList))
	for _, u := range usersList {
		resp = append(resp, ToResponse(u))
	}

	count := len(resp)
	hasNext := int64(offset+count) < total

	return PaginatedUserResponse{
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

func (s *Service) Update(ctx context.Context, callerID, id string, req UpdateUserRequest) (UserResponse, error) {
	var role *Role
	if req.Role != nil {
		r := Role(*req.Role)
		if !r.Valid() {
			return UserResponse{}, ErrInvalidRole
		}
		role = &r
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return UserResponse{}, err
	}

	if role != nil {
		// an admin must not change their own role: self-demotion would
		// lock them out, self-promotion bypasses separation of duties
		if callerID == id && current.Role != *role {
			return UserResponse{}, ErrSelfRoleChange
		}

		// keep at least one active admin in the system
		if current.Role == RoleAdmin && *role != RoleAdmin {
			admins, err := s.repo.CountAdmins(ctx)
			if err != nil {
				return UserResponse{}, err
			}
			if admins <= 1 {
				return UserResponse{}, ErrLastAdmin
			}
		}
	}

	u, err := s.repo.Update(ctx, id, req.Email, role)
	if err != nil {
		return UserResponse{}, err
	}

	// role change must invalidate existing sessions: the JWT carries
	// the old role until expiry, so stale tokens would keep old permissions
	if role != nil && current.Role != u.Role {
		s.log.Info("user role changed",
			zap.String(logger.FieldUserID, id),
			zap.String(logger.FieldUserOldRole, string(current.Role)),
			zap.String(logger.FieldUserNewRole, string(u.Role)),
		)
		s.revokeSessions(ctx, id)
	}

	return ToResponse(u), nil
}

func (s *Service) Delete(ctx context.Context, callerID, id string) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if callerID == id {
		return ErrSelfDelete
	}

	if current.Role == RoleAdmin {
		admins, err := s.repo.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if admins <= 1 {
			return ErrLastAdmin
		}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.log.Info("user deleted",
		zap.String(logger.FieldUserID, id),
		zap.String(logger.FieldUserRole, string(current.Role)),
	)

	s.revokeSessions(ctx, id)

	return nil
}

func (s *Service) EnsureBootstrapAdmin(ctx context.Context, fullName, email, passwordHash string) (bool, error) {
	if passwordHash == "" {
		return false, errors.New("bootstrap.password_hash is required")
	}

	n, err := s.repo.Count(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return false, nil
	}

	u := User{
		FullName:     fullName,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         RoleAdmin,
	}

	id, err := s.repo.Create(ctx, u)
	if err != nil {
		return false, fmt.Errorf("bootstrap admin: %w", err)
	}

	s.log.Info("bootstrap admin created",
		zap.String(logger.FieldUserID, id),
		zap.String(logger.FieldUserRole, string(RoleAdmin)),
	)

	return true, nil
}

const maxAvatarSize = 5 << 20 // 5 MB

var allowedAvatarTypes = map[string]bool{
	".jpeg": true,
	".jpg":  true,
	".png":  true,
	".webp": true,
}

func (s *Service) UploadAvatar(ctx context.Context, userID string, file *multipart.FileHeader) (UserResponse, error) {
	if s.storage == nil {
		return UserResponse{}, ErrStorageUnavailable
	}

	if file.Size == 0 {
		return UserResponse{}, ErrInvalidFileType
	}

	if file.Size > maxAvatarSize {
		return UserResponse{}, ErrFileTooLarge
	}

	ext := ExtFromFilename(file.Filename)
	if ext == "" || !allowedAvatarTypes[ext] {
		return UserResponse{}, ErrInvalidFileType
	}

	src, err := file.Open()
	if err != nil {
		return UserResponse{}, fmt.Errorf("open uploaded file: %w", err)
	}
	defer src.Close()

	buf := make([]byte, file.Size)
	if _, err := src.Read(buf); err != nil {
		return UserResponse{}, fmt.Errorf("read uploaded file: %w", err)
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return UserResponse{}, err
	}

	if user.AvatarURL != "" {
		if err := s.storage.DeleteAvatar(ctx, user.AvatarURL); err != nil {
			s.log.Warn("old avatar deletion failed",
				zap.String(logger.FieldUserID, userID),
				zap.Error(err),
			)
		}
	}

	avatarURL, err := s.storage.UploadAvatar(ctx, userID, ext, buf)
	if err != nil {
		return UserResponse{}, fmt.Errorf("upload avatar: %w", err)
	}

	if err := s.repo.UpdateAvatarURL(ctx, userID, avatarURL); err != nil {
		return UserResponse{}, err
	}

	s.log.Debug("avatar uploaded",
		zap.String(logger.FieldUserID, userID),
		zap.String(logger.FieldStorageOperation, "put_object"),
		zap.String(logger.FieldStoragePrefix, "avatars"),
	)

	return s.GetByID(ctx, userID)
}

func (s *Service) DeleteAvatar(ctx context.Context, userID string) (UserResponse, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return UserResponse{}, err
	}

	if user.AvatarURL == "" {
		return ToResponse(user), nil
	}

	if s.storage != nil {
		if err := s.storage.DeleteAvatar(ctx, user.AvatarURL); err != nil {
			s.log.Warn("avatar deletion from storage failed",
				zap.String(logger.FieldUserID, userID),
				zap.Error(err),
			)
		}
	}

	if err := s.repo.UpdateAvatarURL(ctx, userID, ""); err != nil {
		return UserResponse{}, err
	}

	s.log.Debug("avatar deleted",
		zap.String(logger.FieldUserID, userID),
		zap.String(logger.FieldStorageOperation, "delete_object"),
		zap.String(logger.FieldStoragePrefix, "avatars"),
	)

	return s.GetByID(ctx, userID)
}

func (s *Service) revokeSessions(ctx context.Context, userID string) error {
	if s.revoker == nil {
		return nil
	}

	if err := s.revoker.RevokeUserSessions(ctx, userID); err != nil {
		s.log.Warn("session revocation failed",
			zap.String(logger.FieldUserID, userID),
			zap.Error(err),
		)
	}

	return nil
}
