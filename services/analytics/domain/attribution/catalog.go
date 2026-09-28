package attribution

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type exposureCatalogQuerier interface {
	EventTypesWithExposure(ctx context.Context) (map[string]bool, error)
}

type PGExposureCatalog struct {
	DB *pgxpool.Pool
}

func (p PGExposureCatalog) EventTypesWithExposure(ctx context.Context) (map[string]bool, error) {
	rows, err := p.DB.Query(ctx, `SELECT key, require_exposure FROM event_types WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)

	for rows.Next() {
		var key string
		var requireExposure bool
		if err := rows.Scan(&key, &requireExposure); err != nil {
			return nil, err
		}

		out[key] = requireExposure
	}

	return out, rows.Err()
}

type ExposureCatalog struct {
	querier         exposureCatalogQuerier
	refreshInterval time.Duration

	mu    sync.RWMutex
	types map[string]bool

	log *zap.Logger
}

func NewExposureCatalog(querier exposureCatalogQuerier, refreshInterval time.Duration, log *zap.Logger) (*ExposureCatalog, error) {
	c := &ExposureCatalog{
		querier:         querier,
		refreshInterval: refreshInterval,
		types:           make(map[string]bool),
		log:             log,
	}

	if err := c.Reload(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ExposureCatalog) Reload(ctx context.Context) error {
	types, err := c.querier.EventTypesWithExposure(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.types = types
	c.mu.Unlock()

	return nil
}

func (c *ExposureCatalog) RequiresExposure(eventType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.types[eventType]
}

func (c *ExposureCatalog) Start(ctx context.Context) {
	if c.refreshInterval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(c.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.Reload(ctx); err != nil {
					c.log.Warn("attribution catalog reload failed", zap.Error(err))
				}
			}
		}
	}()
}
