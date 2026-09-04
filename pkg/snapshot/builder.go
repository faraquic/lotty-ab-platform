package snapshot

import "sort"

type FlagInput struct {
	Key   string
	Type  string
	Value []byte
}

func Build(flags []FlagInput) *Snapshot {
	snapshots := make([]FlagSnapshot, len(flags))
	for i, f := range flags {
		snapshots[i] = FlagSnapshot(f)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Key < snapshots[j].Key
	})

	return &Snapshot{
		Revision: generateRevision(),
		Flags:    snapshots,
	}
}
