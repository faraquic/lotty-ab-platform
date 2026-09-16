package snapshot

import (
	"context"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	pkgsnapshot "github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"go.uber.org/zap"
)

const (
	refreshBufferSize = 1000
	refreshOpTimeout  = 2 * time.Second
)

type Refresher interface {
	Refresh()
	RefreshSync(ctx context.Context) error
	Stop()
}

type Source interface {
	ListFlags(ctx context.Context) ([]pkgsnapshot.FlagInput, error)
	ListExperiments(ctx context.Context) ([]pkgsnapshot.ExperimentInput, error)
}

type RefresherImpl struct {
	src     Source
	storage *pkgsnapshot.Writer
	ch      chan struct{}
	log     *zap.Logger
	done    chan struct{}
}

func NewRefresher(src Source, storage *pkgsnapshot.Writer, log *zap.Logger) *RefresherImpl {
	r := &RefresherImpl{
		src:     src,
		storage: storage,
		ch:      make(chan struct{}, refreshBufferSize),
		log:     log.Named("snapshot_refresher"),
		done:    make(chan struct{}),
	}
	go r.worker()
	return r
}

func (r *RefresherImpl) Refresh() {
	select {
	case r.ch <- struct{}{}:
	default:
		r.log.Warn("snapshot refresh buffer full, dropping request",
			zap.Int(logger.FieldQueueLen, len(r.ch)),
			zap.Int(logger.FieldQueueCap, cap(r.ch)),
		)
	}
}

func (r *RefresherImpl) RefreshSync(ctx context.Context) error {
	return r.doRefresh(ctx)
}

func (r *RefresherImpl) Stop() {
	close(r.ch)
	<-r.done
}

func (r *RefresherImpl) worker() {
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

func (r *RefresherImpl) doRefresh(ctx context.Context) error {
	start := time.Now()

	flags, err := r.src.ListFlags(ctx)
	if err != nil {
		r.log.Error("failed to list flags for snapshot",
			zap.String(logger.FieldCacheOperation, "set"),
			zap.Error(err),
		)
		return err
	}

	experiments, err := r.src.ListExperiments(ctx)
	if err != nil {
		r.log.Error("failed to list experiments for snapshot",
			zap.String(logger.FieldCacheOperation, "set"),
			zap.Error(err),
		)
		return err
	}

	snap := pkgsnapshot.Build(flags, experiments)

	if err := r.storage.SetWithContext(ctx, snap); err != nil {
		return err
	}

	r.log.Info("snapshot refreshed",
		zap.String(logger.FieldCacheOperation, "set"),
		zap.Int(logger.FieldSnapshotFlagCount, len(flags)),
		zap.Int(logger.FieldSnapshotExperimentCount, len(experiments)),
		zap.Float64(logger.FieldDurationMs, float64(time.Since(start).Nanoseconds())/1e6),
	)
	return nil
}
