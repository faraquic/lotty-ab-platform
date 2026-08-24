package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const pingTimeout = 3 * time.Second

func NewPostgres(ctx context.Context, dsn string, maxConns int32, minConns int32, log *zap.Logger) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, errors.New("postgres dsn is empty")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pool.Config().MaxConns = maxConns
	pool.Config().MinConns = minConns

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	log.Info("connected to postgres",
		zap.String("host", cfg.ConnConfig.Host),
		zap.Uint16("port", cfg.ConnConfig.Port),
		zap.String("database", cfg.ConnConfig.Database),
	)

	return pool, nil
}
