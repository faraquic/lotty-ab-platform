package outbox

import (
	"errors"
	"strings"
	"sync"

	"github.com/goccy/go-json"
)

const defaultDedupCap = 1024

type Deduper struct {
	mu    sync.Mutex
	max   int
	seen  map[string]struct{}
	order []string
}

func NewDeduper(maxSize int) *Deduper {
	if maxSize <= 0 {
		maxSize = defaultDedupCap
	}

	return &Deduper{
		max:  maxSize,
		seen: make(map[string]struct{}),
	}
}

func (d *Deduper) SeenOrMark(id string) bool {
	if d == nil {
		return false
	}

	if strings.TrimSpace(id) == "" {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.seen[id]; ok {
		return true
	}

	d.seen[id] = struct{}{}
	d.order = append(d.order, id)

	for len(d.order) > d.max {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.seen, oldest)
	}

	return false
}

func (d *Deduper) Len() int {
	if d == nil {
		return 0
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.seen)
}

func ParseEnvelope(raw []byte) (Envelope, error) {
	var env Envelope

	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, err
	}

	if strings.TrimSpace(env.ID) == "" {
		return Envelope{}, errors.New("outbox envelope id is empty")
	}

	if strings.TrimSpace(env.Type) == "" {
		return Envelope{}, errors.New("outbox envelope type is empty")
	}

	if len(env.Data) == 0 || string(env.Data) == "null" {
		return Envelope{}, errors.New("outbox envelope data is empty")
	}

	return env, nil
}
