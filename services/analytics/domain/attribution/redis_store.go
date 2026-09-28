package attribution

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

type RedisPendingStore struct {
	redis *rueidis.Client
	log   *zap.Logger
}

func NewRedisPendingStore(redis *rueidis.Client, log *zap.Logger) *RedisPendingStore {
	return &RedisPendingStore{redis: redis, log: log.Named("pending_store")}
}

func (s *RedisPendingStore) RegisterExposure(ctx context.Context, decisionID string, pe PendingExposure, ttl time.Duration) error {
	raw, err := json.Marshal(pe)
	if err != nil {
		return fmt.Errorf("marshal pending exposure: %w", err)
	}

	key := pendingExposureKeyPrefix + decisionID
	err = (*s.redis).Do(ctx, (*s.redis).B().Set().Key(key).Value(string(raw)).Nx().Ex(ttl).Build()).Error()
	if err != nil {
		return fmt.Errorf("register pending exposure: %w", err)
	}

	return nil
}

func (s *RedisPendingStore) GetExposure(ctx context.Context, decisionID string) (*PendingExposure, error) {
	key := pendingExposureKeyPrefix + decisionID
	raw, err := (*s.redis).Do(ctx, (*s.redis).B().Get().Key(key).Build()).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending exposure: %w", err)
	}

	var pe PendingExposure
	if err := json.Unmarshal([]byte(raw), &pe); err != nil {
		return nil, fmt.Errorf("unmarshal pending exposure: %w", err)
	}

	return &pe, nil
}

func (s *RedisPendingStore) DeleteExposure(ctx context.Context, decisionID string) error {
	key := pendingExposureKeyPrefix + decisionID
	if err := (*s.redis).Do(ctx, (*s.redis).B().Del().Key(key).Build()).Error(); err != nil {
		return fmt.Errorf("delete pending exposure: %w", err)
	}

	return nil
}

func (s *RedisPendingStore) RegisterConversion(ctx context.Context, decisionID string, pc PendingConversion, ttl time.Duration) error {
	raw, err := json.Marshal(pc)
	if err != nil {
		return fmt.Errorf("marshal pending conversion: %w", err)
	}

	key := pendingConversionKeyPrefix + decisionID
	err = (*s.redis).Do(ctx, (*s.redis).B().Set().Key(key).Value(string(raw)).Nx().Ex(ttl).Build()).Error()
	if err != nil {
		return fmt.Errorf("register pending conversion: %w", err)
	}

	err = (*s.redis).Do(ctx, (*s.redis).B().Zadd().Key(pendingConversionsZSet).ScoreMember().ScoreMember(float64(pc.TTLAt.Unix()), decisionID).Build()).Error()
	if err != nil {
		return fmt.Errorf("index pending conversion: %w", err)
	}

	return nil
}

func (s *RedisPendingStore) GetConversion(ctx context.Context, decisionID string) (*PendingConversion, error) {
	key := pendingConversionKeyPrefix + decisionID
	raw, err := (*s.redis).Do(ctx, (*s.redis).B().Get().Key(key).Build()).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending conversion: %w", err)
	}

	var pc PendingConversion
	if err := json.Unmarshal([]byte(raw), &pc); err != nil {
		return nil, fmt.Errorf("unmarshal pending conversion: %w", err)
	}

	return &pc, nil
}

func (s *RedisPendingStore) DeleteConversion(ctx context.Context, decisionID string) error {
	key := pendingConversionKeyPrefix + decisionID
	if err := (*s.redis).Do(ctx, (*s.redis).B().Del().Key(key).Build()).Error(); err != nil {
		return fmt.Errorf("delete pending conversion: %w", err)
	}

	if err := (*s.redis).Do(ctx, (*s.redis).B().Zrem().Key(pendingConversionsZSet).Member(decisionID).Build()).Error(); err != nil {
		return fmt.Errorf("unindex pending conversion: %w", err)
	}

	return nil
}

func (s *RedisPendingStore) ExpiredConversions(ctx context.Context, now time.Time) ([]string, error) {
	res, err := (*s.redis).Do(ctx, (*s.redis).B().Zrangebyscore().Key(pendingConversionsZSet).Min("0").Max(fmt.Sprintf("%d", now.Unix())).Build()).AsStrSlice()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get expired conversions: %w", err)
	}

	return res, nil
}
