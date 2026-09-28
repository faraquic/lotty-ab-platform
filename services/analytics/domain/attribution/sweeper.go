package attribution

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"go.uber.org/zap"
)

type Sweeper struct {
	store  PendingStore
	writer AttrWriter
	now    func() time.Time
	log    *zap.Logger
}

func NewSweeper(
	store PendingStore,
	writer AttrWriter,
	now func() time.Time,
	log *zap.Logger,
) *Sweeper {
	return &Sweeper{
		store:  store,
		writer: writer,
		now:    now,
		log:    log.Named("sweeper"),
	}
}

func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.sweep(ctx); err != nil {
					s.log.Warn("sweep failed", zap.Error(err))
				}
			}
		}
	}()
}

func (s *Sweeper) sweep(ctx context.Context) error {
	now := s.now()
	expired, err := s.store.ExpiredConversions(ctx, now)
	if err != nil {
		return fmt.Errorf("get expired conversions: %w", err)
	}

	for _, decisionID := range expired {
		if err := s.expireConversion(ctx, decisionID, now); err != nil {
			s.log.Warn("failed to expire conversion",
				zap.String(logger.FieldDecisionID, decisionID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (s *Sweeper) expireConversion(ctx context.Context, decisionID string, now time.Time) error {
	pc, err := s.store.GetConversion(ctx, decisionID)
	if err != nil {
		return fmt.Errorf("get pending conversion: %w", err)
	}

	if pc == nil {
		_ = s.store.DeleteConversion(ctx, decisionID)
		return nil
	}

	decision, err := uuid.Parse(decisionID)
	if err != nil {
		return fmt.Errorf("parse decision_id: %w", err)
	}

	eventID, err := uuid.Parse(pc.EventID)
	if err != nil {
		return fmt.Errorf("parse event_id: %w", err)
	}

	s.writer.WriteAttributed(ctx, clickhouse.AttributedEventRow{
		EventID:      eventID,
		DecisionID:   decision,
		ExperimentID: uuid.Nil,
		VariantID:    uuid.Nil,
		SubjectID:    pc.SubjectID,
		SaltVersion:  pc.SaltVersion,
		EventType:    pc.EventType,
		OccurredAt:   pc.OccurredAt.UTC(),
		ReceivedAt:   now.UTC(),
	})

	if err := s.store.DeleteConversion(ctx, decisionID); err != nil {
		return fmt.Errorf("delete pending conversion: %w", err)
	}

	return nil
}
