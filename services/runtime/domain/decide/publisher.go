package decide

import (
	"context"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const DecisionEnvelopeType = "decision"

type DecisionRecord struct {
	DecisionID          string    `json:"decision_id"`
	RequestID           string    `json:"request_id"`
	SubjectID           string    `json:"subject_id"`
	FlagKey             string    `json:"flag_key"`
	ExperimentID        string    `json:"experiment_id"`
	ExperimentVersionID string    `json:"experiment_version_id"`
	VariantID           string    `json:"variant_id"`
	ResultSource        string    `json:"result_source"`
	ConfigRevision      string    `json:"config_revision"`
	CreatedAt           time.Time `json:"created_at"`
}

type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type DecisionPublisher struct {
	ch     chan DecisionRecord
	writer MessageWriter
	log    *zap.Logger
	wg     sync.WaitGroup
}

func NewDecisionPublisher(writer MessageWriter, bufferSize int, log *zap.Logger) *DecisionPublisher {
	if bufferSize <= 0 {
		bufferSize = 1024
	}

	p := &DecisionPublisher{
		ch:     make(chan DecisionRecord, bufferSize),
		writer: writer,
		log:    log.Named("decision_publisher"),
	}

	p.wg.Add(1)
	go p.run()

	return p
}

func (p *DecisionPublisher) Publish(record DecisionRecord) {
	select {
	case p.ch <- record:
	default:
		p.log.Warn("decision publish channel full, dropping record",
			zap.String(logger.FieldDecisionID, record.DecisionID),
			zap.String(logger.FieldFlagKey, record.FlagKey),
		)
	}
}

func (p *DecisionPublisher) Stop() {
	close(p.ch)
	p.wg.Wait()
}

func (p *DecisionPublisher) run() {
	defer p.wg.Done()

	for record := range p.ch {
		if err := p.write(context.Background(), record); err != nil {
			p.log.Warn("failed to publish decision",
				zap.String(logger.FieldDecisionID, record.DecisionID),
				zap.Error(err),
			)
		}
	}
}

func (p *DecisionPublisher) write(ctx context.Context, record DecisionRecord) error {
	if p.writer == nil {
		return nil
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	env := outbox.Envelope{
		ID:     record.DecisionID,
		Type:   DecisionEnvelopeType,
		Source: "runtime",
		Time:   record.CreatedAt,
		Data:   payload,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	key, err := uuid.Parse(record.DecisionID)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   key[:],
		Value: envBytes,
	})
}
