package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/rueidis"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	libauth "github.com/faraquic/lotty-ab-platform/pkg/auth"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

const sessionKeyPrefix = "labp:panel:auth:jwt:"

type Service struct {
	repo      AuthRepo
	tokenizer libauth.Tokenizer
	redis     *rueidis.Client
	ttl       time.Duration
	log       *zap.Logger
}

func NewService(repo AuthRepo, tokenizer libauth.Tokenizer, redisClient *rueidis.Client, ttl time.Duration, log *zap.Logger) *Service {
	return &Service{repo, tokenizer, redisClient, ttl, log}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (string, time.Time, error) {
	id, role, hash, err := s.repo.GetCredentialsByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", time.Time{}, ErrInvalidCredentials
		}
		return "", time.Time{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		s.log.Debug("login failed: password mismatch", zap.Int64("user_id", id))
		return "", time.Time{}, ErrInvalidCredentials
	}

	token, expiry, err := s.tokenizer.GenerateToken(id, role)
	if err != nil {
		return "", time.Time{}, err
	}

	s.StoreSession(ctx, token, id)

	return token, expiry, nil
}

// StoreSession remembers the issued token in Redis until it expires.
// Enables server-side revocation; failures degrade gracefully.
func (s *Service) StoreSession(ctx context.Context, token string, userID int64) {
	if s.redis == nil {
		return
	}

	err := (*s.redis).Do(ctx, (*s.redis).B().Set().Key(s.sessionKey(token, userID)).Value("1").Ex(s.ttl).Build()).Error()
	if err != nil {
		s.log.Warn(
			"failed to store session in redis",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
	}
}

// SessionActive reports whether the token is known to Redis.
// When Redis is unavailable the check fails open: JWT validation remains
// the source of truth rather than taking the whole service down.
func (s *Service) SessionActive(ctx context.Context, token string, userID int64) bool {
	if s.redis == nil || token == "" {
		return true
	}

	exists, err := (*s.redis).Do(ctx, (*s.redis).B().Exists().Key(s.sessionKey(token, userID)).Build()).AsBool()
	if err != nil {
		s.log.Warn("session check failed, failing open", zap.Error(err))
		return true
	}

	return exists
}

func (s *Service) sessionKey(token string, userID int64) string {
	sum := sha256.Sum256([]byte(token))

	return sessionKeyPrefix + fmt.Sprint(userID) + ":" + hex.EncodeToString(sum[:])
}

// RevokeUserSessions deletes every session of the user.
// Called when a user's role changes or the user is deleted,
// so stale tokens cannot act under outdated permissions.
func (s *Service) RevokeUserSessions(ctx context.Context, userID int64) error {
	if s.redis == nil || userID < 1 {
		return nil
	}

	pattern := sessionKeyPrefix + fmt.Sprint(userID) + ":*"
	var keys []string
	var cursor uint64

	for {
		entry, err := (*s.redis).Do(
			ctx,
			(*s.redis).B().Scan().Cursor(cursor).Match(pattern).Count(100).Build(),
		).AsScanEntry()
		if err != nil {
			return fmt.Errorf("scan sessions: %w", err)
		}

		keys = append(keys, entry.Elements...)
		cursor = entry.Cursor
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return nil
	}

	if err := (*s.redis).Do(ctx, (*s.redis).B().Del().Key(keys...).Build()).Error(); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	s.log.Info("user sessions revoked", zap.Int64("user_id", userID), zap.Int("count", len(keys)))

	return nil
}
