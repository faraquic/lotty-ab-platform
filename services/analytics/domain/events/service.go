package events

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/database"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"go.uber.org/zap"
)

type eventTypeStore interface {
	ListActiveEventTypes(ctx context.Context) ([]EventType, error)
}

type idempotencyStore interface {
	FindIdempotencyKey(ctx context.Context, eventID string) (IdempotencyRecord, error)
	InsertIdempotencyKey(ctx context.Context, rec IdempotencyRecord) error
	ReplaceIdempotencyKey(ctx context.Context, rec IdempotencyRecord) error
	DeleteIdempotencyKeys(ctx context.Context, eventIDs []string) error
}

type recordPublisher interface {
	Publish(ctx context.Context, records []Record) error
}

type Service struct {
	repo           eventTypeStore
	claims         idempotencyStore
	producer       recordPublisher
	eventsTopic    string
	exposuresTopic string
	pii            config.PIIConfig
	log            *zap.Logger
	now            func() time.Time
}

func NewService(repo *Repository, producer *Producer, cfg *config.Config, log *zap.Logger) *Service {
	return &Service{
		repo:           repo,
		claims:         repo,
		producer:       producer,
		eventsTopic:    cfg.Analytics.Kafka.EventsTopic,
		exposuresTopic: cfg.Analytics.Kafka.ExposuresTopic,
		pii:            cfg.Analytics.PII,
		log:            log.Named("events"),
		now:            time.Now,
	}
}

func (s *Service) ListEventTypes(ctx context.Context) ([]EventType, error) {
	return s.repo.ListActiveEventTypes(ctx)
}

func (s *Service) IngestEvents(ctx context.Context, req EventBatchRequest) (BatchData, error) {
	types, err := s.activeTypes(ctx)
	if err != nil {
		return BatchData{}, err
	}

	now := s.now().UTC()
	results := make([]ItemResult, 0, len(req.Events))
	pending := make([]pendingEvent, 0, len(req.Events))

	for _, item := range req.Events {
		validated, hash, res, ok := s.checkEvent(item, types, now)
		if !ok {
			results = append(results, res)
			continue
		}

		claim, res, err := s.claim(ctx, item.EventID, hash, now)
		if err != nil {
			return BatchData{}, err
		}

		results = append(results, res)

		if claim {
			pending = append(pending, pendingEvent{item: item, validated: validated, hash: hash})
		}
	}

	if err := s.publishEvents(ctx, pending); err != nil {
		s.release(ctx, pendingEventIDs(pending))
		return BatchData{}, err
	}

	return summarize(results), nil
}

func (s *Service) IngestExposures(ctx context.Context, req ExposureBatchRequest) (BatchData, error) {
	now := s.now().UTC()
	results := make([]ItemResult, 0, len(req.Exposures))
	pending := make([]pendingExposure, 0, len(req.Exposures))

	for _, item := range req.Exposures {
		validated, res, ok := s.checkExposure(item, now)
		if !ok {
			results = append(results, res)
			continue
		}

		hash, err := RequestHashExposure(item)
		if err != nil {
			return BatchData{}, err
		}

		claim, res, err := s.claim(ctx, item.EventID, hash, now)
		if err != nil {
			return BatchData{}, err
		}

		results = append(results, res)

		if claim {
			pending = append(pending, pendingExposure{item: item, validated: validated, hash: hash})
		}
	}

	if err := s.publishExposures(ctx, pending); err != nil {
		s.release(ctx, pendingExposureIDs(pending))
		return BatchData{}, err
	}

	return summarize(results), nil
}

type pendingEvent struct {
	item      EventItem
	validated ValidatedEvent
	hash      []byte
}

type pendingExposure struct {
	item      ExposureItem
	validated ValidatedExposure
	hash      []byte
}

func pendingEventIDs(pending []pendingEvent) []string {
	ids := make([]string, 0, len(pending))
	for _, p := range pending {
		ids = append(ids, p.validated.EventID)
	}

	return ids
}

func pendingExposureIDs(pending []pendingExposure) []string {
	ids := make([]string, 0, len(pending))
	for _, p := range pending {
		ids = append(ids, p.validated.EventID)
	}

	return ids
}

