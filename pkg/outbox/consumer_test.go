package outbox

import (
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestDeduperFirstSeenThenDuplicate(t *testing.T) {
	d := NewDeduper(0)

	if d.SeenOrMark("a") {
		t.Error("first SeenOrMark(a) = true, want false")
	}

	if !d.SeenOrMark("a") {
		t.Error("second SeenOrMark(a) = false, want true")
	}

	if d.SeenOrMark("b") {
		t.Error("first SeenOrMark(b) = true, want false")
	}
}

func TestDeduperBlankIDNeverDeduped(t *testing.T) {
	d := NewDeduper(0)

	for _, id := range []string{"", "   "} {
		if d.SeenOrMark(id) {
			t.Errorf("SeenOrMark(%q) = true, want false", id)
		}

		if d.SeenOrMark(id) {
			t.Errorf("repeated SeenOrMark(%q) = true, want false", id)
		}
	}

	if got := d.Len(); got != 0 {
		t.Errorf("Len = %d, want 0", got)
	}
}

func TestDeduperEvictsOldest(t *testing.T) {
	d := NewDeduper(2)

	d.SeenOrMark("a")
	d.SeenOrMark("b")
	d.SeenOrMark("c")

	if !d.SeenOrMark("b") {
		t.Error("SeenOrMark(b) = false, want true")
	}

	if !d.SeenOrMark("c") {
		t.Error("SeenOrMark(c) = false, want true")
	}

	if d.SeenOrMark("a") {
		t.Error("SeenOrMark(a) after eviction = true, want false")
	}
}

func TestDeduperNilSafe(t *testing.T) {
	var d *Deduper

	if d.SeenOrMark("a") {
		t.Error("nil SeenOrMark = true, want false")
	}

	if got := d.Len(); got != 0 {
		t.Errorf("nil Len = %d, want 0", got)
	}
}

func TestParseEnvelope(t *testing.T) {
	valid := `{"id":"row-1","type":"experiment.created","source":"labp-panel","time":"2026-09-27T00:00:00Z","data":{"experiment_id":"e1"}}`

	env, err := ParseEnvelope([]byte(valid))
	if err != nil {
		t.Fatalf("ParseEnvelope(valid) error: %v", err)
	}

	if env.ID != "row-1" || env.Type != "experiment.created" {
		t.Errorf("envelope = %+v, want id row-1 type experiment.created", env)
	}

	if string(env.Data) != `{"experiment_id":"e1"}` {
		t.Errorf("data = %s", env.Data)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{name: "malformed", raw: "{not json"},
		{name: "empty id", raw: `{"id":"","type":"t","data":{"a":1}}`},
		{name: "blank id", raw: `{"id":"  ","type":"t","data":{"a":1}}`},
		{name: "missing id", raw: `{"type":"t","data":{"a":1}}`},
		{name: "empty type", raw: `{"id":"x","type":"","data":{"a":1}}`},
		{name: "missing data", raw: `{"id":"x","type":"t"}`},
		{name: "null data", raw: `{"id":"x","type":"t","data":null}`},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseEnvelope([]byte(tt.raw)); err == nil {
				t.Errorf("ParseEnvelope(%s) = nil error, want error", tt.raw)
			}
		})
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	env := Envelope{
		ID:     "row-2",
		Type:   "experiment.created",
		Source: "labp-panel",
		Time:   time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		Data:   []byte(`{"experiment_id":"e1"}`),
	}

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	back, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}

	if back.ID != env.ID || back.Type != env.Type || string(back.Data) != string(env.Data) {
		t.Errorf("round trip = %+v, want %+v", back, env)
	}
}
