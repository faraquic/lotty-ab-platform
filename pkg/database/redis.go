package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

func NewRedis(ctx context.Context, url string, log *zap.Logger) (*rueidis.Client, error) {
	if url == "" {
		return nil, errors.New("redis url is empty")
	}

	opt, err := rueidis.ParseURL(toRedisURL(url))
	if err != nil {
		return nil, fmt.Errorf("parse redis config: %w", err)
	}

	client, err := rueidis.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("create redis client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := client.Do(pingCtx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info(
		"connected to redis",
		zap.Any(logger.FieldCacheAddrs, opt.InitAddress),
		zap.Int(logger.FieldCacheDB, opt.SelectDB),
	)

	return &client, nil
}

// toRedisURL accepts both plain host:port and full redis:// URLs;
// rueidis.ParseURL only understands the latter.
func toRedisURL(url string) string {
	if strings.Contains(url, "://") {
		return url
	}

	return "redis://" + url
}
