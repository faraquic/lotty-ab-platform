package events

import (
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

func TestResolveSubjectPassthrough(t *testing.T) {
	ref, err := ResolveSubject("user-1", "", "", false)
	if err != nil {
		t.Fatalf("ResolveSubject: %v", err)
	}

	if ref.Value != "user-1" || ref.SaltVersion != "" {
		t.Errorf("ref = %+v, want passthrough", ref)
	}
}

func TestResolveSubjectHash(t *testing.T) {
	a, err := ResolveSubject("user-1", "v1", "salt", true)
	if err != nil {
		t.Fatalf("ResolveSubject: %v", err)
	}

	if a.Value == "user-1" {
		t.Error("hashed value equals raw subject")
	}

	if a.SaltVersion != "v1" {
		t.Errorf("salt version = %q, want v1", a.SaltVersion)
	}

	b, err := ResolveSubject("user-1", "v1", "salt", true)
	if err != nil {
		t.Fatalf("ResolveSubject: %v", err)
	}

	if a.Value != b.Value {
		t.Error("hash is not deterministic")
	}

	c, err := ResolveSubject("user-1", "v2", "other", true)
	if err != nil {
		t.Fatalf("ResolveSubject: %v", err)
	}

	if a.Value == c.Value {
		t.Error("salt rotation did not change the hash")
	}
}

func TestResolveSubjectLimits(t *testing.T) {
	if _, err := ResolveSubject("", "v1", "salt", true); err != ErrSubjectInvalid {
		t.Errorf("empty subject err = %v, want ErrSubjectInvalid", err)
	}

	if _, err := ResolveSubject(strings.Repeat("a", 129), "v1", "salt", true); err != ErrSubjectInvalid {
		t.Errorf("129-byte subject err = %v, want ErrSubjectInvalid", err)
	}

	multibyte := strings.Repeat("é", 65)
	if len([]byte(multibyte)) <= MaxSubjectBytes {
		t.Fatalf("test setup: want multibyte string over %d bytes", MaxSubjectBytes)
	}

	if _, err := ResolveSubject(multibyte, "v1", "salt", true); err != ErrSubjectInvalid {
		t.Errorf("over-limit multibyte subject err = %v, want ErrSubjectInvalid", err)
	}

	if _, err := ResolveSubject("user-1", "v9", "", true); err != ErrPIISaltMissing {
		t.Errorf("missing salt err = %v, want ErrPIISaltMissing", err)
	}
}

func TestCheckOccurredAt(t *testing.T) {
	now := time.Now()

	if err := CheckOccurredAt(now, now); err != nil {
		t.Errorf("now err = %v, want nil", err)
	}

	if err := CheckOccurredAt(now.Add(FutureSkew-time.Second), now); err != nil {
		t.Errorf("within skew err = %v, want nil", err)
	}

	if err := CheckOccurredAt(now.Add(FutureSkew+time.Second), now); err != ErrOccurredAtOutOfRange {
		t.Errorf("future err = %v, want ErrOccurredAtOutOfRange", err)
	}

	if err := CheckOccurredAt(now.Add(-LateWindow-time.Second), now); err != ErrOccurredAtOutOfRange {
		t.Errorf("late err = %v, want ErrOccurredAtOutOfRange", err)
	}
}

func TestCanonicalPayload(t *testing.T) {
	out, err := CanonicalPayload(json.RawMessage(`{"b":1,"a":2}`))
	if err != nil {
		t.Fatalf("CanonicalPayload: %v", err)
	}

	if string(out) != `{"a":2,"b":1}` {
		t.Errorf("canonical = %s, want sorted keys", out)
	}

	if _, err := CanonicalPayload(json.RawMessage(`{nope}`)); err != ErrPayloadInvalid {
		t.Errorf("invalid json err = %v, want ErrPayloadInvalid", err)
	}

	big := json.RawMessage(`"` + strings.Repeat("x", MaxPayloadBytes) + `"`)
	if _, err := CanonicalPayload(big); err != ErrPayloadTooLarge {
		t.Errorf("oversize err = %v, want ErrPayloadTooLarge", err)
	}
}

func TestCheckIDs(t *testing.T) {
	id := uuid.NewString()

	if err := CheckEventID(id); err != nil {
		t.Errorf("CheckEventID(valid) = %v", err)
	}

	if err := CheckEventID("nope"); err != ErrEventIDInvalid {
		t.Errorf("CheckEventID(invalid) = %v, want ErrEventIDInvalid", err)
	}

	if err := CheckDecisionID(""); err != nil {
		t.Errorf("CheckDecisionID(empty) = %v", err)
	}

	if err := CheckDecisionID(id); err != nil {
		t.Errorf("CheckDecisionID(valid) = %v", err)
	}

	if err := CheckDecisionID("nope"); err != ErrDecisionIDInvalid {
		t.Errorf("CheckDecisionID(invalid) = %v, want ErrDecisionIDInvalid", err)
	}

	if err := CheckReferenceID(id); err != nil {
		t.Errorf("CheckReferenceID(valid) = %v", err)
	}

	if err := CheckReferenceID("nope"); err != ErrExposureFieldInvalid {
		t.Errorf("CheckReferenceID(invalid) = %v, want ErrExposureFieldInvalid", err)
	}
}

func TestEventPartitionKey(t *testing.T) {
	if got := EventPartitionKey("d1", "e1"); got != "d1" {
		t.Errorf("with decision = %q, want d1", got)
	}

	if got := EventPartitionKey("", "e1"); got != "e1" {
		t.Errorf("without decision = %q, want e1", got)
	}
}

func TestRequestHashDeterministic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	eventID := uuid.NewString()

	mk := func() EventItem {
		return EventItem{
			EventID:    eventID,
			EventType:  "purchase",
			SubjectID:  "user-1",
			OccurredAt: now,
			Payload:    json.RawMessage(`{"sku":"a"}`),
		}
	}

	a, err := RequestHashEvent(mk())
	if err != nil {
		t.Fatalf("RequestHashEvent: %v", err)
	}

	b, err := RequestHashEvent(mk())
	if err != nil {
		t.Fatalf("RequestHashEvent: %v", err)
	}

	if string(a) != string(b) {
		t.Error("identical items produced different hashes")
	}

	cItem := mk()
	cItem.SubjectID = "user-2"

	c, err := RequestHashEvent(cItem)
	if err != nil {
		t.Fatalf("RequestHashEvent: %v", err)
	}

	if string(a) == string(c) {
		t.Error("different subjects produced identical hashes")
	}

	dItem := mk()
	dItem.Payload = json.RawMessage(`{"sku": "a"}`)

	d, err := RequestHashEvent(dItem)
	if err != nil {
		t.Fatalf("RequestHashEvent: %v", err)
	}

	if string(a) != string(d) {
		t.Error("whitespace-only payload difference changed the hash")
	}
}
