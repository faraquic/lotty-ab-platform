package attribution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/consumer"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type fakePendingStore struct {
	exposures   map[string]PendingExposure
	conversions map[string]PendingConversion
}

func newFakePendingStore() *fakePendingStore {
	return &fakePendingStore{
		exposures:   make(map[string]PendingExposure),
		conversions: make(map[string]PendingConversion),
	}
}

func (f *fakePendingStore) RegisterExposure(_ context.Context, decisionID string, pe PendingExposure, _ time.Duration) error {
	if _, exists := f.exposures[decisionID]; exists {
		return nil
	}
	f.exposures[decisionID] = pe
	return nil
}

func (f *fakePendingStore) GetExposure(_ context.Context, decisionID string) (*PendingExposure, error) {
	pe, ok := f.exposures[decisionID]
	if !ok {
		return nil, nil
	}
	return &pe, nil
}

func (f *fakePendingStore) DeleteExposure(_ context.Context, decisionID string) error {
	delete(f.exposures, decisionID)
	return nil
}

func (f *fakePendingStore) RegisterConversion(_ context.Context, decisionID string, pc PendingConversion, _ time.Duration) error {
	if _, exists := f.conversions[decisionID]; exists {
		return nil
	}
	f.conversions[decisionID] = pc
	return nil
}

func (f *fakePendingStore) GetConversion(_ context.Context, decisionID string) (*PendingConversion, error) {
	pc, ok := f.conversions[decisionID]
	if !ok {
		return nil, nil
	}
	return &pc, nil
}

func (f *fakePendingStore) DeleteConversion(_ context.Context, decisionID string) error {
	delete(f.conversions, decisionID)
	return nil
}

func (f *fakePendingStore) ExpiredConversions(_ context.Context, now time.Time) ([]string, error) {
	var expired []string
	for id, pc := range f.conversions {
		if pc.TTLAt.Before(now) {
			expired = append(expired, id)
		}
	}
	return expired, nil
}

type fakeAttrWriter struct {
	mu   sync.Mutex
	rows []clickhouse.AttributedEventRow
}

func (f *fakeAttrWriter) WriteAttributed(_ context.Context, row clickhouse.AttributedEventRow) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, row)
}

func (f *fakeAttrWriter) getRows() []clickhouse.AttributedEventRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]clickhouse.AttributedEventRow, len(f.rows))
	copy(out, f.rows)
	return out
}

type fakeCatalog struct {
	types map[string]bool
}

func (f *fakeCatalog) RequiresExposure(eventType string) bool {
	return f.types[eventType]
}

func makeEventEnvelope(t *testing.T, eventID, eventType, decisionID string, occurredAt time.Time) kafka.Message {
	t.Helper()

	payload, err := json.Marshal(events.KafkaEventPayload{
		EventID:    eventID,
		EventType:  eventType,
		SubjectID:  "user-1",
		OccurredAt: occurredAt,
		Payload:    json.RawMessage(`{}`),
		DecisionID: decisionID,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	env, err := json.Marshal(outbox.Envelope{ID: eventID, Type: events.EventEnvelopeType, Data: payload})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	return kafka.Message{Value: env}
}

func makeAttributor(store PendingStore, catalog *fakeCatalog, writer *fakeAttrWriter) *Attributor {
	return NewAttributor(
		store,
		catalog,
		writer,
		7*24*time.Hour,
		time.Hour,
		time.Now,
		zap.NewNop(),
	)
}

func TestAttributor_ExposureThenConversion_Attributes(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"conversion": true}}
	attr := makeAttributor(store, catalog, writer)

	decisionID := uuid.New().String()
	conversionID := uuid.New().String()
	now := time.Now()

	store.exposures[decisionID] = PendingExposure{
		ExperimentID:        uuid.New().String(),
		ExperimentVersionID: uuid.New().String(),
		VariantID:           uuid.New().String(),
		FlagID:              uuid.New().String(),
		SubjectID:           "user-1",
		OccurredAt:          now,
		TTLAt:               now.Add(7 * 24 * time.Hour),
	}

	expectedExpID := store.exposures[decisionID].ExperimentID
	expectedVarID := store.exposures[decisionID].VariantID

	msg := makeEventEnvelope(t, conversionID, "conversion", decisionID, now)
	if err := attr.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	rows := writer.getRows()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	if rows[0].DecisionID.String() != decisionID {
		t.Errorf("DecisionID = %s, want %s", rows[0].DecisionID, decisionID)
	}

	if rows[0].ExperimentID.String() != expectedExpID {
		t.Errorf("ExperimentID = %s, want %s", rows[0].ExperimentID, expectedExpID)
	}

	if rows[0].VariantID.String() != expectedVarID {
		t.Errorf("VariantID = %s, want %s", rows[0].VariantID, expectedVarID)
	}

	if _, ok := store.exposures[decisionID]; ok {
		t.Error("pending exposure should be deleted after attribution")
	}
}

