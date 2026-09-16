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
	exps   atomic.Pointer[map[string]ExperimentSnapshot]
	rev    atomic.Pointer[Revision]
	loaded atomic.Int64
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
		rr.store(s)
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
			rr.store(s)
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
	var exps []ExperimentSnapshot
	if em := rr.exps.Load(); em != nil {
		exps = make([]ExperimentSnapshot, 0, len(*em))
		for _, v := range *em {
			exps = append(exps, v)
		}
	}
	return &Snapshot{Revision: rev, Flags: flags, Experiments: exps}
}

func (rr *Reader) store(s *Snapshot) {
	rr.flags.Store(snapshotToMap(s))
	rr.exps.Store(experimentsToMap(s))
	rr.rev.Store(s.Revision)
	rr.loaded.Store(time.Now().UnixNano())
}

func (rr *Reader) Age() time.Duration {
	ts := rr.loaded.Load()
	if ts == 0 {
		return -1
	}
	return time.Since(time.Unix(0, ts))
}

func (rr *Reader) Stale(maxAge time.Duration) bool {
	age := rr.Age()
	return age < 0 || age > maxAge
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

func (rr *Reader) GetExperiment(flagKey string) (ExperimentSnapshot, bool) {
	m := rr.exps.Load()
	if m == nil {
		return ExperimentSnapshot{}, false
	}
	v, ok := (*m)[flagKey]
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
		rr.store(s)
		rr.log.Info("snapshot updated", zap.String(logger.FieldCacheKeyNS, keySnapshot))
	}
	return nil
}

func (rr *Reader) Get() (*Snapshot, error)         { return rr.get() }
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

func experimentsToMap(s *Snapshot) *map[string]ExperimentSnapshot {
	m := make(map[string]ExperimentSnapshot, len(s.Experiments))
	for _, e := range s.Experiments {
		m[e.FlagKey] = e
	}
	return &m
}
