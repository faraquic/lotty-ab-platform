package decide

import (
	"errors"

	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"go.uber.org/zap"
)

var ErrSnapshotUnavailable = errors.New("snapshot unavailable")

type Repository struct {
	reader *snapshot.Reader
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

func (r *Repository) Flags() map[string]snapshot.FlagSnapshot {
	return r.reader.Flags()
}
