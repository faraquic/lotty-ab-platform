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

type fakeDedup struct {
	existing map[string]bool
}

func (f fakeDedup) Exists(_ context.Context, id string) (bool, error) {
	return f.existing[id], nil
}

func (f fakeDedup) Insert(_ context.Context, id string) (bool, error) {
	if f.existing[id] {
		return false, nil
	}

	f.existing[id] = true

	return true, nil
}

func dedupEnvelope(t *testing.T, id string) kafka.Message {
	t.Helper()

	payload, err := json.Marshal(events.KafkaEventPayload{
		EventID:    id,
		EventType:  "conversion",
		SubjectID:  "user-1",
		OccurredAt: time.Now(),
		Payload:    json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	env, err := json.Marshal(outbox.Envelope{ID: id, Type: events.EventEnvelopeType, Data: payload})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	return kafka.Message{Value: env}
}

func TestDedupFirstSeenPasses(t *testing.T) {
	store := &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    fakeDedup{existing: map[string]bool{}},
		log:  zap.NewNop(),
	}

	msg := dedupEnvelope(t, "11111111-1111-1111-1111-111111111111")

	if err := store.Handle(context.Background(), msg); err != nil {
		t.Errorf("Handle = %v, want nil", err)
	}
}

func TestDedupInMemoryDuplicateDropped(t *testing.T) {
	store := &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    fakeDedup{existing: map[string]bool{}},
		log:  zap.NewNop(),
	}

	msg := dedupEnvelope(t, "11111111-1111-1111-1111-111111111111")

	if err := store.Handle(context.Background(), msg); err != nil {
		t.Fatalf("first Handle: %v", err)
	}

	err := store.Handle(context.Background(), msg)
	if !errors.Is(err, consumer.ErrDrop) {
		t.Errorf("second Handle = %v, want ErrDrop", err)
	}
}

func TestDedupPersistentDuplicateDropped(t *testing.T) {
	store := &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    fakeDedup{existing: map[string]bool{"11111111-1111-1111-1111-111111111111": true}},
		log:  zap.NewNop(),
	}

	msg := dedupEnvelope(t, "11111111-1111-1111-1111-111111111111")

	err := store.Handle(context.Background(), msg)
	if !errors.Is(err, consumer.ErrDrop) {
		t.Errorf("Handle = %v, want ErrDrop", err)
	}
}

func TestDedupInsertRaceDropped(t *testing.T) {
	store := &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    fakeDedup{existing: map[string]bool{}},
		log:  zap.NewNop(),
	}

	msg := dedupEnvelope(t, "11111111-1111-1111-1111-111111111111")

	if err := store.Handle(context.Background(), msg); err != nil {
		t.Fatalf("first Handle: %v", err)
	}

	store.seen = outbox.NewDeduper(0)

	err := store.Handle(context.Background(), msg)
	if !errors.Is(err, consumer.ErrDrop) {
		t.Errorf("Handle after in-memory reset = %v, want ErrDrop", err)
	}
}

func TestDedupMalformedGoesToDLQ(t *testing.T) {
	store := &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    fakeDedup{existing: map[string]bool{}},
		log:  zap.NewNop(),
	}

	err := store.Handle(context.Background(), kafka.Message{Value: []byte(`{broken`)})
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if store.IsRetryable(err) {
		t.Error("parse error should be non-retryable")
	}
}

func TestDedupTransientErrorRetryable(t *testing.T) {
	store := &DedupStore{
		seen: outbox.NewDeduper(0),
		q:    fakeDedup{existing: map[string]bool{}},
		log:  zap.NewNop(),
	}

	err := errors.New("pg down")

	if !store.IsRetryable(err) {
		t.Error("pg error should be retryable")
	}
}
