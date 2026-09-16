package decide

import (
	"errors"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"go.uber.org/zap"
)

var ErrSnapshotUnavailable = errors.New("snapshot unavailable")
var ErrUnknownFlag = errors.New("unknown flag")

type snapshotProvider interface {
	Current() *snapshot.Snapshot
	GetFlag(key string) (snapshot.FlagSnapshot, bool)
	GetExperiment(flagKey string) (snapshot.ExperimentSnapshot, bool)
	Stale(maxAge time.Duration) bool
}

type Repository struct {
	reader snapshotProvider
	log    *zap.Logger
}

func NewRepository(reader *snapshot.Reader, log *zap.Logger) *Repository {
	return &Repository{reader: reader, log: log.Named("decide_repo")}
}

func (r *Repository) GetSnapshot() (*snapshot.Snapshot, error) {
	snap := r.reader.Current()
	if snap == nil {
		return nil, ErrSnapshotUnavailable
	}
	return snap, nil
}

func (r *Repository) FindFlag(key string) (*snapshot.FlagSnapshot, bool) {
	f, ok := r.reader.GetFlag(key)
	if !ok {
		return nil, false
	}
	return &f, true
}

func (r *Repository) FindExperiment(flagKey string) (*snapshot.ExperimentSnapshot, bool) {
	e, ok := r.reader.GetExperiment(flagKey)
	if !ok {
		return nil, false
	}
	return &e, true
}

func (r *Repository) Stale(maxAge time.Duration) bool {
	return r.reader.Stale(maxAge)
}

func (r *Repository) Flags() map[string]snapshot.FlagSnapshot {
	snap := r.reader.Current()
	if snap == nil {
		return nil
	}
	m := make(map[string]snapshot.FlagSnapshot, len(snap.Flags))
	for _, f := range snap.Flags {
		m[f.Key] = f
	}
	return m
}
