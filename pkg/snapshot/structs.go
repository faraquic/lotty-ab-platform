package snapshot

type Revision [14]byte

type Snapshot struct {
	Revision    *Revision            `json:"r"`
	Flags       []FlagSnapshot       `json:"f"`
	Experiments []ExperimentSnapshot `json:"e,omitempty"`
}

type FlagSnapshot struct {
	Key   string `json:"k"`
	Type  string `json:"t"`
	Value []byte `json:"v"`
}

type ExperimentSnapshot struct {
	ID           string            `json:"id"`
	FlagKey      string            `json:"fk"`
	VersionNum   int               `json:"vn"`
	Salt         string            `json:"s"`
	AllocationBp int               `json:"ab"`
	Targeting    []byte            `json:"tg,omitempty"`
	Variants     []VariantSnapshot `json:"v"`
}

type VariantSnapshot struct {
	ID       string `json:"id"`
	Name     string `json:"n"`
	Value    []byte `json:"v"`
	WeightBp int    `json:"w"`
}
