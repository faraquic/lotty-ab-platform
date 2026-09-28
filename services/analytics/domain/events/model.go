package events

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

var (
	ErrEventIDInvalid       = errors.New("event_id must be a uuid")
	ErrUnknownEventType     = errors.New("unknown event type")
	ErrEventTypeArchived    = errors.New("event type is archived")
	ErrSubjectInvalid       = errors.New("subject_id must be 1..128 bytes")
	ErrOccurredAtOutOfRange = errors.New("occurred_at is outside the accepted window")
	ErrDecisionIDInvalid    = errors.New("decision_id must be a uuid")
	ErrPayloadInvalid       = errors.New("payload must be valid json")
	ErrPayloadTooLarge      = errors.New("payload exceeds 32kb")
	ErrExposureFieldInvalid = errors.New("exposure reference id must be a uuid")
	ErrIdempotencyConflict  = errors.New("event_id already used with a different body")
	ErrBrokerUnavailable    = errors.New("event broker unavailable")
	ErrPIISaltMissing       = errors.New("pii salt is missing for the current version")
	ErrIdempotencyNotFound  = errors.New("idempotency record not found")
)

const (
	MaxBatchItems   = 500
	MaxSubjectBytes = 128
	MaxPayloadBytes = 32 * 1024
	FutureSkew      = 5 * time.Minute
	LateWindow      = 30 * 24 * time.Hour
	IdempotencyTTL  = 24 * time.Hour
)

const (
	ItemAccepted  = "accepted"
	ItemDuplicate = "duplicate"
	ItemRejected  = "rejected"
)

type EventType struct {
	Key             string
	Name            string
	Description     *string
	SchemaVersion   int
	RequireExposure bool
	Status          string
}

func (t EventType) Active() bool {
	return t.Status == "active"
}

type SubjectRef struct {
	Value       string
	SaltVersion string
}

func ResolveSubject(subject, saltVersion, salt string, hash bool) (SubjectRef, error) {
	if len(subject) == 0 || len([]byte(subject)) > MaxSubjectBytes {
		return SubjectRef{}, ErrSubjectInvalid
	}

	if !hash {
		return SubjectRef{Value: subject}, nil
	}

	if salt == "" {
		return SubjectRef{}, ErrPIISaltMissing
	}

	sum := sha256.Sum256([]byte(salt + ":" + subject))

	return SubjectRef{Value: hex.EncodeToString(sum[:]), SaltVersion: saltVersion}, nil
}

type ValidatedEvent struct {
	EventID       string
	EventType     string
	Subject       SubjectRef
	OccurredAt    time.Time
	ReceivedAt    time.Time
	DecisionID    string
	SchemaVersion int
	Payload       json.RawMessage
}

type ValidatedExposure struct {
	EventID             string
	DecisionID          string
	Subject             SubjectRef
	ExperimentID        string
	ExperimentVersionID string
	VariantID           string
	FlagID              string
	OccurredAt          time.Time
	ReceivedAt          time.Time
}

type KafkaEventPayload struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SubjectID     string          `json:"subject_id"`
	SaltVersion   string          `json:"salt_version,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	ReceivedAt    time.Time       `json:"received_at"`
	DecisionID    string          `json:"decision_id,omitempty"`
	SchemaVersion int             `json:"schema_version"`
	Payload       json.RawMessage `json:"payload"`
}

type KafkaExposurePayload struct {
	EventID             string    `json:"event_id"`
	DecisionID          string    `json:"decision_id"`
	SubjectID           string    `json:"subject_id"`
	SaltVersion         string    `json:"salt_version,omitempty"`
	ExperimentID        string    `json:"experiment_id"`
	ExperimentVersionID string    `json:"experiment_version_id"`
	VariantID           string    `json:"variant_id"`
	FlagID              string    `json:"flag_id"`
	OccurredAt          time.Time `json:"occurred_at"`
	ReceivedAt          time.Time `json:"received_at"`
}

func CheckEventID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrEventIDInvalid
	}

	return nil
}

func CheckReferenceID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrExposureFieldInvalid
	}

	return nil
}

func CheckDecisionID(id string) error {
	if id == "" {
		return nil
	}

	if _, err := uuid.Parse(id); err != nil {
		return ErrDecisionIDInvalid
	}

	return nil
}

func CheckOccurredAt(ts, now time.Time) error {
	if ts.After(now.Add(FutureSkew)) || ts.Before(now.Add(-LateWindow)) {
		return ErrOccurredAtOutOfRange
	}

	return nil
}

func CanonicalPayload(raw json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, ErrPayloadInvalid
	}

	var v any

	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, ErrPayloadInvalid
	}

	out, err := json.Marshal(v)
	if err != nil {
		return nil, ErrPayloadInvalid
	}

	if len(out) > MaxPayloadBytes {
		return nil, ErrPayloadTooLarge
	}

	return out, nil
}

func EventPartitionKey(decisionID, eventID string) string {
	if decisionID != "" {
		return decisionID
	}

	return eventID
}

type eventHashInput struct {
	ID       string
	Type     string
	Subject  string
	Occurred int64
	Decision string
	Schema   int
	Payload  json.RawMessage
}

type exposureHashInput struct {
	Event      string
	Decision   string
	Subject    string
	Experiment string
	Version    string
	Variant    string
	Flag       string
	Occurred   int64
}

func RequestHashEvent(item EventItem) ([]byte, error) {
	payload, err := CanonicalPayload(item.Payload)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(eventHashInput{
		ID:       item.EventID,
		Type:     item.EventType,
		Subject:  item.SubjectID,
		Occurred: item.OccurredAt.UTC().UnixNano(),
		Decision: item.DecisionIDValue(),
		Schema:   item.SchemaVersionValue(),
		Payload:  payload,
	})
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(raw)

	return sum[:], nil
}

func RequestHashExposure(item ExposureItem) ([]byte, error) {
	raw, err := json.Marshal(exposureHashInput{
		Event:      item.EventID,
		Decision:   item.DecisionID,
		Subject:    item.SubjectID,
		Experiment: item.ExperimentID,
		Version:    item.ExperimentVersionID,
		Variant:    item.VariantID,
		Flag:       item.FlagID,
		Occurred:   item.OccurredAt.UTC().UnixNano(),
	})
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(raw)

	return sum[:], nil
}
