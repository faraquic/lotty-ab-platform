package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type fakeTypes struct {
	types map[string]EventType
	err   error
}

func (f *fakeTypes) ListActiveEventTypes(context.Context) ([]EventType, error) {
	if f.err != nil {
		return nil, f.err
	}

	out := make([]EventType, 0, len(f.types))
	for _, t := range f.types {
		out = append(out, t)
	}

	return out, nil
}

type fakeClaims struct {
	rows        map[string]IdempotencyRecord
	insertErr   error
	replaceErr  error
	deleted     [][]string
	uniqueCodes map[string]bool
}

func newFakeClaims() *fakeClaims {
	return &fakeClaims{rows: map[string]IdempotencyRecord{}, uniqueCodes: map[string]bool{}}
}

func (f *fakeClaims) FindIdempotencyKey(_ context.Context, eventID string) (IdempotencyRecord, error) {
	rec, ok := f.rows[eventID]
	if !ok {
		return IdempotencyRecord{}, ErrIdempotencyNotFound
	}

	return rec, nil
}

func (f *fakeClaims) InsertIdempotencyKey(_ context.Context, rec IdempotencyRecord) error {
	if f.insertErr != nil {
		return f.insertErr
	}

	if _, ok := f.rows[rec.EventID]; ok || f.uniqueCodes[rec.EventID] {
		return &pgconn.PgError{Code: "23505"}
	}

	f.rows[rec.EventID] = rec

	return nil
}

func (f *fakeClaims) ReplaceIdempotencyKey(_ context.Context, rec IdempotencyRecord) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}

	if _, ok := f.rows[rec.EventID]; !ok {
		return ErrIdempotencyNotFound
	}

	f.rows[rec.EventID] = rec

	return nil
}

func (f *fakeClaims) DeleteIdempotencyKeys(_ context.Context, eventIDs []string) error {
	f.deleted = append(f.deleted, eventIDs)

	for _, id := range eventIDs {
		delete(f.rows, id)
	}

	return nil
}

type fakePublisher struct {
	published [][]Record
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, records []Record) error {
	if f.err != nil {
		return f.err
	}

	f.published = append(f.published, records)

	return nil
}

func testService(now time.Time, types map[string]EventType, claims *fakeClaims, pub *fakePublisher, pii config.PIIConfig) *Service {
	return &Service{
		repo:           &fakeTypes{types: types},
		claims:         claims,
		producer:       pub,
		eventsTopic:    "events",
		exposuresTopic: "exposures",
		pii:            pii,
		log:            zap.NewNop(),
		now:            func() time.Time { return now },
	}
}

func testPII() config.PIIConfig {
	return config.PIIConfig{HashSubjectID: true, Salts: map[string]string{"v1": "salt"}, CurrentSaltVersion: "v1"}
}

func testTypes() map[string]EventType {
	return map[string]EventType{
		"purchase": {Key: "purchase", Name: "Purchase", SchemaVersion: 3, RequireExposure: true, Status: "active"},
	}
}

func testEventItem(now time.Time) EventItem {
	decision := uuid.NewString()

	return EventItem{
		EventID:    uuid.NewString(),
		EventType:  "purchase",
		SubjectID:  "user-1",
		OccurredAt: now.Add(-time.Minute),
		DecisionID: &decision,
		Payload:    json.RawMessage(`{"sku":"a"}`),
	}
}

func TestIngestEventsAccepted(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	pub := &fakePublisher{}
	svc := testService(now, testTypes(), claims, pub, testPII())

	item := testEventItem(now)

	res, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{item}})
	if err != nil {
		t.Fatalf("IngestEvents: %v", err)
	}

	if res.Accepted != 1 || res.Rejected != 0 || res.Duplicates != 0 {
		t.Errorf("result = %+v, want 1 accepted", res)
	}

	if len(pub.published) != 1 || len(pub.published[0]) != 1 {
		t.Fatalf("published batches = %d, want 1 batch with 1 record", len(pub.published))
	}

	rec := pub.published[0][0]

	if rec.Topic != "events" {
		t.Errorf("topic = %q, want events", rec.Topic)
	}

	if rec.Key != item.DecisionIDValue() {
		t.Errorf("key = %q, want decision id", rec.Key)
	}

	if rec.ID != item.EventID || rec.Type != EventEnvelopeType {
		t.Errorf("record = %+v, want id match and event type", rec)
	}

	payload, ok := rec.Payload.(KafkaEventPayload)
	if !ok {
		t.Fatalf("payload type = %T, want KafkaEventPayload", rec.Payload)
	}

	if payload.SubjectID == "user-1" {
		t.Error("subject was not hashed")
	}

	if payload.SaltVersion != "v1" {
		t.Errorf("salt version = %q, want v1", payload.SaltVersion)
	}

	if payload.SchemaVersion != 3 {
		t.Errorf("schema version = %d, want catalog default 3", payload.SchemaVersion)
	}

	if _, ok := claims.rows[item.EventID]; !ok {
		t.Error("idempotency row was not stored")
	}
}

