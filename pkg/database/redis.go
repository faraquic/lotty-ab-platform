package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewRedis(ctx context.Context, addr string, password string, db int, log *zap.Logger) (*redis.Client, error) {
	if addr == "" {
		return nil, errors.New("redis addr is empty")
	}

	opt, err := parseRedisOptions(addr)
	if err != nil {
		return nil, fmt.Errorf("parse redis config: %w", err)
	}
	opt.DB = db
	opt.Password = password

	client := redis.NewClient(opt)

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info("connected to redis",
		zap.String("addr", opt.Addr),
		zap.Int("db", opt.DB),
	)

	return client, nil
}

func parseRedisOptions(addr string) (*redis.Options, error) {
	if strings.Contains(addr, "://") {
		return redis.ParseURL(addr)
	}

	return &redis.Options{Addr: addr}, nil
}
