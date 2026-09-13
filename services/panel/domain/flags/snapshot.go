package flags

import (
	"context"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"go.uber.org/zap"
)

const (
	refreshBufferSize = 1000
	refreshOpTimeout  = 2 * time.Second
)

type SnapshotRefresher interface {
	Refresh()
	RefreshSync(ctx context.Context) error
	Stop()
}

type Refresher struct {
	repo    FlagRepo
	storage *snapshot.Writer
	ch      chan struct{}
	log     *zap.Logger
	done    chan struct{}
}

func NewRefresher(repo FlagRepo, storage *snapshot.Writer, log *zap.Logger) *Refresher {
	r := &Refresher{
		repo:    repo,
		storage: storage,
		ch:      make(chan struct{}, refreshBufferSize),
		log:     log.Named("snapshot_refresher"),
		done:    make(chan struct{}),
	}
	go r.worker()
	return r
}

func (r *Refresher) Refresh() {
	select {
	case r.ch <- struct{}{}:
	default:
		r.log.Warn("snapshot refresh buffer full, dropping request",
			zap.Int(logger.FieldQueueLen, len(r.ch)),
			zap.Int(logger.FieldQueueCap, cap(r.ch)),
		)
	}
}

func (r *Refresher) RefreshSync(ctx context.Context) error {
	return r.doRefresh(ctx)
}

func (r *Refresher) Stop() {
	close(r.ch)
	<-r.done
}

func (r *Refresher) worker() {
	defer close(r.done)

	for range r.ch {
		ctx, cancel := context.WithTimeout(context.Background(), refreshOpTimeout)
		if err := r.doRefresh(ctx); err != nil {
			r.log.Warn("snapshot refresh failed",
				zap.String(logger.FieldCacheOperation, "set"),
				zap.Error(err),
			)
		}
		cancel()
	}
}

func (r *Refresher) doRefresh(ctx context.Context) error {
	start := time.Now()

	flags, err := r.repo.List(ctx, 10000, 0)
	if err != nil {
		r.log.Error("failed to list flags for snapshot",
			zap.String(logger.FieldCacheOperation, "set"),
			zap.Error(err),
		)
		return err
	}

	inputs := make([]snapshot.FlagInput, 0, len(flags))
	for _, f := range flags {
		inputs = append(inputs, snapshot.FlagInput{
			Key:   f.Key,
			Type:  string(f.Type),
			Value: []byte(f.DefaultValue),
		})
	}

	snap := snapshot.Build(inputs)

	if err := r.storage.SetWithContext(ctx, snap); err != nil {
		return err
	}

	r.log.Info("snapshot refreshed",
		zap.String(logger.FieldCacheOperation, "set"),
		zap.Int(logger.FieldSnapshotFlagCount, len(flags)),
		zap.Float64(logger.FieldDurationMs, float64(time.Since(start).Nanoseconds())/1e6),
	)
	return nil
}