func TestIngestEventsValidation(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name   string
		mutate func(*EventItem)
	}{
		{name: "bad id", mutate: func(i *EventItem) { i.EventID = "nope" }},
		{name: "unknown type", mutate: func(i *EventItem) { i.EventType = "nope" }},
		{name: "bad decision", mutate: func(i *EventItem) { d := "nope"; i.DecisionID = &d }},
		{name: "future", mutate: func(i *EventItem) { i.OccurredAt = now.Add(time.Hour) }},
		{name: "too old", mutate: func(i *EventItem) { i.OccurredAt = now.Add(-LateWindow - time.Hour) }},
		{name: "bad payload", mutate: func(i *EventItem) { i.Payload = json.RawMessage(`{nope}`) }},
		{name: "empty subject", mutate: func(i *EventItem) { i.SubjectID = "" }},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			claims := newFakeClaims()
			pub := &fakePublisher{}
			svc := testService(now, testTypes(), claims, pub, testPII())

			item := testEventItem(now)
			tt.mutate(&item)

			res, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{item}})
			if err != nil {
				t.Fatalf("IngestEvents: %v", err)
			}

			if res.Rejected != 1 || res.Accepted != 0 {
				t.Errorf("result = %+v, want 1 rejected", res)
			}

			if res.Results[0].Error == nil {
				t.Error("rejected item has no error message")
			}

			if len(pub.published) != 0 {
				t.Error("invalid item was published")
			}
		})
	}
}

func TestIngestEventsDuplicateAndConflict(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	pub := &fakePublisher{}
	svc := testService(now, testTypes(), claims, pub, testPII())

	item := testEventItem(now)
	req := EventBatchRequest{Events: []EventItem{item}}

	first, err := svc.IngestEvents(context.Background(), req)
	if err != nil {
		t.Fatalf("first IngestEvents: %v", err)
	}

	if first.Accepted != 1 {
		t.Fatalf("first result = %+v, want accepted", first)
	}

	second, err := svc.IngestEvents(context.Background(), req)
	if err != nil {
		t.Fatalf("second IngestEvents: %v", err)
	}

	if second.Duplicates != 1 || second.Accepted != 0 {
		t.Errorf("second result = %+v, want 1 duplicate", second)
	}

	if len(pub.published) != 1 {
		t.Errorf("published batches = %d, want 1 (no republish)", len(pub.published))
	}

	changed := item
	changed.SubjectID = "user-2"

	third, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{changed}})
	if err != nil {
		t.Fatalf("third IngestEvents: %v", err)
	}

	if third.Rejected != 1 {
		t.Errorf("third result = %+v, want 1 rejected", third)
	}

	if third.Results[0].Error == nil || *third.Results[0].Error != ErrIdempotencyConflict.Error() {
		t.Errorf("third error = %+v, want conflict", third.Results[0])
	}

	if len(pub.published) != 1 {
		t.Errorf("published batches = %d, want 1 (conflict not published)", len(pub.published))
	}
}

func TestIngestEventsExpiredTakeover(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	pub := &fakePublisher{}
	svc := testService(now, testTypes(), claims, pub, testPII())

	item := testEventItem(now)

	claims.rows[item.EventID] = IdempotencyRecord{
		EventID:     item.EventID,
		RequestHash: []byte("stale"),
		Response:    AcceptedResponse(),
		ExpiresAt:   now.Add(-time.Hour),
	}

	res, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{item}})
	if err != nil {
		t.Fatalf("IngestEvents: %v", err)
	}

	if res.Accepted != 1 {
		t.Errorf("result = %+v, want accepted after takeover", res)
	}

	if len(pub.published) != 1 {
		t.Errorf("published batches = %d, want 1", len(pub.published))
	}
}

