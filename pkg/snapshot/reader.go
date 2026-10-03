package snapshot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Provider is the read surface used by decide paths.
type Provider interface {
	Current() *Snapshot
	GetFlag(key string) (FlagSnapshot, bool)
	GetExperiment(flagKey string) (ExperimentSnapshot, bool)
	Refresh() error
	Stale(maxAge time.Duration) bool
	Age() time.Duration
	Stop()
}

// Reader keeps the latest snapshot in memory. It bootstraps from Redis and,
// when a kafka reader is provided, applies newer revisions consumed from the
// snapshot topic. A nil kafka reader disables the subscription.
//
// Kafka is a notification channel only, and it cannot be trusted to deliver
// everything: a fresh consumer group starts at the log end once the group is
// assigned, which happens seconds after the first read, so every snapshot
// published between the Redis bootstrap and that assignment is lost for good.
// Without a revalidation loop those revisions would only reach the process on
// the next publish, so the reader polls the cheap revision key in Redis and
// pulls the payload only when the revision actually moved.
type Reader struct {
	k          *kafka.Reader
	r          *rueidis.Client
	log        *zap.Logger
	seen       *outbox.Deduper
	flags      atomic.Pointer[map[string]FlagSnapshot]
	exps       atomic.Pointer[map[string]ExperimentSnapshot]
	rev        atomic.Uint64
	loaded     atomic.Int64
	revalidate time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// Option configures optional reader behaviour.
type Option func(*Reader)

// WithRevalidation enables the Redis revision poll at the given interval.
// A non-positive interval disables it.
func WithRevalidation(interval time.Duration) Option {
	return func(rr *Reader) {
		if interval > 0 {
			rr.revalidate = interval
		}
	}
}

func NewReader(k *kafka.Reader, r *rueidis.Client, log *zap.Logger, opts ...Option) *Reader {
	ctx, cancel := context.WithCancel(context.Background())

	rr := &Reader{
		k:      k,
		r:      r,
		log:    log.Named("snapshot_reader"),
		seen:   outbox.NewDeduper(0),
		ctx:    ctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		opt(rr)
	}

	if s, err := rr.loadFromRedis(ctx); err != nil {
		rr.log.Warn("snapshot redis bootstrap failed; waiting for kafka", zap.Error(err))
	} else {
		rr.store(s)
	}

	if k != nil {
		rr.wg.Add(1)
		go rr.subscribe()
	}

	if rr.revalidate > 0 {
		rr.wg.Add(1)
		go rr.pollRevalidate()
	}

	return rr
}

// Current rebuilds the latest snapshot, or nil when nothing was loaded yet.
func (rr *Reader) Current() *Snapshot {
	m := rr.flags.Load()
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

	return &Snapshot{Revision: Revision(rr.rev.Load()), Flags: flags, Experiments: exps}
}

func (rr *Reader) store(s *Snapshot) {
	rr.flags.Store(snapshotToMap(s))
	rr.exps.Store(experimentsToMap(s))
	rr.rev.Store(uint64(s.Revision))
	rr.loaded.Store(time.Now().UnixNano())
}

// Refresh reloads the snapshot from Redis when it holds a newer revision.
// It is the read-through path for memory misses: Redis is the source of
// truth, Kafka is the notification channel.
func (rr *Reader) Refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), OpTimeout)
	defer cancel()

	s, err := rr.loadFromRedis(ctx)
	if err != nil {
		return err
	}

	if !newerRevision(rr.rev.Load(), uint64(s.Revision)) {
		return nil
	}

	rr.store(s)
	rr.log.Debug(
		"snapshot refreshed from redis",
		zap.Uint64(logger.FieldSnapshotRevision, uint64(s.Revision)),
	)

	return nil
}

// newerRevision reports whether remote supersedes current. Revisions are
// UnixNano timestamps, so a strictly greater value is always a later snapshot.
func newerRevision(current, remote uint64) bool {
	return remote > current
}

// pollRevalidate compares the Redis revision key with the loaded revision and
// refreshes only when it moved. The revision key is a short decimal string, so
// an unchanged poll costs one small GET and no snapshot unmarshalling.
func (rr *Reader) pollRevalidate() {
	defer rr.wg.Done()

	ticker := time.NewTicker(rr.revalidate)
	defer ticker.Stop()

	for {
		select {
		case <-rr.ctx.Done():
			return
		case <-ticker.C:
			rr.revalidateOnce()
		}
	}
}