func (s *Service) activeTypes(ctx context.Context) (map[string]EventType, error) {
	list, err := s.repo.ListActiveEventTypes(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]EventType, len(list))
	for _, t := range list {
		out[t.Key] = t
	}

	return out, nil
}

func (s *Service) checkEvent(item EventItem, types map[string]EventType, now time.Time) (ValidatedEvent, []byte, ItemResult, bool) {
	fail := func(err error) (ValidatedEvent, []byte, ItemResult, bool) {
		msg := err.Error()
		return ValidatedEvent{}, nil, ItemResult{EventID: item.EventID, Status: ItemRejected, Error: &msg}, false
	}

	if err := CheckEventID(item.EventID); err != nil {
		return fail(err)
	}

	typ, ok := types[item.EventType]
	if !ok {
		return fail(ErrUnknownEventType)
	}

	if !typ.Active() {
		return fail(ErrEventTypeArchived)
	}

	if err := CheckDecisionID(item.DecisionIDValue()); err != nil {
		return fail(err)
	}

	if err := CheckOccurredAt(item.OccurredAt, now); err != nil {
		return fail(err)
	}

	payload, err := CanonicalPayload(item.Payload)
	if err != nil {
		return fail(err)
	}

	subject, err := s.resolveSubject(item.SubjectID)
	if err != nil {
		return fail(err)
	}

	schema := item.SchemaVersionValue()
	if schema == 0 {
		schema = typ.SchemaVersion
	}

	hash, err := RequestHashEvent(EventItem{
		EventID:       item.EventID,
		EventType:     item.EventType,
		SubjectID:     item.SubjectID,
		OccurredAt:    item.OccurredAt,
		DecisionID:    item.DecisionID,
		SchemaVersion: &schema,
		Payload:       payload,
	})
	if err != nil {
		return fail(err)
	}

	validated := ValidatedEvent{
		EventID:       item.EventID,
		EventType:     item.EventType,
		Subject:       subject,
		OccurredAt:    item.OccurredAt.UTC(),
		ReceivedAt:    now,
		DecisionID:    item.DecisionIDValue(),
		SchemaVersion: schema,
		Payload:       payload,
	}

	return validated, hash, ItemResult{EventID: item.EventID, Status: ItemAccepted}, true
}

func (s *Service) checkExposure(item ExposureItem, now time.Time) (ValidatedExposure, ItemResult, bool) {
	fail := func(err error) (ValidatedExposure, ItemResult, bool) {
		msg := err.Error()
		return ValidatedExposure{}, ItemResult{EventID: item.EventID, Status: ItemRejected, Error: &msg}, false
	}

	if err := CheckEventID(item.EventID); err != nil {
		return fail(err)
	}

	for _, id := range []string{item.DecisionID, item.ExperimentID, item.ExperimentVersionID, item.VariantID, item.FlagID} {
		if err := CheckReferenceID(id); err != nil {
			return fail(err)
		}
	}

	if err := CheckOccurredAt(item.OccurredAt, now); err != nil {
		return fail(err)
	}

	subject, err := s.resolveSubject(item.SubjectID)
	if err != nil {
		return fail(err)
	}

	validated := ValidatedExposure{
		EventID:             item.EventID,
		DecisionID:          item.DecisionID,
		Subject:             subject,
		ExperimentID:        item.ExperimentID,
		ExperimentVersionID: item.ExperimentVersionID,
		VariantID:           item.VariantID,
		FlagID:              item.FlagID,
		OccurredAt:          item.OccurredAt.UTC(),
		ReceivedAt:          now,
	}

	return validated, ItemResult{EventID: item.EventID, Status: ItemAccepted}, true
}

func (s *Service) resolveSubject(subject string) (SubjectRef, error) {
	if !s.pii.HashSubjectID {
		return ResolveSubject(subject, "", "", false)
	}

	return ResolveSubject(subject, s.pii.CurrentSaltVersion, s.pii.Salts[s.pii.CurrentSaltVersion], true)
}