func TestIngestEventsBrokerFailureReleasesClaims(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	pub := &fakePublisher{err: ErrBrokerUnavailable}
	svc := testService(now, testTypes(), claims, pub, testPII())

	item := testEventItem(now)

	_, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{item}})
	if !errors.Is(err, ErrBrokerUnavailable) {
		t.Fatalf("err = %v, want ErrBrokerUnavailable", err)
	}

	if _, ok := claims.rows[item.EventID]; ok {
		t.Error("claim was not released after publish failure")
	}

	if len(claims.deleted) != 1 || len(claims.deleted[0]) != 1 {
		t.Errorf("deleted = %+v, want 1 release call", claims.deleted)
	}
}

func TestIngestEventsPassthroughPII(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	pub := &fakePublisher{}
	pii := config.PIIConfig{HashSubjectID: false}
	svc := testService(now, testTypes(), claims, pub, pii)

	item := testEventItem(now)

	if _, err := svc.IngestEvents(context.Background(), EventBatchRequest{Events: []EventItem{item}}); err != nil {
		t.Fatalf("IngestEvents: %v", err)
	}

	payload := pub.published[0][0].Payload.(KafkaEventPayload)

	if payload.SubjectID != "user-1" || payload.SaltVersion != "" {
		t.Errorf("payload = %+v, want raw subject without salt version", payload)
	}
}

func testExposureItem(now time.Time) ExposureItem {
	return ExposureItem{
		EventID:             uuid.NewString(),
		DecisionID:          uuid.NewString(),
		SubjectID:           "user-9",
		ExperimentID:        uuid.NewString(),
		ExperimentVersionID: uuid.NewString(),
		VariantID:           uuid.NewString(),
		FlagID:              uuid.NewString(),
		OccurredAt:          now.Add(-time.Minute),
	}
}

func TestIngestExposuresAccepted(t *testing.T) {
	now := time.Now()
	claims := newFakeClaims()
	pub := &fakePublisher{}
	svc := testService(now, testTypes(), claims, pub, testPII())

	item := testExposureItem(now)

	res, err := svc.IngestExposures(context.Background(), ExposureBatchRequest{Exposures: []ExposureItem{item}})
	if err != nil {
		t.Fatalf("IngestExposures: %v", err)
	}

	if res.Accepted != 1 || res.Rejected != 0 {
		t.Errorf("result = %+v, want 1 accepted", res)
	}

	rec := pub.published[0][0]

	if rec.Topic != "exposures" {
		t.Errorf("topic = %q, want exposures", rec.Topic)
	}

	if rec.Key != item.ExperimentID {
		t.Errorf("key = %q, want experiment id", rec.Key)
	}

	payload, ok := rec.Payload.(KafkaExposurePayload)
	if !ok {
		t.Fatalf("payload type = %T, want KafkaExposurePayload", rec.Payload)
	}

	if payload.SubjectID == "user-9" {
		t.Error("subject was not hashed")
	}

	if payload.ExperimentID != item.ExperimentID || payload.DecisionID != item.DecisionID {
		t.Errorf("payload = %+v, want ids preserved", payload)
	}
}

func TestIngestExposuresValidation(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name   string
		mutate func(*ExposureItem)
	}{
		{name: "bad event id", mutate: func(i *ExposureItem) { i.EventID = "nope" }},
		{name: "bad decision", mutate: func(i *ExposureItem) { i.DecisionID = "nope" }},
		{name: "bad experiment", mutate: func(i *ExposureItem) { i.ExperimentID = "nope" }},
		{name: "bad version", mutate: func(i *ExposureItem) { i.ExperimentVersionID = "nope" }},
		{name: "bad variant", mutate: func(i *ExposureItem) { i.VariantID = "nope" }},
		{name: "bad flag", mutate: func(i *ExposureItem) { i.FlagID = "nope" }},
		{name: "future", mutate: func(i *ExposureItem) { i.OccurredAt = now.Add(time.Hour) }},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			claims := newFakeClaims()
			pub := &fakePublisher{}
			svc := testService(now, testTypes(), claims, pub, testPII())

			item := testExposureItem(now)
			tt.mutate(&item)

			res, err := svc.IngestExposures(context.Background(), ExposureBatchRequest{Exposures: []ExposureItem{item}})
			if err != nil {
				t.Fatalf("IngestExposures: %v", err)
			}

			if res.Rejected != 1 {
				t.Errorf("result = %+v, want 1 rejected", res)
			}

			if len(pub.published) != 0 {
				t.Error("invalid exposure was published")
			}
		})
	}
}
