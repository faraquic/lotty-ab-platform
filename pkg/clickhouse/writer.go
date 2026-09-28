package clickhouse

import (
	"context"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	insertEvents     = `INSERT INTO events_raw (event_id, event_type, subject_id, salt_version, occurred_at, received_at, decision_id, payload, schema_version) VALUES`
	insertExposures  = `INSERT INTO exposures (event_id, decision_id, subject_id, salt_version, experiment_id, experiment_version_id, variant_id, flag_id, occurred_at, received_at) VALUES`
	insertDecisions  = `INSERT INTO decisions_raw (decision_id, request_id, subject_id, salt_version, flag_key, experiment_id, experiment_version_id, variant_id, result_source, config_revision, created_at) VALUES`
	insertAttributed = `INSERT INTO attributed_events (event_id, decision_id, experiment_id, variant_id, subject_id, salt_version, event_type, occurred_at, received_at) VALUES`
)

type EventRow struct {
	EventID       uuid.UUID
	EventType     string
	SubjectID     string
	SaltVersion   string
	OccurredAt    time.Time
	ReceivedAt    time.Time
	DecisionID    uuid.UUID
	Payload       string
	SchemaVersion uint16
}

type ExposureRow struct {
	EventID             uuid.UUID
	DecisionID          uuid.UUID
	SubjectID           string
	SaltVersion         string
	ExperimentID        uuid.UUID
	ExperimentVersionID uuid.UUID
	VariantID           uuid.UUID
	FlagID              uuid.UUID
	OccurredAt          time.Time
	ReceivedAt          time.Time
}

type DecisionRow struct {
	DecisionID          uuid.UUID
	RequestID           uuid.UUID
	SubjectID           string
	SaltVersion         string
	FlagKey             string
	ExperimentID        uuid.UUID
	ExperimentVersionID uuid.UUID
	VariantID           uuid.UUID
	ResultSource        string
	ConfigRevision      uint64
	CreatedAt           time.Time
}

type AttributedEventRow struct {
	EventID      uuid.UUID
	DecisionID   uuid.UUID
	ExperimentID uuid.UUID
	VariantID    uuid.UUID
	SubjectID    string
	SaltVersion  string
	EventType    string
	OccurredAt   time.Time
	ReceivedAt   time.Time
}

type Batch interface {
	Append(v ...any) error
	Send() error
	Abort() error
}

type Conn interface {
	PrepareBatch(ctx context.Context, query string) (Batch, error)
}

type nativeBatch struct {
	batch driver.Batch
}

func (b nativeBatch) Append(v ...any) error { return b.batch.Append(v...) }
func (b nativeBatch) Send() error           { return b.batch.Send() }
func (b nativeBatch) Abort() error          { return b.batch.Abort() }

type nativeConn struct {
	conn clickhouse.Conn
}

func (n nativeConn) PrepareBatch(ctx context.Context, query string) (Batch, error) {
	b, err := n.conn.PrepareBatch(ctx, query)
	if err != nil {
		return nil, err
	}

	return nativeBatch{batch: b}, nil
}

func WrapConn(conn clickhouse.Conn) Conn {
	return nativeConn{conn: conn}
}

type Writer struct {
	conn       Conn
	log        *zap.Logger
	batchSize  int
	mu         sync.Mutex
	events     []EventRow
	exposures  []ExposureRow
	decisions  []DecisionRow
	attributed []AttributedEventRow
}

func NewWriter(conn Conn, batchSize int, log *zap.Logger) *Writer {
	if batchSize <= 0 {
		batchSize = 1000
	}

	return &Writer{
		conn:      conn,
		log:       log.Named("clickhouse_writer"),
		batchSize: batchSize,
	}
}

func (w *Writer) BatchSize() int {
	return w.batchSize
}

func (w *Writer) AppendEvent(r EventRow) {
	w.mu.Lock()
	w.events = append(w.events, r)
	w.mu.Unlock()
}

func (w *Writer) AppendExposure(r ExposureRow) {
	w.mu.Lock()
	w.exposures = append(w.exposures, r)
	w.mu.Unlock()
}

func (w *Writer) AppendDecision(r DecisionRow) {
	w.mu.Lock()
	w.decisions = append(w.decisions, r)
	w.mu.Unlock()
}

func (w *Writer) AppendAttributedEvent(r AttributedEventRow) {
	w.mu.Lock()
	w.attributed = append(w.attributed, r)
	w.mu.Unlock()
}

