package snapshot

import "sort"

type FlagInput struct {
	Key   string
	Type  string
	Value []byte
}

type ExperimentInput struct {
	ID           string
	FlagKey      string
	VersionNum   int
	Salt         string
	AllocationBp int
	Targeting    []byte
	Variants     []VariantInput
}

type VariantInput struct {
	ID       string
	Name     string
	Value    []byte
	WeightBp int
}

func Build(flags []FlagInput, experiments []ExperimentInput) *Snapshot {
	flagSnapshots := make([]FlagSnapshot, len(flags))
	for i, f := range flags {
		flagSnapshots[i] = FlagSnapshot(f)
	}

	sort.Slice(flagSnapshots, func(i, j int) bool {
		return flagSnapshots[i].Key < flagSnapshots[j].Key
	})

	expSnapshots := make([]ExperimentSnapshot, len(experiments))
	for i, e := range experiments {
		variants := make([]VariantSnapshot, len(e.Variants))
		for j, v := range e.Variants {
			variants[j] = VariantSnapshot(v)
		}
		sort.Slice(variants, func(a, b int) bool {
			return variants[a].Name < variants[b].Name
		})
		expSnapshots[i] = ExperimentSnapshot{
			ID:           e.ID,
			FlagKey:      e.FlagKey,
			VersionNum:   e.VersionNum,
			Salt:         e.Salt,
			AllocationBp: e.AllocationBp,
			Targeting:    e.Targeting,
			Variants:     variants,
		}
	}

	sort.Slice(expSnapshots, func(i, j int) bool {
		if expSnapshots[i].FlagKey != expSnapshots[j].FlagKey {
			return expSnapshots[i].FlagKey < expSnapshots[j].FlagKey
		}
		return expSnapshots[i].ID < expSnapshots[j].ID
	})

	return &Snapshot{
		Revision:    generateRevision(),
		Flags:       flagSnapshots,
		Experiments: expSnapshots,
	}
}
