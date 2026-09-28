package database

import (
	"context"
	"errors"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/zap"
)

func NewClickHouse(ctx context.Context, dsn string, log *zap.Logger) (clickhouse.Conn, error) {
	if dsn == "" {
		return nil, errors.New("clickhouse dsn is empty")
	}

	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	if opts.DialTimeout == 0 {
		opts.DialTimeout = 5 * time.Second
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := conn.Ping(pingCtx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	log.Info("connected to clickhouse")

	return conn, nil
}