func (rr *Reader) revalidateOnce() {
	ctx, cancel := context.WithTimeout(rr.ctx, OpTimeout)
	defer cancel()

	remote, err := rr.loadRevision(ctx)
	if err != nil {
		if rr.ctx.Err() == nil {
			rr.log.Warn("snapshot revision poll failed", zap.Error(err))
		}

		return
	}

	if !newerRevision(rr.rev.Load(), remote) {
		return
	}

	if err := rr.Refresh(); err != nil {
		rr.log.Warn("snapshot revalidation failed", zap.Error(err))
	}
}

// loadRevision reads only the revision key written next to the snapshot.
func (rr *Reader) loadRevision(ctx context.Context) (uint64, error) {
	raw, err := (*rr.r).Do(ctx, (*rr.r).B().Get().Key(keySnapshotRev).Build()).ToString()
	if err != nil {
		return 0, err
	}

	rev, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse snapshot revision %q: %w", raw, err)
	}

	return rev, nil
}

// Age reports how long ago the current snapshot was loaded, or -1 when
// nothing was loaded yet.
func (rr *Reader) Age() time.Duration {
	ts := rr.loaded.Load()
	if ts == 0 {
		return -1
	}
	return time.Since(time.Unix(0, ts))
}

// Stale reports whether the loaded snapshot is older than maxAge, or no
// snapshot was loaded at all.
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

// Exists reports whether a snapshot is stored in Redis.
func (rr *Reader) Exists(ctx context.Context) (bool, error) {
	return (*rr.r).Do(ctx, (*rr.r).B().Exists().Key(keySnapshot).Build()).AsBool()
}

// Stop cancels the subscription, waits for it and closes the kafka reader.
func (rr *Reader) Stop() {
	rr.cancel()
	rr.wg.Wait()

	if rr.k != nil {
		_ = rr.k.Close()
	}
}

func (rr *Reader) loadFromRedis(ctx context.Context) (*Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, OpTimeout)
	defer cancel()

	raw, err := (*rr.r).Do(ctx, (*rr.r).B().Get().Key(keySnapshot).Build()).ToString()
	if err != nil {
		return nil, err
	}

	var s Snapshot
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}

	return &s, nil
}

func (rr *Reader) subscribe() {
	defer rr.wg.Done()

	for {
		msg, err := rr.k.ReadMessage(rr.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || rr.ctx.Err() != nil {
				return
			}
			rr.log.Warn("snapshot kafka read failed", zap.Error(err))
			continue
		}

		rr.applyRecord(msg.Value)
	}
}

func (rr *Reader) applyRecord(value []byte) bool {
	s, envelopeID, err := decodeSnapshotValue(value)
	if err != nil {
		rr.log.Warn("snapshot parse failed", zap.Error(err))
		return false
	}

	if envelopeID != "" && rr.seen.SeenOrMark(envelopeID) {
		rr.log.Debug(
			"snapshot duplicate skipped",
			zap.String(logger.FieldOutboxID, envelopeID),
			zap.Uint64(logger.FieldSnapshotRevision, uint64(s.Revision)),
		)

		return false
	}

	if !newerRevision(rr.rev.Load(), uint64(s.Revision)) {
		return false
	}

	rr.store(s)
	rr.log.Debug(
		"snapshot applied",
		zap.Uint64(logger.FieldSnapshotRevision, uint64(s.Revision)),
		zap.Int(logger.FieldSnapshotFlagCount, len(s.Flags)),
		zap.Int(logger.FieldSnapshotExperimentCount, len(s.Experiments)),
	)

	return true
}

func decodeSnapshotValue(value []byte) (*Snapshot, string, error) {
	var env outbox.Envelope

	if err := json.Unmarshal(value, &env); err == nil && strings.TrimSpace(env.ID) != "" {
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return nil, "", errors.New("snapshot envelope data is empty")
		}

		var s Snapshot

		if err := json.Unmarshal(env.Data, &s); err != nil {
			return nil, "", err
		}

		return &s, env.ID, nil
	}

	var s Snapshot

	if err := json.Unmarshal(value, &s); err != nil {
		return nil, "", err
	}

	return &s, "", nil
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
