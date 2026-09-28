package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goccy/go-json"

	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type fakeCatalog struct {
	keys map[string]struct{}
}

func (f fakeCatalog) ActiveEventTypes(context.Context) ([]string, error) {
	out := make([]string, 0, len(f.keys))
	for k := range f.keys {
		out = append(out, k)
	}

	return out, nil
}

func eventEnvelope(t *testing.T, id, eventType string, occurred time.Time) kafka.Message {
	t.Helper()

	payload, err := json.Marshal(events.KafkaEventPayload{
		EventID:    id,
		EventType:  eventType,
		SubjectID:  "user-1",
		OccurredAt: occurred,
		ReceivedAt: occurred,
		DecisionID: id,
		Payload:    json.RawMessage(`{"sku":"a"}`),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	env, err := json.Marshal(outbox.Envelope{
		ID:     id,
		Type:   events.EventEnvelopeType,
		Source: "labp-analytics",
		Time:   occurred,
		Data:   payload,
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	return kafka.Message{Topic: "events", Value: env}
}

func TestValidatorAcceptsValidEvent(t *testing.T) {
	catalog, err := NewValidationCatalog(fakeCatalog{keys: map[string]struct{}{"conversion": {}}}, 0, zap.NewNop())
	if err != nil {
		t.Fatalf("NewValidationCatalog: %v", err)
	}

	v := &Validator{catalog: catalog, now: time.Now}

	msg := eventEnvelope(t, "11111111-1111-1111-1111-111111111111", "conversion", time.Now().Add(-time.Minute))

	if err := v.Handle(context.Background(), msg); err != nil {
		t.Errorf("Handle = %v, want nil", err)
	}
}

func TestValidatorRejectsUnknownType(t *testing.T) {
	catalog, err := NewValidationCatalog(fakeCatalog{keys: map[string]struct{}{"conversion": {}}}, 0, zap.NewNop())
	if err != nil {
		t.Fatalf("NewValidationCatalog: %v", err)
	}

	v := &Validator{catalog: catalog, now: time.Now}

	msg := eventEnvelope(t, "11111111-1111-1111-1111-111111111111", "unknown", time.Now())

	err = v.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if !isNonRetryable(err) {
		t.Errorf("err = %v, want non-retryable", err)
	}

	if v.IsRetryable(err) {
		t.Error("validation error should be non-retryable")
	}
}

func TestValidatorRejectsOutOfRangeTimestamp(t *testing.T) {
	catalog, err := NewValidationCatalog(fakeCatalog{keys: map[string]struct{}{"conversion": {}}}, 0, zap.NewNop())
	if err != nil {
		t.Fatalf("NewValidationCatalog: %v", err)
	}

	v := &Validator{catalog: catalog, now: time.Now}

	msg := eventEnvelope(t, "11111111-1111-1111-1111-111111111111", "conversion", time.Now().Add(-48*time.Hour))

	err = v.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if !isNonRetryable(err) {
		t.Errorf("err = %v, want non-retryable", err)
	}
}

func TestValidatorRejectsMalformedEnvelope(t *testing.T) {
	catalog, err := NewValidationCatalog(fakeCatalog{keys: map[string]struct{}{"conversion": {}}}, 0, zap.NewNop())
	if err != nil {
		t.Fatalf("NewValidationCatalog: %v", err)
	}

	v := &Validator{catalog: catalog, now: time.Now}

	msg := kafka.Message{Value: []byte(`{broken`)}

	err = v.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if !isNonRetryable(err) {
		t.Errorf("err = %v, want non-retryable", err)
	}
}

func TestValidatorRejectsWrongRecordType(t *testing.T) {
	catalog, err := NewValidationCatalog(fakeCatalog{keys: map[string]struct{}{"conversion": {}}}, 0, zap.NewNop())
	if err != nil {
		t.Fatalf("NewValidationCatalog: %v", err)
	}

	v := &Validator{catalog: catalog, now: time.Now}

	payload, err := json.Marshal(events.KafkaEventPayload{
		EventID:    "11111111-1111-1111-1111-111111111111",
		EventType:  "conversion",
		SubjectID:  "user-1",
		OccurredAt: time.Now(),
		Payload:    json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	env, err := json.Marshal(outbox.Envelope{ID: "11111111-1111-1111-1111-111111111111", Type: "exposure", Data: payload})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	err = v.Handle(context.Background(), kafka.Message{Value: env})
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if !isNonRetryable(err) {
		t.Errorf("err = %v, want non-retryable", err)
	}
}

func TestCatalogReload(t *testing.T) {
	fc := fakeCatalog{keys: map[string]struct{}{"conversion": {}}}

	catalog, err := NewValidationCatalog(fc, 0, zap.NewNop())
	if err != nil {
		t.Fatalf("NewValidationCatalog: %v", err)
	}

	if !catalog.Contains("conversion") {
		t.Error("conversion should be in catalog")
	}

	fc.keys["purchase"] = struct{}{}

	if err := catalog.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if !catalog.Contains("purchase") {
		t.Error("purchase should be in catalog after reload")
	}
}

func isNonRetryable(err error) bool {
	return errors.Is(err, consumer.ErrNonRetryable)
}
