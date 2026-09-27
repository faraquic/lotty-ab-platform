package snapshot

import (
	"testing"
	"time"

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
