package outbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Message is a single outbox entry. Topic and Type are required, Payload is
// marshaled to JSON and wrapped into an Envelope at insert time.
type Message struct {
	Topic   string
	Key     string
	Type    string
	Payload any
	Headers map[string]string
	Source  string
}

// Envelope is the on-the-wire format published to Kafka. ID carries the
// outbox row id so consumers can deduplicate (at-least-once delivery).
type Envelope struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Source string          `json:"source"`
	Time   time.Time       `json:"time"`
	Data   json.RawMessage `json:"data"`
}

// CreateMessage inserts msg into the outbox within the caller's transaction.
// The caller owns the transaction and must commit it for the message to
// become visible to the publisher. Payload must be JSON-marshalable.
func CreateMessage(ctx context.Context, tx pgx.Tx, msg Message) error {
	if strings.TrimSpace(msg.Topic) == "" {
		return errors.New("outbox topic is empty")
	}

	if strings.TrimSpace(msg.Type) == "" {
		return errors.New("outbox type is empty")
	}

	if msg.Payload == nil {
		return errors.New("outbox payload is nil")
	}

	data, err := json.Marshal(msg.Payload)
	if err != nil {
		return err
	}

	source := msg.Source
	if strings.TrimSpace(source) == "" {
		source = config.ServiceName
	}

	entryID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	envelope, err := json.Marshal(Envelope{
		ID:     entryID.String(),
		Type:   msg.Type,
		Source: source,
		Time:   time.Now().UTC(),
		Data:   data,
	})
	if err != nil {
		return err
	}

	var headers any
	if len(msg.Headers) > 0 {
		headers = msg.Headers
	}

	var key any
	if value := strings.TrimSpace(msg.Key); value != "" {
		key = value
	}

	_, err = tx.Exec(
		ctx, `
INSERT INTO outbox (id, topic, key, type, payload, headers)
VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb)`,
		entryID.String(), msg.Topic, key, msg.Type, envelope, headers,
	)

	return err
}
