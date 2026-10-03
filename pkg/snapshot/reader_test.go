package snapshot

import (
	"context"
	"testing"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

func testReader() *Reader {
	return &Reader{log: zap.NewNop()}
}

func testStoredSnapshot(rev Revision) *Snapshot {
	return &Snapshot{
		Revision: rev,
		Flags: []FlagSnapshot{
			{Key: "b", Type: "string", Value: []byte(`"2"`)},
			{Key: "a", Type: "string", Value: []byte(`"1"`)},
		},
		Experiments: []ExperimentSnapshot{
			{ID: "e1", FlagKey: "a", VersionNum: 1, Salt: "s", AllocationBp: 10000},
		},
	}
}

func TestReaderEmpty(t *testing.T) {
	rr := testReader()

	if got := rr.Current(); got != nil {
		t.Errorf("Current = %+v, want nil", got)
	}

	if _, ok := rr.GetFlag("a"); ok {
		t.Error("GetFlag on empty reader should miss")
	}

	if _, ok := rr.GetExperiment("a"); ok {
		t.Error("GetExperiment on empty reader should miss")
	}

	if age := rr.Age(); age != -1 {
		t.Errorf("Age = %v, want -1", age)
	}

	if !rr.Stale(time.Minute) {
		t.Error("empty reader should be stale")
	}
}

func TestReaderStoreCurrent(t *testing.T) {
	rr := testReader()
	rr.store(testStoredSnapshot(42))

	cur := rr.Current()
	if cur == nil {
		t.Fatal("Current = nil, want snapshot")
	}

	if cur.Revision != 42 {
		t.Errorf("Revision = %d, want 42", cur.Revision)
	}

	if len(cur.Flags) != 2 {
		t.Fatalf("Flags = %d, want 2", len(cur.Flags))
	}

	f, ok := rr.GetFlag("a")
	if !ok || string(f.Value) != `"1"` {
		t.Errorf("GetFlag(a) = %+v, %v", f, ok)
	}

	e, ok := rr.GetExperiment("a")
	if !ok || e.ID != "e1" {
		t.Errorf("GetExperiment(a) = %+v, %v", e, ok)
	}

	if age := rr.Age(); age < 0 {
		t.Errorf("Age = %v, want >= 0", age)
	}

	if rr.Stale(time.Minute) {
		t.Error("fresh reader should not be stale")
	}
}

func TestReaderStaleAfterAge(t *testing.T) {
	rr := testReader()
	rr.store(testStoredSnapshot(7))
	rr.loaded.Store(time.Now().Add(-time.Hour).UnixNano())

	if !rr.Stale(time.Minute) {
		t.Error("hour-old snapshot should be stale")
	}
}

func marshalSnapshotPayload(t *testing.T, s *Snapshot) []byte {
	t.Helper()

	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	return raw
}

func marshalEnvelopePayload(t *testing.T, id string, s *Snapshot) []byte {
	t.Helper()

	raw, err := json.Marshal(outbox.Envelope{
		ID:     id,
		Type:   "snapshot",
		Source: "labp-panel",
		Time:   time.Now().UTC(),
		Data:   marshalSnapshotPayload(t, s),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	return raw
}

func TestApplyRecordNewerApplies(t *testing.T) {
	rr := testReader()

	if !rr.applyRecord(marshalSnapshotPayload(t, testStoredSnapshot(10))) {
		t.Fatal("applyRecord(newer) = false, want true")
	}

	if got := Revision(rr.rev.Load()); got != 10 {
		t.Errorf("rev = %d, want 10", got)
	}
}

func TestApplyRecordDuplicateRawIgnored(t *testing.T) {
	rr := testReader()
	raw := marshalSnapshotPayload(t, testStoredSnapshot(10))

	if !rr.applyRecord(raw) {
		t.Fatal("first applyRecord = false, want true")
	}

	loaded := rr.loaded.Load()

	if rr.applyRecord(raw) {
		t.Error("second applyRecord(duplicate) = true, want false")
	}

	if got := Revision(rr.rev.Load()); got != 10 {
		t.Errorf("rev = %d, want 10", got)
	}

	if f, ok := rr.GetFlag("a"); !ok || string(f.Value) != `"1"` {
		t.Errorf("GetFlag(a) = %+v, %v after duplicate", f, ok)
	}

	if rr.loaded.Load() != loaded {
		t.Error("duplicate delivery rewrote loaded timestamp")
	}
}

func TestApplyRecordOlderIgnored(t *testing.T) {
	rr := testReader()
	rr.store(testStoredSnapshot(10))

	if rr.applyRecord(marshalSnapshotPayload(t, testStoredSnapshot(9))) {
		t.Error("applyRecord(older) = true, want false")
	}

	if got := Revision(rr.rev.Load()); got != 10 {
		t.Errorf("rev = %d, want 10", got)
	}
}

func TestApplyRecordEnvelopeDedup(t *testing.T) {
	rr := testReader()
	rr.seen = outbox.NewDeduper(0)

	raw := marshalEnvelopePayload(t, "row-1", testStoredSnapshot(11))

	if !rr.applyRecord(raw) {
		t.Fatal("first applyRecord(envelope) = false, want true")
	}

	if rr.applyRecord(raw) {
		t.Error("second applyRecord(same envelope id) = true, want false")
	}

	if got := Revision(rr.rev.Load()); got != 11 {
		t.Errorf("rev = %d, want 11", got)
	}

	older := marshalEnvelopePayload(t, "row-2", testStoredSnapshot(10))
	if rr.applyRecord(older) {
		t.Error("applyRecord(envelope, older revision) = true, want false")
	}

	newer := marshalEnvelopePayload(t, "row-3", testStoredSnapshot(12))
	if !rr.applyRecord(newer) {
		t.Error("applyRecord(envelope, newer revision) = false, want true")
	}
}

func TestApplyRecordMalformedSkipped(t *testing.T) {
	rr := testReader()
	rr.store(testStoredSnapshot(10))

	for _, raw := range []string{
		"{not json",
		"",
		`{"id":"row-9","type":"snapshot","data":"not-a-snapshot"}`,
		`{"id":"row-10","type":"snapshot"}`,
	} {
		if rr.applyRecord([]byte(raw)) {
			t.Errorf("applyRecord(%q) = true, want false", raw)
		}
	}

	if got := Revision(rr.rev.Load()); got != 10 {
		t.Errorf("rev = %d, want 10", got)
	}
}

func TestNewerRevision(t *testing.T) {
	tests := []struct {
		name    string
		current uint64
		remote  uint64
		want    bool
	}{
		{name: "newer applies", current: 10, remote: 11, want: true},
		{name: "same is no-op", current: 10, remote: 10, want: false},
		{name: "older never rewinds", current: 10, remote: 9, want: false},
		{name: "nothing loaded yet", current: 0, remote: 1, want: true},
		{name: "empty redis keeps state", current: 7, remote: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newerRevision(tt.current, tt.remote); got != tt.want {
				t.Errorf("newerRevision(%d, %d) = %v, want %v", tt.current, tt.remote, got, tt.want)
			}
		})
	}
}

func TestWithRevalidationInterval(t *testing.T) {
	rr := &Reader{log: zap.NewNop()}
	if rr.revalidate != 0 {
		t.Fatalf("revalidate = %v, want 0 by default", rr.revalidate)
	}

	WithRevalidation(3 * time.Second)(rr)
	if rr.revalidate != 3*time.Second {
		t.Errorf("revalidate = %v, want 3s", rr.revalidate)
	}

	WithRevalidation(0)(rr)
	if rr.revalidate != 3*time.Second {
		t.Errorf("revalidate = %v, want zero interval to be ignored", rr.revalidate)
	}

	WithRevalidation(-time.Second)(rr)
	if rr.revalidate != 3*time.Second {
		t.Errorf("revalidate = %v, want negative interval to be ignored", rr.revalidate)
	}
}

func TestPollRevalidateStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	rr := &Reader{
		log:        zap.NewNop(),
		seen:       outbox.NewDeduper(0),
		revalidate: time.Hour,
		ctx:        ctx,
		cancel:     cancel,
	}

	rr.wg.Add(1)
	go rr.pollRevalidate()
	cancel()

	done := make(chan struct{})
	go func() {
		rr.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pollRevalidate did not return after context cancellation")
	}
}