func TestAttributor_ConversionThenExposure_OutOfOrder(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"conversion": true}}
	attr := makeAttributor(store, catalog, writer)

	decisionID := uuid.New().String()
	conversionID := uuid.New().String()
	now := time.Now()

	msg := makeEventEnvelope(t, conversionID, "conversion", decisionID, now)
	if err := attr.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if len(writer.getRows()) != 0 {
		t.Fatal("no attribution should happen before exposure")
	}

	pc, ok := store.conversions[decisionID]
	if !ok {
		t.Fatal("pending conversion should be registered")
	}

	if pc.EventID != conversionID {
		t.Errorf("EventID = %s, want %s", pc.EventID, conversionID)
	}

	store.exposures[decisionID] = PendingExposure{
		ExperimentID:        uuid.New().String(),
		ExperimentVersionID: uuid.New().String(),
		VariantID:           uuid.New().String(),
		FlagID:              uuid.New().String(),
		SubjectID:           "user-1",
		OccurredAt:          now,
		TTLAt:               now.Add(7 * 24 * time.Hour),
	}

	msg2 := makeEventEnvelope(t, conversionID, "conversion", decisionID, now)
	if err := attr.Handle(context.Background(), msg2); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	rows := writer.getRows()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	if rows[0].DecisionID.String() != decisionID {
		t.Errorf("DecisionID = %s, want %s", rows[0].DecisionID, decisionID)
	}
}

func TestAttributor_TechnicalEvent_BypassesExposure(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"error": false}}
	attr := makeAttributor(store, catalog, writer)

	decisionID := uuid.New().String()
	eventID := uuid.New().String()
	now := time.Now()

	msg := makeEventEnvelope(t, eventID, "error", decisionID, now)
	if err := attr.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	rows := writer.getRows()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	if rows[0].ExperimentID != uuid.Nil {
		t.Errorf("ExperimentID should be Nil for technical event, got %s", rows[0].ExperimentID)
	}

	if rows[0].VariantID != uuid.Nil {
		t.Errorf("VariantID should be Nil for technical event, got %s", rows[0].VariantID)
	}
}

func TestAttributor_NoDecisionID_Skips(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"conversion": true}}
	attr := makeAttributor(store, catalog, writer)

	eventID := uuid.New().String()
	now := time.Now()

	msg := makeEventEnvelope(t, eventID, "conversion", "", now)
	if err := attr.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if len(writer.getRows()) != 0 {
		t.Fatal("no attribution without decision_id")
	}
}

