package decide

import (
	"testing"

	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
)

func TestBucketPosVectors(t *testing.T) {
	vectors := map[string]int{
		"user-1": 1544,
		"user-2": 3112,
		"user-3": 3179,
		"alice":  1425,
		"bob":    2285,
	}
	for subject, want := range vectors {
		if got := bucketPos("exp-123", "deadbeef", subject); got != want {
			t.Errorf("bucketPos(exp-123, deadbeef, %s): got %d, want %d", subject, got, want)
		}
	}
}

func TestBucketPosRange(t *testing.T) {
	seen := make(map[int]struct{})
	for i := 0; i < 1000; i++ {
		pos := bucketPos("exp-range", "salt", string(rune('a'+i%26))+string(rune('0'+i%10))+string(rune(i)))
		if pos < 0 || pos >= basisPoints {
			t.Fatalf("pos out of range: %d", pos)
		}
		seen[pos] = struct{}{}
	}
	if len(seen) < 500 {
		t.Errorf("poor distribution: %d distinct buckets of 1000", len(seen))
	}
}

func TestBucketPosStickiness(t *testing.T) {
	first := bucketPos("exp-1", "s1", "subject-42")
	for i := 0; i < 10; i++ {
		if got := bucketPos("exp-1", "s1", "subject-42"); got != first {
			t.Fatalf("not sticky: got %d, want %d", got, first)
		}
	}
	if other := bucketPos("exp-1", "s2", "subject-42"); other == first {
		t.Logf("note: same bucket after salt change (possible collision): %d", other)
	}
}

func TestSelectVariant(t *testing.T) {
	exp := snapshot.ExperimentSnapshot{
		AllocationBp: 10000,
		Variants: []snapshot.VariantSnapshot{
			{ID: "c", Name: "control", WeightBp: 5000},
			{ID: "t", Name: "treatment", WeightBp: 5000},
		},
	}

	cases := []struct {
		pos    int
		wantID string
		wantOK bool
	}{
		{0, "c", true},
		{4999, "c", true},
		{5000, "t", true},
		{9999, "t", true},
		{10000, "", false},
		{-1, "", false},
	}
	for _, tc := range cases {
		v, ok := selectVariant(exp, tc.pos)
		if ok != tc.wantOK {
			t.Errorf("pos %d: ok=%v, want %v", tc.pos, ok, tc.wantOK)
			continue
		}
		if ok && v.ID != tc.wantID {
			t.Errorf("pos %d: got %q, want %q", tc.pos, v.ID, tc.wantID)
		}
	}
}

func TestSelectVariantPartialAllocation(t *testing.T) {
	exp := snapshot.ExperimentSnapshot{
		AllocationBp: 1000,
		Variants: []snapshot.VariantSnapshot{
			{ID: "c", Name: "control", WeightBp: 500},
			{ID: "t", Name: "treatment", WeightBp: 500},
		},
	}
	if _, ok := selectVariant(exp, 1000); ok {
		t.Error("pos == allocation should be excluded")
	}
	if v, ok := selectVariant(exp, 999); !ok || v.ID != "t" {
		t.Errorf("pos 999: got %+v, %v", v, ok)
	}
}

func TestTargetingMatch(t *testing.T) {
	for _, ok := range [][]byte{nil, {}, []byte("  "), []byte("null"), []byte("{}"), []byte("  {}  ")} {
		if !targetingMatch(ok) {
			t.Errorf("%q: expected match", ok)
		}
	}
	if targetingMatch([]byte(`{"country":"DE"}`)) {
		t.Error("non-empty targeting should not match before the DSL milestone")
	}
}
