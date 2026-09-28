package ingest

import (
	"testing"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

func envelopeFor(t *testing.T, id, typ string, payload any) outbox.Envelope {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return outbox.Envelope{ID: id, Type: typ, Source: "labp-analytics", Time: time.Now().UTC(), Data: data}
}

func TestEventRowFromEnvelope(t *testing.T) {
	eventID := uuid.NewString()
	decisionID := uuid.NewString()

	env := envelopeFor(t, eventID, "event", map[string]any{
		"event_id":       eventID,
		"event_type":     "conversion",
		"subject_id":     "abc",
		"salt_version":   "v1",
		"occurred_at":    "2026-09-27T12:00:00Z",
		"received_at":    "2026-09-27T12:00:01Z",
		"decision_id":    decisionID,
		"schema_version": 3,
		"payload":        map[string]any{"sku": "a"},
	})

	row, err := EventRowFromEnvelope(env)
	if err != nil {
		t.Fatalf("EventRowFromEnvelope: %v", err)
	}

	if row.EventID.String() != eventID {
		t.Errorf("event id = %s, want %s", row.EventID, eventID)
	}

	if row.EventType != "conversion" || row.SubjectID != "abc" || row.SaltVersion != "v1" {
		t.Errorf("row = %+v", row)
	}

	if row.DecisionID.String() != decisionID {
		t.Errorf("decision id = %s, want %s", row.DecisionID, decisionID)
	}

	if row.SchemaVersion != 3 {
		t.Errorf("schema version = %d, want 3", row.SchemaVersion)
	}

	if row.Payload != `{"sku":"a"}` {
		t.Errorf("payload = %s", row.Payload)
	}
}

func TestEventRowFromEnvelopeEmptyDecision(t *testing.T) {
	eventID := uuid.NewString()

	env := envelopeFor(t, eventID, "event", map[string]any{
		"event_id":    eventID,
		"event_type":  "error",
		"subject_id":  "abc",
		"occurred_at": "2026-09-27T12:00:00Z",
		"received_at": "2026-09-27T12:00:01Z",
		"payload":     map[string]any{},
	})

	row, err := EventRowFromEnvelope(env)
	if err != nil {
		t.Fatalf("EventRowFromEnvelope: %v", err)
	}

	if row.DecisionID != uuid.Nil {
		t.Errorf("decision id = %s, want nil uuid", row.DecisionID)
	}
}

func TestEventRowFromEnvelopeInvalid(t *testing.T) {
	env := envelopeFor(t, "not-a-uuid", "event", map[string]any{"event_id": "not-a-uuid"})

	if _, err := EventRowFromEnvelope(env); err == nil {
		t.Error("bad event id: got nil error")
	}

	broken := outbox.Envelope{ID: uuid.NewString(), Type: "event", Data: []byte(`{nope}`)}

	if _, err := EventRowFromEnvelope(broken); err == nil {
		t.Error("broken data: got nil error")
	}
}

func TestExposureRowFromEnvelope(t *testing.T) {
	ids := map[string]string{
		"event_id":              uuid.NewString(),
		"decision_id":           uuid.NewString(),
		"experiment_id":         uuid.NewString(),
		"experiment_version_id": uuid.NewString(),
		"variant_id":            uuid.NewString(),
		"flag_id":               uuid.NewString(),
	}

	payload := map[string]any{
		"subject_id":   "abc",
		"salt_version": "v2",
		"occurred_at":  "2026-09-27T12:00:00Z",
		"received_at":  "2026-09-27T12:00:01Z",
	}
	for k, v := range ids {
		payload[k] = v
	}

	env := envelopeFor(t, ids["event_id"], "exposure", payload)

	row, err := ExposureRowFromEnvelope(env)
	if err != nil {
		t.Fatalf("ExposureRowFromEnvelope: %v", err)
	}

	if row.EventID.String() != ids["event_id"] {
		t.Errorf("event id = %s", row.EventID)
	}

	if row.ExperimentID.String() != ids["experiment_id"] {
		t.Errorf("experiment id = %s", row.ExperimentID)
	}

	if row.FlagID.String() != ids["flag_id"] {
		t.Errorf("flag id = %s", row.FlagID)
	}

	if row.SubjectID != "abc" || row.SaltVersion != "v2" {
		t.Errorf("row = %+v", row)
	}
}

func TestExposureRowFromEnvelopeInvalid(t *testing.T) {
	env := envelopeFor(t, uuid.NewString(), "exposure", map[string]any{
		"event_id":    uuid.NewString(),
		"decision_id": "nope",
	})

	if _, err := ExposureRowFromEnvelope(env); err == nil {
		t.Error("bad decision id: got nil error")
	}
}