func TestAttributor_DuplicateConversion_NoDuplicateAttribution(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"conversion": true}}
	attr := makeAttributor(store, catalog, writer)

	decisionID := uuid.New().String()
	conversionID := uuid.New().String()
	now := time.Now()

	store.exposures[decisionID] = PendingExposure{
		ExperimentID:        uuid.New().String(),
		ExperimentVersionID: uuid.New().String(),
		VariantID:           uuid.New().String(),
		FlagID:              uuid.New().String(),
		SubjectID:           "user-1",
		OccurredAt:          now,
		TTLAt:               now.Add(7 * 24 * time.Hour),
	}

	msg := makeEventEnvelope(t, conversionID, "conversion", decisionID, now)
	if err := attr.Handle(context.Background(), msg); err != nil {
		t.Fatalf("first Handle: %v", err)
	}

	if err := attr.Handle(context.Background(), msg); err != nil {
		t.Fatalf("second Handle: %v", err)
	}

	rows := writer.getRows()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (duplicate suppressed)", len(rows))
	}
}

func TestAttributor_MalformedEnvelope_NonRetryable(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"conversion": true}}
	attr := makeAttributor(store, catalog, writer)

	err := attr.Handle(context.Background(), kafka.Message{Value: []byte(`{broken`)})
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if !errors.Is(err, consumer.ErrNonRetryable) {
		t.Error("parse error should be non-retryable")
	}
}

func TestAttributor_WrongRecordType_NonRetryable(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	catalog := &fakeCatalog{types: map[string]bool{"conversion": true}}
	attr := makeAttributor(store, catalog, writer)

	eventID := uuid.New().String()
	now := time.Now()

	payload, _ := json.Marshal(events.KafkaEventPayload{
		EventID:    eventID,
		EventType:  "conversion",
		SubjectID:  "user-1",
		OccurredAt: now,
		Payload:    json.RawMessage(`{}`),
	})

	env, _ := json.Marshal(outbox.Envelope{ID: eventID, Type: "exposure", Data: payload})

	err := attr.Handle(context.Background(), kafka.Message{Value: env})
	if err == nil {
		t.Fatal("Handle = nil, want error")
	}

	if !errors.Is(err, consumer.ErrNonRetryable) {
		t.Error("wrong record type should be non-retryable")
	}
}

func TestSweeper_ExpiredConversion_WritesExpiredFact(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	sweeper := NewSweeper(store, writer, time.Now, zap.NewNop())

	decisionID := uuid.New().String()
	eventID := uuid.New().String()
	now := time.Now()

	store.conversions[decisionID] = PendingConversion{
		EventID:    eventID,
		EventType:  "conversion",
		SubjectID:  "user-1",
		OccurredAt: now.Add(-8 * 24 * time.Hour),
		TTLAt:      now.Add(-time.Hour),
	}

	if err := sweeper.sweep(context.Background()); err != nil {
		t.Fatalf("sweep = %v, want nil", err)
	}

	rows := writer.getRows()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	if rows[0].ExperimentID != uuid.Nil {
		t.Errorf("ExperimentID should be Nil for expired, got %s", rows[0].ExperimentID)
	}

	if rows[0].VariantID != uuid.Nil {
		t.Errorf("VariantID should be Nil for expired, got %s", rows[0].VariantID)
	}

	if _, ok := store.conversions[decisionID]; ok {
		t.Error("pending conversion should be deleted after expiry")
	}
}

func TestSweeper_ActiveConversion_NotExpired(t *testing.T) {
	store := newFakePendingStore()
	writer := &fakeAttrWriter{}
	sweeper := NewSweeper(store, writer, time.Now, zap.NewNop())

	decisionID := uuid.New().String()
	eventID := uuid.New().String()
	now := time.Now()

	store.conversions[decisionID] = PendingConversion{
		EventID:    eventID,
		EventType:  "conversion",
		SubjectID:  "user-1",
		OccurredAt: now,
		TTLAt:      now.Add(24 * time.Hour),
	}

	if err := sweeper.sweep(context.Background()); err != nil {
		t.Fatalf("sweep = %v, want nil", err)
	}

	if len(writer.getRows()) != 0 {
		t.Fatal("active conversion should not be expired")
	}
}