func (w *Writer) Pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return len(w.events) + len(w.exposures) + len(w.decisions) + len(w.attributed)
}

func (w *Writer) Full() bool {
	return w.Pending() >= w.batchSize
}

func (w *Writer) Flush(ctx context.Context) error {
	if err := w.flushEvents(ctx); err != nil {
		return err
	}

	if err := w.flushExposures(ctx); err != nil {
		return err
	}

	if err := w.flushDecisions(ctx); err != nil {
		return err
	}

	return w.flushAttributed(ctx)
}

func (w *Writer) flushEvents(ctx context.Context) error {
	w.mu.Lock()
	rows := w.events
	w.events = nil
	w.mu.Unlock()

	if len(rows) == 0 {
		return nil
	}

	if err := w.sendEvents(ctx, rows); err != nil {
		w.mu.Lock()
		w.events = append(rows, w.events...)
		w.mu.Unlock()

		return err
	}

	return nil
}

func (w *Writer) flushExposures(ctx context.Context) error {
	w.mu.Lock()
	rows := w.exposures
	w.exposures = nil
	w.mu.Unlock()

	if len(rows) == 0 {
		return nil
	}

	if err := w.sendExposures(ctx, rows); err != nil {
		w.mu.Lock()
		w.exposures = append(rows, w.exposures...)
		w.mu.Unlock()

		return err
	}

	return nil
}

func (w *Writer) flushDecisions(ctx context.Context) error {
	w.mu.Lock()
	rows := w.decisions
	w.decisions = nil
	w.mu.Unlock()

	if len(rows) == 0 {
		return nil
	}

	if err := w.sendDecisions(ctx, rows); err != nil {
		w.mu.Lock()
		w.decisions = append(rows, w.decisions...)
		w.mu.Unlock()

		return err
	}

	return nil
}

func (w *Writer) sendEvents(ctx context.Context, rows []EventRow) error {
	batch, err := w.conn.PrepareBatch(ctx, insertEvents)
	if err != nil {
		return err
	}

	for _, r := range rows {
		if err := batch.Append(
			r.EventID,
			r.EventType,
			r.SubjectID,
			r.SaltVersion,
			r.OccurredAt,
			r.ReceivedAt,
			r.DecisionID,
			r.Payload,
			r.SchemaVersion,
		); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}

func (w *Writer) sendExposures(ctx context.Context, rows []ExposureRow) error {
	batch, err := w.conn.PrepareBatch(ctx, insertExposures)
	if err != nil {
		return err
	}

	for _, r := range rows {
		if err := batch.Append(
			r.EventID,
			r.DecisionID,
			r.SubjectID,
			r.SaltVersion,
			r.ExperimentID,
			r.ExperimentVersionID,
			r.VariantID,
			r.FlagID,
			r.OccurredAt,
			r.ReceivedAt,
		); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}

func (w *Writer) sendDecisions(ctx context.Context, rows []DecisionRow) error {
	batch, err := w.conn.PrepareBatch(ctx, insertDecisions)
	if err != nil {
		return err
	}

	for _, r := range rows {
		if err := batch.Append(
			r.DecisionID,
			r.RequestID,
			r.SubjectID,
			r.SaltVersion,
			r.FlagKey,
			r.ExperimentID,
			r.ExperimentVersionID,
			r.VariantID,
			r.ResultSource,
			r.ConfigRevision,
			r.CreatedAt,
		); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}

func (w *Writer) flushAttributed(ctx context.Context) error {
	w.mu.Lock()
	rows := w.attributed
	w.attributed = nil
	w.mu.Unlock()

	if len(rows) == 0 {
		return nil
	}

	if err := w.sendAttributed(ctx, rows); err != nil {
		w.mu.Lock()
		w.attributed = append(rows, w.attributed...)
		w.mu.Unlock()

		return err
	}

	return nil
}

func (w *Writer) sendAttributed(ctx context.Context, rows []AttributedEventRow) error {
	batch, err := w.conn.PrepareBatch(ctx, insertAttributed)
	if err != nil {
		return err
	}

	for _, r := range rows {
		if err := batch.Append(
			r.EventID,
			r.DecisionID,
			r.ExperimentID,
			r.VariantID,
			r.SubjectID,
			r.SaltVersion,
			r.EventType,
			r.OccurredAt,
			r.ReceivedAt,
		); err != nil {
			_ = batch.Abort()
			return err
		}
	}

	return batch.Send()
}
