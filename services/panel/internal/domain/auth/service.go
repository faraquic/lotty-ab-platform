package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	libauth "github.com/faraquic/lotty-ab-platform/services/panel/internal/lib/auth"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

const sessionKeyPrefix = "panel:sessions:"

type Service struct {
	repo      AuthRepo
	tokenizer libauth.Tokenizer
	redis     *redis.Client
	ttl       time.Duration
	redisTTL  time.Duration
	log       *zap.Logger
}

func NewService(repo AuthRepo, tokenizer libauth.Tokenizer, redisClient *redis.Client, ttl, redisTTL time.Duration, log *zap.Logger) *Service {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	return &Service{
		repo:      repo,
		tokenizer: tokenizer,
		redis:     redisClient,
		ttl:       ttl,
		redisTTL:  redisTTL,
		log:       log,
	}
}

// ExpiresAt mirrors the exp claim minted by lib/auth (now + TTL).
func (s *Service) ExpiresAt() time.Time {
	return time.Now().Add(s.ttl)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (string, error) {
	id, hash, err := s.repo.GetCredentialsByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.HashPassword)); err != nil {
		s.log.Debug("login failed: password mismatch", zap.Int64("user_id", id))
		return "", ErrInvalidCredentials
	}

	token, err := s.tokenizer.GenerateToken(id)
	if err != nil {
		return "", err
	}

	s.StoreSession(ctx, token, id)

	return token, nil
}

// StoreSession remembers the issued token in Redis until it expires.
// Enables server-side revocation; failures degrade gracefully.
func (s *Service) StoreSession(ctx context.Context, token string, userID int64) {
	if s.redis == nil || s.redisTTL <= 0 {
		return
	}

	err := s.redis.Set(ctx, s.sessionKey(token), userID, s.redisTTL).Err()
	if err != nil {
		s.log.Warn("failed to store session in redis",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
	}
}

// SessionActive reports whether the token is known to Redis.
// When Redis is unavailable the check fails open: JWT validation remains
// the source of truth rather than taking the whole service down.
func (s *Service) SessionActive(ctx context.Context, token string) bool {
	if s.redis == nil || token == "" {
		return true
	}

	count, err := s.redis.Exists(ctx, s.sessionKey(token)).Result()
	if err != nil {
		s.log.Warn("session check failed, failing open", zap.Error(err))
		return true
	}

	return count == 1
}

func (s *Service) sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))

	return sessionKeyPrefix + hex.EncodeToString(sum[:])
}
