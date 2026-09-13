package snapshot

import (
	"context"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

const (
	keySnapshot    = "labp:runtime:snapshot"
	keySnapshotRev = "labp:runtime:snapshot:rev"
	redisOpTimeout = 5 * time.Second
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type Writer struct {
	r   *rueidis.Client
	log *zap.Logger
}

func NewWriter(r *rueidis.Client, log *zap.Logger) *Writer {
	return &Writer{r: r, log: log.Named("snapshot_writer")}
}

func (w *Writer) Set(s *Snapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	return w.SetWithContext(ctx, s)
}

func (w *Writer) SetWithContext(ctx context.Context, s *Snapshot) error {
	if s.Revision == nil {
		s.Revision = generateRevision()
	}
	b, err := json.Marshal(s)
	if err != nil {
		w.log.Error("failed to marshal snapshot", zap.String(logger.FieldErrorType, string(logger.ErrorTypeInternalError)), zap.Error(err))
		return err
	}
	cmds := []rueidis.Completed{
		(*w.r).B().Set().Key(keySnapshot).Value(string(b)).Build(),
		(*w.r).B().Set().Key(keySnapshotRev).Value(string(s.Revision[:])).Build(),
		(*w.r).B().Publish().Channel(database.ChannelUpdateSnapshot).Message(string(s.Revision[:])).Build(),
	}
	for i, r := range (*w.r).DoMulti(ctx, cmds...) {
		if err := r.Error(); err != nil {
			if i == 2 {
				w.log.Warn("publish failed", zap.Error(err))
				continue
			}
			w.log.Error("failed to set snapshot", zap.Error(err))
			return err
		}
	}
	w.log.Debug("snapshot stored", zap.String(logger.FieldCacheKeyNS, keySnapshot))
	return nil
}

func generateRevision() *Revision {
	var r [7]byte
	var d Revision
	_, _ = rng.Read(r[:])
	hex.Encode(d[:], r[:])
	return &d
}
