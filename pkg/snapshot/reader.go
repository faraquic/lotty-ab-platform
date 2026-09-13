package snapshot

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

const watchInterval = 30 * time.Second

type Reader struct {
	r      *rueidis.Client
	flags  atomic.Pointer[map[string]FlagSnapshot]
	rev    atomic.Pointer[Revision]
	log    *zap.Logger
	done   chan struct{}
	notify chan struct{}
}

type SnapshotManager = Reader
type Provider = Reader

func NewReader(r *rueidis.Client, log *zap.Logger) *Reader {
	rr := &Reader{
		r:      r,
		log:    log.Named("snapshot_reader"),
		done:   make(chan struct{}),
		notify: make(chan struct{}, 1),
	}
	if s, err := rr.get(); err == nil {
		rr.flags.Store(snapshotToMap(s))
		rr.rev.Store(s.Revision)
	}
	go rr.subscribe()
	go rr.watch()
	return rr
}

func NewSnapshotManager(r *rueidis.Client, log *zap.Logger) (*SnapshotManager, error) {
	rr := NewReader(r, log)
	if rr.Current() == nil {
		if s, err := rr.get(); err != nil {
			return nil, err
		} else {
			rr.flags.Store(snapshotToMap(s))
			rr.rev.Store(s.Revision)
		}
	}
	return rr, nil
}

func NewProvider(r *rueidis.Client, log *zap.Logger) *Provider { return NewReader(r, log) }

func (rr *Reader) Current() *Snapshot {
	m := rr.flags.Load()
	rev := rr.rev.Load()
	if m == nil {
		return nil
	}
	flags := make([]FlagSnapshot, 0, len(*m))
	for _, v := range *m {
		flags = append(flags, v)
	}
	return &Snapshot{Revision: rev, Flags: flags}
}

func (rr *Reader) Flags() map[string]FlagSnapshot {
	m := rr.flags.Load()
	if m == nil {
		return nil
	}
	return *m
}

func (rr *Reader) GetFlag(key string) (FlagSnapshot, bool) {
	m := rr.flags.Load()
	if m == nil {
		return FlagSnapshot{}, false
	}
	v, ok := (*m)[key]
	return v, ok
}

func (rr *Reader) Stop() {
	select {
	case <-rr.done:
	default:
		close(rr.done)
	}
}

func (rr *Reader) subscribe() {
	for {
		select {
		case <-rr.done:
			return
		default:
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() { <-rr.done; cancel() }()
		err := (*rr.r).Receive(ctx, (*rr.r).B().Subscribe().Channel(database.ChannelUpdateSnapshot).Build(), func(m rueidis.PubSubMessage) {
			select {
			case rr.notify <- struct{}{}:
			default:
			}
		})
		cancel()
		if err != nil {
			select {
			case <-rr.done:
				return
			default:
				rr.log.Warn("pubsub retry", zap.Error(err))
				time.Sleep(time.Second)
			}
		}
	}
}

func (rr *Reader) watch() {
	t := time.NewTicker(watchInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_ = rr.checkUpdate()
		case <-rr.notify:
			_ = rr.checkUpdate()
		case <-rr.done:
			return
		}
	}
}

func (rr *Reader) checkUpdate() error {
	rev, err := rr.getRevision()
	if err != nil {
		return err
	}
	if cached := rr.rev.Load(); cached == nil || *cached != *rev {
		s, err := rr.get()
		if err != nil {
			return err
		}
		rr.flags.Store(snapshotToMap(s))
		rr.rev.Store(s.Revision)
		rr.log.Info("snapshot updated", zap.String(logger.FieldCacheKeyNS, keySnapshot))
	}
	return nil
}

func (rr *Reader) Get() (*Snapshot, error) { return rr.get() }
func (rr *Reader) GetRevision() (*Revision, error) { return rr.getRevision() }

func (rr *Reader) get() (*Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	b, err := (*rr.r).Do(ctx, (*rr.r).B().Get().Key(keySnapshot).Build()).AsBytes()
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (rr *Reader) getRevision() (*Revision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	b, err := (*rr.r).Do(ctx, (*rr.r).B().Get().Key(keySnapshotRev).Build()).AsBytes()
	if err != nil {
		return nil, err
	}
	var rev Revision
	copy(rev[:], b)
	return &rev, nil
}

func (rr *Reader) Exists(ctx context.Context) (bool, error) {
	return (*rr.r).Do(ctx, (*rr.r).B().Exists().Key(keySnapshot).Build()).AsBool()
}

func snapshotToMap(s *Snapshot) *map[string]FlagSnapshot {
	m := make(map[string]FlagSnapshot, len(s.Flags))
	for _, f := range s.Flags {
		m[f.Key] = f
	}
	return &m
}
