package snapshot

import (
	"testing"

	"github.com/goccy/go-json"
)

func TestBuildSorts(t *testing.T) {
	snap := Build(
		[]FlagInput{{Key: "b", Type: "string", Value: []byte(`"2"`)}, {Key: "a", Type: "string", Value: []byte(`"1"`)}},
		[]ExperimentInput{
			{
				ID: "e2", FlagKey: "b", VersionNum: 1, Salt: "s", AllocationBp: 10000,
				Variants: []VariantInput{
					{ID: "2", Name: "b-var", Value: []byte(`"x"`), WeightBp: 5000},
					{ID: "1", Name: "a-var", Value: []byte(`"y"`), WeightBp: 5000},
				},
			},
			{ID: "e1", FlagKey: "a", VersionNum: 1, Salt: "s", AllocationBp: 10000},
		},
	)

	if snap.Flags[0].Key != "a" || snap.Flags[1].Key != "b" {
		t.Errorf("flags not sorted: %+v", snap.Flags)
	}
	if snap.Experiments[0].FlagKey != "a" || snap.Experiments[1].FlagKey != "b" {
		t.Errorf("experiments not sorted: %+v", snap.Experiments)
	}
	if snap.Experiments[1].Variants[0].Name != "a-var" {
		t.Errorf("variants not sorted: %+v", snap.Experiments[1].Variants)
	}
	if snap.Revision == nil {
		t.Error("revision should be set")
	}
}

func TestLegacyFormatCompat(t *testing.T) {
	raw := `{"f":[{"k":"a","t":"string","v":"IjEi"}]}`
	var snap Snapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		t.Fatalf("unmarshal legacy snapshot: %v", err)
	}
	if len(snap.Flags) != 1 || snap.Flags[0].Key != "a" {
		t.Errorf("flags: %+v", snap.Flags)
	}
	if len(snap.Experiments) != 0 {
		t.Errorf("experiments should be empty: %+v", snap.Experiments)
	}
}

func TestRoundTrip(t *testing.T) {
	snap := Build(
		[]FlagInput{{Key: "a", Type: "bool", Value: []byte(`true`)}},
		[]ExperimentInput{{
			ID: "e1", FlagKey: "a", VersionNum: 2, Salt: "s", AllocationBp: 5000,
			Targeting: []byte(`{"country":"DE"}`),
			Variants:  []VariantInput{{ID: "v1", Name: "c", Value: []byte(`true`), WeightBp: 5000}},
		}},
	)
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Snapshot
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Experiments) != 1 || back.Experiments[0].VersionNum != 2 {
		t.Fatalf("experiments: %+v", back.Experiments)
	}
	if string(back.Experiments[0].Targeting) != `{"country":"DE"}` {
		t.Errorf("targeting: %s", back.Experiments[0].Targeting)
	}
	if back.Experiments[0].Variants[0].WeightBp != 5000 {
		t.Errorf("variants: %+v", back.Experiments[0].Variants)
	}
}
