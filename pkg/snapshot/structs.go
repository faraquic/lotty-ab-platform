package snapshot

type Revision [14]byte

type Snapshot struct {
	Revision *Revision      `json:"r"`
	Flags    []FlagSnapshot `json:"f"`
}

type FlagSnapshot struct {
	Key   string `json:"k"`
	Type  string `json:"t"`
	Value []byte `json:"v"`
}
