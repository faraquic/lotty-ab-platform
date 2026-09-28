package clickhouse

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type fakeBatch struct {
	query    string
	appended [][]any
	sendErr  error
	sent     bool
	aborted  bool
}

func (b *fakeBatch) Append(v ...any) error {
	b.appended = append(b.appended, v)
	return nil
}

func (b *fakeBatch) Send() error {
	if b.sendErr != nil {
		return b.sendErr
	}

	b.sent = true

	return nil
}

func (b *fakeBatch) Abort() error {
	b.aborted = true
	return nil
}

type fakeConn struct {
	batches    []*fakeBatch
	prepareErr error
	sendErr    error
}

func (c *fakeConn) PrepareBatch(_ context.Context, query string) (Batch, error) {
	if c.prepareErr != nil {
		return nil, c.prepareErr
	}

	b := &fakeBatch{query: query, sendErr: c.sendErr}
	c.batches = append(c.batches, b)

	return b, nil
}

func testEventRow() EventRow {
	return EventRow{
		EventID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EventType:     "conversion",
		SubjectID:     "abc",
		SaltVersion:   "v1",
		OccurredAt:    time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
		ReceivedAt:    time.Date(2026, 9, 27, 12, 0, 1, 0, time.UTC),
		DecisionID:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Payload:       `{"sku":"a"}`,
		SchemaVersion: 3,
	}
}

func TestWriterFlushEvents(t *testing.T) {
	conn := &fakeConn{}
	w := NewWriter(conn, 1000, zap.NewNop())
	w.AppendEvent(testEventRow())

	if got := w.Pending(); got != 1 {
		t.Fatalf("Pending = %d, want 1", got)
	}

	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if got := w.Pending(); got != 0 {
		t.Errorf("Pending after flush = %d, want 0", got)
	}

	if len(conn.batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(conn.batches))
	}

	b := conn.batches[0]

	if !b.sent {
		t.Error("batch was not sent")
	}

	if len(b.appended) != 1 {
		t.Fatalf("appended rows = %d, want 1", len(b.appended))
	}

	row := b.appended[0]

	if row[0] != testEventRow().EventID {
		t.Errorf("event_id = %v", row[0])
	}

	if row[1] != "conversion" || row[7] != `{"sku":"a"}` || row[8] != uint16(3) {
		t.Errorf("row = %v", row)
	}
}

func TestWriterFlushEmpty(t *testing.T) {
	conn := &fakeConn{}
	w := NewWriter(conn, 1000, zap.NewNop())

	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if len(conn.batches) != 0 {
		t.Errorf("batches = %d, want 0", len(conn.batches))
	}
}

func TestWriterPrepareErrorRequeues(t *testing.T) {
	conn := &fakeConn{prepareErr: errors.New("boom")}
	w := NewWriter(conn, 1000, zap.NewNop())
	w.AppendEvent(testEventRow())

	if err := w.Flush(context.Background()); err == nil {
		t.Fatal("Flush = nil, want error")
	}

	if got := w.Pending(); got != 1 {
		t.Errorf("Pending = %d, want 1 requeued", got)
	}
}

func TestWriterSendErrorRequeues(t *testing.T) {
	conn := &fakeConn{sendErr: errors.New("boom")}
	w := NewWriter(conn, 1000, zap.NewNop())
	w.AppendEvent(testEventRow())

	if err := w.Flush(context.Background()); err == nil {
		t.Fatal("Flush = nil, want error")
	}

	if got := w.Pending(); got != 1 {
		t.Errorf("Pending = %d, want 1 requeued", got)
	}
}

func TestWriterFull(t *testing.T) {
	w := NewWriter(&fakeConn{}, 2, zap.NewNop())

	if w.Full() {
		t.Error("empty writer is full")
	}

	w.AppendEvent(testEventRow())
	w.AppendExposure(ExposureRow{})

	if !w.Full() {
		t.Error("writer with 2 rows and batch size 2 is not full")
	}
}

func TestWriterDefaultBatchSize(t *testing.T) {
	w := NewWriter(&fakeConn{}, 0, zap.NewNop())

	if w.BatchSize() != 1000 {
		t.Errorf("BatchSize = %d, want 1000", w.BatchSize())
	}
}
