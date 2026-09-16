package decide

import (
	"testing"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type fakeProvider struct {
	snap  *snapshot.Snapshot
	stale bool
}

func (f fakeProvider) Current() *snapshot.Snapshot { return f.snap }
func (f fakeProvider) Stale(time.Duration) bool    { return f.stale }
func (f fakeProvider) GetFlag(key string) (snapshot.FlagSnapshot, bool) {
	for _, fl := range f.snap.Flags {
		if fl.Key == key {
			return fl, true
		}
	}
	return snapshot.FlagSnapshot{}, false
}
func (f fakeProvider) GetExperiment(flagKey string) (snapshot.ExperimentSnapshot, bool) {
	for _, e := range f.snap.Experiments {
		if e.FlagKey == flagKey {
			return e, true
		}
	}
	return snapshot.ExperimentSnapshot{}, false
}

func testSnapshot() *snapshot.Snapshot {
	return &snapshot.Snapshot{
		Flags: []snapshot.FlagSnapshot{
			{Key: "checkout_redesign", Type: "string", Value: []byte(`"off"`)},
			{Key: "plain_flag", Type: "string", Value: []byte(`"default"`)},
		},
		Experiments: []snapshot.ExperimentSnapshot{
			{
				ID:           "exp-1",
				FlagKey:      "checkout_redesign",
				VersionNum:   3,
				Salt:         "salty",
				AllocationBp: 10000,
				Variants: []snapshot.VariantSnapshot{
					{ID: "var-c", Name: "control", Value: []byte(`"off"`), WeightBp: 5000},
					{ID: "var-t", Name: "treatment", Value: []byte(`"on"`), WeightBp: 5000},
				},
			},
		},
	}
}

func newTestService(snap *snapshot.Snapshot, stale bool) *Service {
	repo := &Repository{reader: fakeProvider{snap: snap, stale: stale}, log: zap.NewNop()}
	return NewService(repo, time.Minute, zap.NewNop())
}

func TestDecideExperimentHit(t *testing.T) {
	svc := newTestService(testSnapshot(), false)
	resp, err := svc.Decide(CreateDecisionRequest{SubjectID: "user-1", Flags: []string{"checkout_redesign"}}, "req-1")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if resp.Degraded {
		t.Error("should not be degraded")
	}
	d := resp.Flags["checkout_redesign"]
	if d.Source != SourceExperiment {
		t.Fatalf("source: got %q, want experiment (reason: %s)", d.Source, d.Reason)
	}
	if d.ExperimentID != "exp-1" || d.ExperimentVersion != 3 {
		t.Errorf("experiment ref: %+v", d)
	}
	if d.VariantID != "var-c" && d.VariantID != "var-t" {
		t.Errorf("unexpected variant: %q", d.VariantID)
	}
	if d.DecisionID == "" {
		t.Error("decision_id should be set")
	}
}

func TestDecideStickiness(t *testing.T) {
	svc := newTestService(testSnapshot(), false)
	req := CreateDecisionRequest{SubjectID: "sticky-user", Flags: []string{"checkout_redesign"}}
	first, err := svc.Decide(req, "req-1")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	second, err := svc.Decide(req, "req-2")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	a, b := first.Flags["checkout_redesign"], second.Flags["checkout_redesign"]
	if a.VariantID != b.VariantID || a.Value != b.Value {
		t.Errorf("not sticky: %+v vs %+v", a, b)
	}
	if a.DecisionID == b.DecisionID {
		t.Error("decision_id should be unique per decision")
	}
}

func TestDecideDefault(t *testing.T) {
	svc := newTestService(testSnapshot(), false)
	resp, err := svc.Decide(CreateDecisionRequest{SubjectID: "u", Flags: []string{"plain_flag"}}, "req-1")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	plain := resp.Flags["plain_flag"]
	if plain.Source != SourceDefault || plain.Value != "default" {
		t.Errorf("plain flag: %+v", plain)
	}
	if _, err := uuid.Parse(plain.DecisionID); err != nil {
		t.Errorf("default decision_id should be a uuid: %q", plain.DecisionID)
	}
}

func TestDecideUnknownFlag(t *testing.T) {
	svc := newTestService(testSnapshot(), false)
	_, err := svc.Decide(CreateDecisionRequest{SubjectID: "u", Flags: []string{"plain_flag", "nope_flag"}}, "req-1")
	if err != ErrUnknownFlag {
		t.Errorf("expected ErrUnknownFlag, got %v", err)
	}
}

func TestDecideDegraded(t *testing.T) {
	svc := newTestService(testSnapshot(), true)
	resp, err := svc.Decide(CreateDecisionRequest{SubjectID: "u", Flags: []string{"plain_flag"}}, "req-1")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if !resp.Degraded {
		t.Error("should be degraded")
	}
	if resp.Flags["plain_flag"].Source != SourceDefault {
		t.Errorf("stale snapshot should still serve: %+v", resp.Flags["plain_flag"])
	}
}

func TestDecideNoSnapshot(t *testing.T) {
	repo := &Repository{reader: fakeProvider{}, log: zap.NewNop()}
	svc := NewService(repo, time.Minute, zap.NewNop())
	if _, err := svc.Decide(CreateDecisionRequest{SubjectID: "u", Flags: []string{"f"}}, "req-1"); err != ErrSnapshotUnavailable {
		t.Errorf("expected ErrSnapshotUnavailable, got %v", err)
	}
}
