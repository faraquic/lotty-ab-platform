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

	"github.com/alexedwards/argon2id"
	libauth "github.com/faraquic/lotty-ab-platform/pkg/auth"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

const (
	sessionKeyPrefix = "labp:panel:auth:jwt:"
	redisOpTimeout   = 1 * time.Second
)

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

	if valid, err := argon2id.ComparePasswordAndHash(req.Password, hash); !valid || err != nil {
		s.log.Debug(
			"login failed: invalid credentials",
			zap.String(logger.FieldAuthFailure, "password_mismatch"),
		)
		return "", time.Time{}, ErrInvalidCredentials
	}

	token, expiry, err := s.tokenizer.GenerateToken(id, role)
	if err != nil {
		return "", time.Time{}, err
	}

	s.log.Debug(
		"auth token generated",
		zap.String(logger.FieldUserID, id),
		zap.String(logger.FieldAuthTokenSrc, "login"),
	)

	s.StoreSession(ctx, token, id)

	return token, expiry, nil
}

// StoreSession remembers the issued token in Redis until it expires.
// Enables server-side revocation; on failure the token will not be found
// by SessionActive and subsequent requests will be denied.
func (s *Service) StoreSession(ctx context.Context, token string, userID string) {
	if s.redis == nil {
		s.log.Warn(
			"session store failed; redis unavailable",
			zap.String(logger.FieldUserID, userID),
			zap.String(logger.FieldCacheOperation, "set"),
			zap.String(logger.FieldCacheStatus, "unavailable"),
		)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	start := time.Now()

	err := (*s.redis).Do(ctx, (*s.redis).B().Set().Key(s.sessionKey(token, userID)).Value("1").Ex(s.ttl).Build()).Error()
	if err != nil {
		if isRedisTimeout(err) {
			s.log.Warn(
				"session store failed; redis timeout",
				zap.String(logger.FieldUserID, userID),
				zap.String(logger.FieldCacheOperation, "set"),
				zap.String(logger.FieldCacheStatus, "timeout"),
				zap.Error(err),
			)
		} else {
			s.log.Warn(
				"session store failed; redis connection error",
				zap.String(logger.FieldUserID, userID),
				zap.String(logger.FieldCacheOperation, "set"),
				zap.String(logger.FieldCacheStatus, "connection_error"),
				zap.Error(err),
			)
		}
		return
	}

	s.log.Debug(
		"session stored",
		zap.String(logger.FieldUserID, userID),
		zap.String(logger.FieldCacheOperation, "set"),
		zap.Float64(logger.FieldDurationMs, float64(time.Since(start).Nanoseconds())/1e6),
	)
}

// SessionActive reports whether the token is known to Redis.
// When Redis is unavailable or the check fails, access is denied (fail-closed).
func (s *Service) SessionActive(ctx context.Context, token string, userID string) bool {
	if token == "" || userID == "" {
		return false
	}
	if s.redis == nil {
		s.log.Warn(
			"session check failed; redis unavailable; denying",
			zap.String(logger.FieldCacheOperation, "exists"),
			zap.String(logger.FieldCacheStatus, "unavailable"),
			zap.String(logger.FieldUserID, userID),
		)
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	start := time.Now()

	exists, err := (*s.redis).Do(ctx, (*s.redis).B().Exists().Key(s.sessionKey(token, userID)).Build()).AsBool()
	if err != nil {
		if isRedisTimeout(err) {
			s.log.Warn(
				"session check failed; redis timeout; denying",
				zap.String(logger.FieldCacheOperation, "exists"),
				zap.String(logger.FieldCacheStatus, "timeout"),
				zap.String(logger.FieldUserID, userID),
				zap.Error(err),
			)
		} else {
			s.log.Warn(
				"session check failed; redis connection error; denying",
				zap.String(logger.FieldCacheOperation, "exists"),
				zap.String(logger.FieldCacheStatus, "connection_error"),
				zap.String(logger.FieldUserID, userID),
				zap.Error(err),
			)
		}
		return false
	}

	s.log.Debug(
		"session checked",
		zap.String(logger.FieldUserID, userID),
		zap.String(logger.FieldCacheOperation, "exists"),
		zap.Bool(logger.FieldCacheHit, exists),
		zap.Float64(logger.FieldDurationMs, float64(time.Since(start).Nanoseconds())/1e6),
	)

	return exists
}

func (s *Service) sessionKey(token string, userID string) string {
	sum := sha256.Sum256([]byte(token))

	return sessionKeyPrefix + userID + ":" + hex.EncodeToString(sum[:])
}

func isRedisTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var te interface{ Timeout() bool }
	if errors.As(err, &te) && te.Timeout() {
		return true
	}
	return false
}

// RevokeUserSessions deletes every session of the user.
// Called when a user's role changes or the user is deleted,
// so stale tokens cannot act under outdated permissions.
func (s *Service) RevokeUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	if s.redis == nil {
		s.log.Warn(
			"session revoke failed; redis unavailable",
			zap.String(logger.FieldUserID, userID),
			zap.String(logger.FieldCacheOperation, "scan"),
			zap.String(logger.FieldCacheStatus, "unavailable"),
		)
		return errors.New("redis unavailable")
	}

	pattern := sessionKeyPrefix + userID + ":*"
	var keys []string
	var cursor uint64

	for {
		scanCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
		entry, err := (*s.redis).Do(
			scanCtx,
			(*s.redis).B().Scan().Cursor(cursor).Match(pattern).Count(100).Build(),
		).AsScanEntry()
		cancel()
		if err != nil {
			if isRedisTimeout(err) {
				s.log.Warn(
					"session revoke failed; redis timeout",
					zap.String(logger.FieldUserID, userID),
					zap.String(logger.FieldCacheOperation, "scan"),
					zap.String(logger.FieldCacheStatus, "timeout"),
					zap.Error(err),
				)
			} else {
				s.log.Warn(
					"session revoke failed; redis connection error",
					zap.String(logger.FieldUserID, userID),
					zap.String(logger.FieldCacheOperation, "scan"),
					zap.String(logger.FieldCacheStatus, "connection_error"),
					zap.Error(err),
				)
			}
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

	delCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	if err := (*s.redis).Do(delCtx, (*s.redis).B().Del().Key(keys...).Build()).Error(); err != nil {
		if isRedisTimeout(err) {
			s.log.Warn(
				"session revoke failed; redis timeout",
				zap.String(logger.FieldUserID, userID),
				zap.String(logger.FieldCacheOperation, "del"),
				zap.String(logger.FieldCacheStatus, "timeout"),
				zap.Error(err),
			)
		} else {
			s.log.Warn(
				"session revoke failed; redis connection error",
				zap.String(logger.FieldUserID, userID),
				zap.String(logger.FieldCacheOperation, "del"),
				zap.String(logger.FieldCacheStatus, "connection_error"),
				zap.Error(err),
			)
		}
		return fmt.Errorf("delete sessions: %w", err)
	}

	s.log.Info(
		"user sessions revoked",
		zap.String(logger.FieldUserID, userID),
		zap.Int(logger.FieldAuthSessionN, len(keys)),
	)

	return nil
}
