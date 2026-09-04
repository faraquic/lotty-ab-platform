package snapshot

import (
	"context"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

const (
	keySnapshot    = "labp:runtime:snapshot"
	keySnapshotRev = "labp:runtime:snapshot:rev"
	redisOpTimeout = 1 * time.Second
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type Snapshot struct {
	Revision *[14]byte      `json:"r"`
	Flags    []FlagSnapshot `json:"f"`
}

type FlagSnapshot struct {
	Key   string `json:"k"`
	Type  string `json:"t"`
	Value []byte `json:"v"`
}

type SnapshotStorage struct {
	r         *rueidis.Client
	getCmd    rueidis.Completed
	getRevCmd rueidis.Completed
	log       *zap.Logger
}

func NewSnapshotStorage(r *rueidis.Client, log *zap.Logger) *SnapshotStorage {
	return &SnapshotStorage{
		r:         r,
		getCmd:    (*r).B().Get().Key(keySnapshot).Build(),
		getRevCmd: (*r).B().Get().Key(keySnapshotRev).Build(),
		log:       log.Named("snapshot"),
	}
}

func (ss *SnapshotStorage) Get() (*Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	bytes, err := (*ss.r).Do(ctx, ss.getCmd).AsBytes()
	if err != nil {
		ss.log.Error("failed to get snapshot",
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
		ss.log.Error("failed to unmarshal snapshot",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.String(logger.FieldCacheOperation, "get"),
			zap.String(logger.FieldCacheKeyNS, keySnapshot),
			zap.String(logger.FieldErrorType, "internal_error"),
			zap.Error(err),
		)
		return nil, err
	}

	ss.log.Debug("snapshot retrieved",
		zap.String(logger.FieldCacheSystem, "redis"),
		zap.String(logger.FieldCacheOperation, "get"),
		zap.String(logger.FieldCacheKeyNS, keySnapshot),
		zap.Bool(logger.FieldCacheHit, true),
	)
	return &s, nil
}

func (ss *SnapshotStorage) Set(s *Snapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	return ss.SetWithContext(ctx, s)
}

func (ss *SnapshotStorage) SetWithContext(ctx context.Context, s *Snapshot) error {
	if s.Revision == nil {
		s.Revision = generateRevision()
	}

	bytes, err := json.Marshal(s)
	if err != nil {
		ss.log.Error("failed to marshal snapshot",
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
	}

	res := (*ss.r).DoMulti(ctx, cmds...)
	for _, r := range res {
		if r.Error() != nil {
			ss.log.Error("failed to set snapshot",
				zap.String(logger.FieldCacheSystem, "redis"),
				zap.String(logger.FieldCacheOperation, "set"),
				zap.String(logger.FieldCacheKeyNS, keySnapshot),
				zap.Error(r.Error()),
			)
			return r.Error()
		}
	}

	ss.log.Debug("snapshot stored",
		zap.String(logger.FieldCacheSystem, "redis"),
		zap.String(logger.FieldCacheOperation, "set"),
		zap.String(logger.FieldCacheKeyNS, keySnapshot),
	)
	return nil
}

func (ss *SnapshotStorage) GetRevision() (*[14]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	bytes, err := (*ss.r).Do(ctx, ss.getRevCmd).AsBytes()
	if err != nil {
		ss.log.Error("failed to get snapshot revision",
			zap.String(logger.FieldCacheSystem, "redis"),
			zap.String(logger.FieldCacheOperation, "get"),
			zap.String(logger.FieldCacheKeyNS, keySnapshotRev),
			zap.Bool(logger.FieldCacheHit, false),
			zap.Error(err),
		)
		return nil, err
	}

	var rev [14]byte
	copy(rev[:], bytes)

	ss.log.Debug("snapshot revision retrieved",
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

func generateRevision() *[14]byte {
	var rev [7]byte
	_, _ = rng.Read(rev[:])
	var dst [14]byte
	hex.Encode(dst[:], rev[:])
	return &dst
}
