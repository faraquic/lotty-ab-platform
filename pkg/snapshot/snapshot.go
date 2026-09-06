package snapshot

import (
	"context"
	"encoding/hex"
	"math/rand"
	"sync/atomic"
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
	redisOpTimeout = 1 * time.Second
	watchInterval  = 30 * time.Second
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type SnapshotStorage struct {
	r         *rueidis.Client
	getCmd    rueidis.Completed
	getRevCmd rueidis.Completed
	current   atomic.Pointer[Snapshot]
	rev       Revision
	log       *zap.Logger
	done      chan struct{}
}

type SnapshotManager = SnapshotStorage

func NewSnapshotStorage(r *rueidis.Client, log *zap.Logger) *SnapshotStorage {
	ss := &SnapshotStorage{
		r:         r,
		getCmd:    (*r).B().Get().Key(keySnapshot).Build(),
		getRevCmd: (*r).B().Get().Key(keySnapshotRev).Build(),
		log:       log.Named("snapshot"),
		done:      make(chan struct{}),
	}

	if s, err := ss.get(); err == nil {
		ss.current.Store(s)
		ss.rev = *s.Revision
	} else {
		ss.log.Warn(
			"initial snapshot load failed; starting with empty",
			zap.Error(err),
		)
	}

	go ss.watch()

	return ss
}

func NewSnapshotManager(r *rueidis.Client, log *zap.Logger) (*SnapshotManager, error) {
	ss := NewSnapshotStorage(r, log)
	if s := ss.Current(); s == nil {
		if s, err := ss.get(); err != nil {
			return nil, err
		} else {
			ss.current.Store(s)
			ss.rev = *s.Revision
		}
	}
	return ss, nil
}

func (ss *SnapshotStorage) Current() *Snapshot {
	return ss.current.Load()
}

func (ss *SnapshotStorage) RevisionValue() *Revision {
	return &ss.rev
}

func (ss *SnapshotStorage) Stop() {
	select {
	case <-ss.done:
		return
	default:
		close(ss.done)
	}
}

func (ss *SnapshotStorage) SetWithContext(ctx context.Context, s *Snapshot) error {
	if s.Revision == nil {
		s.Revision = generateRevision()
	}

	bytes, err := json.Marshal(s)
	if err != nil {
		ss.log.Error(
			"failed to marshal snapshot",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.String(logger.FieldCacheOperation, "set"),
			zap.String(logger.FieldErrorType, "internal_error"),
			zap.Error(err),
		)
		return err
	}

	b := (*ss.r).B()
	cmds := []rueidis.Completed{
		b.Set().Key(keySnapshot).Value(string(bytes)).Build(),
		b.Set().Key(keySnapshotRev).Value(string(s.Revision[:])).Build(),
		b.Publish().Channel(database.ChannelUpdateSnapshot).Message(string(s.Revision[:])).Build(),
	}

	res := (*ss.r).DoMulti(ctx, cmds...)
	for i, r := range res {
		if r.Error() != nil {
			if i == 2 {
				ss.log.Warn(
					"failed to publish snapshot update",
					zap.String(logger.FieldCacheSystem, "redis"),
					zap.String(logger.FieldCacheOperation, "publish"),
					zap.Error(r.Error()),
				)
				continue
			}
			ss.log.Error(
				"failed to set snapshot",
				zap.String(logger.FieldCacheSystem, "redis"),
				zap.String(logger.FieldCacheOperation, "set"),
				zap.String(logger.FieldCacheKeyNS, keySnapshot),
				zap.Error(r.Error()),
			)
			return r.Error()
		}
	}

	ss.current.Store(s)
	ss.rev = *s.Revision

	ss.log.Debug(
		"snapshot stored",
		zap.String(logger.FieldCacheSystem, "redis"),
		zap.String(logger.FieldCacheOperation, "set"),
		zap.String(logger.FieldCacheKeyNS, keySnapshot),
	)
	return nil
}

func (ss *SnapshotStorage) watch() {
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ss.checkUpdate(); err != nil {
				ss.log.Warn(
					"snapshot check update failed",
					zap.Error(err),
				)
			}
		case <-ss.done:
			return
		}
	}
}

func (ss *SnapshotStorage) checkUpdate() error {
	rev, err := ss.getRevision()
	if err != nil {
		return err
	}

	if ss.rev != *rev {
		s, err := ss.get()
		if err != nil {
			return err
		}
		ss.current.Store(s)
		ss.rev = *s.Revision
		ss.log.Info(
			"snapshot updated from redis",
			zap.String(logger.FieldCacheOperation, "get"),
			zap.String(logger.FieldCacheKeyNS, keySnapshot),
		)
	}
	return nil
}

func (ss *SnapshotStorage) get() (*Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	bytes, err := (*ss.r).Do(ctx, ss.getCmd).AsBytes()
	if err != nil {
		ss.log.Error(
			"failed to get snapshot",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.String(logger.FieldCacheOperation, "get"),
			zap.String(logger.FieldCacheKeyNS, keySnapshot),
			zap.Bool(logger.FieldCacheHit, false),
			zap.Error(err),
		)
		return nil, err
	}

	var s Snapshot
	if err := json.Unmarshal(bytes, &s); err != nil {
		ss.log.Error(
			"failed to unmarshal snapshot",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.String(logger.FieldCacheOperation, "get"),
			zap.String(logger.FieldCacheKeyNS, keySnapshot),
			zap.String(logger.FieldErrorType, "internal_error"),
			zap.Error(err),
		)
		return nil, err
	}

	ss.log.Debug(
		"snapshot retrieved",
		zap.String(logger.FieldCacheSystem, "redis"),
		zap.String(logger.FieldCacheOperation, "get"),
		zap.String(logger.FieldCacheKeyNS, keySnapshot),
		zap.Bool(logger.FieldCacheHit, true),
	)
	return &s, nil
}

func (ss *SnapshotStorage) getRevision() (*Revision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	bytes, err := (*ss.r).Do(ctx, ss.getRevCmd).AsBytes()
	if err != nil {
		ss.log.Error(
			"failed to get snapshot revision",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.String(logger.FieldCacheOperation, "get"),
			zap.String(logger.FieldCacheKeyNS, keySnapshotRev),
			zap.Bool(logger.FieldCacheHit, false),
			zap.Error(err),
		)
		return nil, err
	}

	var rev Revision
	copy(rev[:], bytes)

	ss.log.Debug(
		"snapshot revision retrieved",
		zap.String(logger.FieldCacheSystem, "redis"),
		zap.String(logger.FieldCacheOperation, "get"),
		zap.String(logger.FieldCacheKeyNS, keySnapshotRev),
		zap.Bool(logger.FieldCacheHit, true),
	)
	return &rev, nil
}

func (ss *SnapshotStorage) Exists(ctx context.Context) (bool, error) {
	exists, err := (*ss.r).Do(ctx, (*ss.r).B().Exists().Key(keySnapshot).Build()).AsBool()
	if err != nil {
		return false, err
	}
	return exists, nil
}

func generateRevision() *Revision {
	var rev [7]byte
	var dst Revision
	_, _ = rng.Read(rev[:])
	hex.Encode(dst[:], rev[:])
	return &dst
}
