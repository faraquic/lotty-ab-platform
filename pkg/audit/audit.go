package audit

import (
	"context"
	"strings"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Metadata struct {
	RequestID string
	TraceID   string
}

type contextKey struct{}

type Record struct {
	ActorType    string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Reason       string
	Before       any
	After        any
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, contextKey{}, metadata)
}

func Append(ctx context.Context, tx pgx.Tx, record Record) error {
	metadata, _ := ctx.Value(contextKey{}).(Metadata)
	var requestID any
	if id, err := uuid.Parse(metadata.RequestID); err == nil {
		requestID = id
	}
	var traceID any
	if value := strings.TrimSpace(metadata.TraceID); value != "" {
		traceID = value
	}
	var reason any
	if value := strings.TrimSpace(record.Reason); value != "" {
		reason = value
	}
	var before, after any
	var err error
	if record.Before != nil {
		before, err = json.Marshal(record.Before)
		if err != nil {
			return err
		}
	}
	if record.After != nil {
		after, err = json.Marshal(record.After)
		if err != nil {
			return err
		}
	}
	entryID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		ctx, `
INSERT INTO audit_records (
	id, actor_type, actor_id, action, resource_type, resource_id,
	reason, before_state, after_state, request_id, trace_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11)`,
		entryID, record.ActorType, record.ActorID, record.Action,
		record.ResourceType, record.ResourceID, reason, before, after, requestID, traceID,
	)
	return err
}
