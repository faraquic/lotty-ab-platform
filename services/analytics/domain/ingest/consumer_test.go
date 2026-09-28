package ingest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	pkgclickhouse "github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type fakeFetcher struct {
	mu       sync.Mutex
	messages []kafka.Message
	commits  [][]kafka.Message
	fetchErr error
}

func (f *fakeFetcher) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if f.fetchErr != nil {
		return kafka.Message{}, f.fetchErr
	}

	for {
		f.mu.Lock()

		if len(f.messages) > 0 {
			msg := f.messages[0]
			f.messages = f.messages[1:]
			f.mu.Unlock()

			return msg, nil
		}

		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return kafka.Message{}, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func (f *fakeFetcher) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append(f.commits, msgs)

	return nil
}

func (f *fakeFetcher) committedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0
	for _, c := range f.commits {
		n += len(c)
	}

	return n
}

type fakeBatchConn struct {
	mu      sync.Mutex
	batches int
	sendErr error
}

func (c *fakeBatchConn) PrepareBatch(_ context.Context, _ string) (pkgclickhouse.Batch, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.batches++

	return &fakeBatch{sendErr: c.sendErr}, nil
}

type fakeBatch struct {
	sendErr error
}

func (b *fakeBatch) Append(...any) error { return nil }
func (b *fakeBatch) Send() error         { return b.sendErr }
func (b *fakeBatch) Abort() error        { return nil }

func kafkaRecord(t *testing.T, id, typ string, payload map[string]any) kafka.Message {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	env, err := json.Marshal(outbox.Envelope{ID: id, Type: typ, Source: "labp-analytics", Time: time.Now().UTC(), Data: data})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	return kafka.Message{Topic: "events", Value: env}
}

func eventPayload(id string) map[string]any {
	return map[string]any{
		"event_id":    id,
		"event_type":  "conversion",
		"subject_id":  "abc",
		"occurred_at": "2026-09-27T12:00:00Z",
		"received_at": "2026-09-27T12:00:01Z",
		"decision_id": uuid.NewString(),
		"payload":     map[string]any{"sku": "a"},
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, what string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

func TestConsumerFlushesFullBatchAndCommits(t *testing.T) {
	conn := &fakeBatchConn{}
	writer := pkgclickhouse.NewWriter(conn, 2, zap.NewNop())
	fetcher := &fakeFetcher{}

	consumer := NewTopicConsumer(fetcher, writer, RowKindEvent, "events", time.Hour, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = consumer.Run(ctx)
	}()

	id1, id2 := uuid.NewString(), uuid.NewString()
	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages,
		kafkaRecord(t, id1, "event", eventPayload(id1)),
		kafkaRecord(t, id2, "event", eventPayload(id2)),
	)
	fetcher.mu.Unlock()

	waitFor(t, 5*time.Second, func() bool { return fetcher.committedCount() == 2 }, "2 commits")

	cancel()
	<-done

	if writer.Pending() != 0 {
		t.Errorf("pending = %d, want 0", writer.Pending())
	}
}

func TestConsumerTickerFlushCommits(t *testing.T) {
	conn := &fakeBatchConn{}
	writer := pkgclickhouse.NewWriter(conn, 1000, zap.NewNop())
	fetcher := &fakeFetcher{}

	consumer := NewTopicConsumer(fetcher, writer, RowKindEvent, "events", 10*time.Millisecond, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = consumer.Run(ctx)
	}()

	id := uuid.NewString()
	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafkaRecord(t, id, "event", eventPayload(id)))
	fetcher.mu.Unlock()

	waitFor(t, 5*time.Second, func() bool { return fetcher.committedCount() == 1 }, "ticker commit")

	cancel()
	<-done
}

func TestConsumerPoisonCommittedWithoutFlush(t *testing.T) {
	conn := &fakeBatchConn{}
	writer := pkgclickhouse.NewWriter(conn, 1000, zap.NewNop())
	fetcher := &fakeFetcher{}

	consumer := NewTopicConsumer(fetcher, writer, RowKindEvent, "events", time.Hour, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = consumer.Run(ctx)
	}()

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages,
		kafka.Message{Topic: "events", Value: []byte(`{broken`)},
		kafkaRecord(t, uuid.NewString(), "nope", map[string]any{}),
	)
	fetcher.mu.Unlock()

	waitFor(t, 5*time.Second, func() bool { return fetcher.committedCount() == 2 }, "poison commits")

	cancel()
	<-done

	if writer.Pending() != 0 {
		t.Errorf("pending = %d, want 0 (poison not buffered)", writer.Pending())
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.batches != 0 {
		t.Errorf("batches = %d, want 0", conn.batches)
	}
}

func TestConsumerFlushFailureBlocksCommit(t *testing.T) {
	conn := &fakeBatchConn{sendErr: errors.New("clickhouse down")}
	writer := pkgclickhouse.NewWriter(conn, 1, zap.NewNop())
	fetcher := &fakeFetcher{}

	consumer := NewTopicConsumer(fetcher, writer, RowKindEvent, "events", time.Hour, zap.NewNop())
	consumer.retryInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = consumer.Run(ctx)
	}()

	id := uuid.NewString()
	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafkaRecord(t, id, "event", eventPayload(id)))
	fetcher.mu.Unlock()

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	if got := fetcher.committedCount(); got != 0 {
		t.Errorf("committed = %d, want 0 while flush fails", got)
	}

	if got := writer.Pending(); got != 1 {
		t.Errorf("pending = %d, want 1 retained", got)
	}
}