func (s *Service) claim(ctx context.Context, eventID string, hash []byte, now time.Time) (bool, ItemResult, error) {
	rec := IdempotencyRecord{
		EventID:     eventID,
		RequestHash: hash,
		Response:    AcceptedResponse(),
		ExpiresAt:   now.Add(IdempotencyTTL),
	}

	if err := s.claims.InsertIdempotencyKey(ctx, rec); err == nil {
		return true, ItemResult{EventID: eventID, Status: ItemAccepted}, nil
	} else if !database.IsUniqueViolation(err) {
		return false, ItemResult{}, err
	}

	existing, err := s.claims.FindIdempotencyKey(ctx, eventID)
	if err != nil {
		return false, ItemResult{}, err
	}

	if existing.Expired(s.now()) {
		if err := s.claims.ReplaceIdempotencyKey(ctx, rec); err != nil {
			if errors.Is(err, ErrIdempotencyNotFound) {
				return s.claim(ctx, eventID, hash, now)
			}

			return false, ItemResult{}, err
		}

		return true, ItemResult{EventID: eventID, Status: ItemAccepted}, nil
	}

	if bytes.Equal(existing.RequestHash, hash) {
		return false, ItemResult{EventID: eventID, Status: ItemDuplicate}, nil
	}

	msg := ErrIdempotencyConflict.Error()

	return false, ItemResult{EventID: eventID, Status: ItemRejected, Error: &msg}, nil
}

func (s *Service) release(ctx context.Context, eventIDs []string) {
	if len(eventIDs) == 0 {
		return
	}

	if err := s.claims.DeleteIdempotencyKeys(ctx, eventIDs); err != nil {
		s.log.Warn(
			"idempotency release failed",
			zap.Int(logger.FieldBatchSize, len(eventIDs)),
			zap.Error(err),
		)
	}
}

func (s *Service) publishEvents(ctx context.Context, pending []pendingEvent) error {
	if len(pending) == 0 {
		return nil
	}

	records := make([]Record, 0, len(pending))

	for _, p := range pending {
		records = append(records, Record{
			Topic: s.eventsTopic,
			Key:   EventPartitionKey(p.validated.DecisionID, p.validated.EventID),
			ID:    p.validated.EventID,
			Type:  EventEnvelopeType,
			Payload: KafkaEventPayload{
				EventID:       p.validated.EventID,
				EventType:     p.validated.EventType,
				SubjectID:     p.validated.Subject.Value,
				SaltVersion:   p.validated.Subject.SaltVersion,
				OccurredAt:    p.validated.OccurredAt,
				ReceivedAt:    p.validated.ReceivedAt,
				DecisionID:    p.validated.DecisionID,
				SchemaVersion: p.validated.SchemaVersion,
				Payload:       p.validated.Payload,
			},
		})
	}

	return s.producer.Publish(ctx, records)
}

func (s *Service) publishExposures(ctx context.Context, pending []pendingExposure) error {
	if len(pending) == 0 {
		return nil
	}

	records := make([]Record, 0, len(pending))

	for _, p := range pending {
		records = append(records, Record{
			Topic: s.exposuresTopic,
			Key:   p.validated.ExperimentID,
			ID:    p.validated.EventID,
			Type:  ExposureEnvelopeType,
			Payload: KafkaExposurePayload{
				EventID:             p.validated.EventID,
				DecisionID:          p.validated.DecisionID,
				SubjectID:           p.validated.Subject.Value,
				SaltVersion:         p.validated.Subject.SaltVersion,
				ExperimentID:        p.validated.ExperimentID,
				ExperimentVersionID: p.validated.ExperimentVersionID,
				VariantID:           p.validated.VariantID,
				FlagID:              p.validated.FlagID,
				OccurredAt:          p.validated.OccurredAt,
				ReceivedAt:          p.validated.ReceivedAt,
			},
		})
	}

	return s.producer.Publish(ctx, records)
}

func summarize(results []ItemResult) BatchData {
	out := BatchData{Results: results}

	for _, r := range results {
		switch r.Status {
		case ItemAccepted:
			out.Accepted++
		case ItemDuplicate:
			out.Duplicates++
		case ItemRejected:
			out.Rejected++
		}
	}

	return out
}
