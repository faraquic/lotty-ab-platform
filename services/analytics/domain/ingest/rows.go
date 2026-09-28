package ingest

import (
	"errors"

	pkgclickhouse "github.com/faraquic/lotty-ab-platform/pkg/clickhouse"
	"github.com/faraquic/lotty-ab-platform/pkg/outbox"
	events "github.com/faraquic/lotty-ab-platform/services/analytics/domain/events"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

var ErrUnknownRecordType = errors.New("unknown record type")

func EventRowFromEnvelope(env outbox.Envelope) (pkgclickhouse.EventRow, error) {
	var p events.KafkaEventPayload

	if err := json.Unmarshal(env.Data, &p); err != nil {
		return pkgclickhouse.EventRow{}, err
	}

	eventID, err := uuid.Parse(p.EventID)
	if err != nil {
		return pkgclickhouse.EventRow{}, err
	}

	decisionID, err := parseOptionalUUID(p.DecisionID)
	if err != nil {
		return pkgclickhouse.EventRow{}, err
	}

	return pkgclickhouse.EventRow{
		EventID:       eventID,
		EventType:     p.EventType,
		SubjectID:     p.SubjectID,
		SaltVersion:   p.SaltVersion,
		OccurredAt:    p.OccurredAt.UTC(),
		ReceivedAt:    p.ReceivedAt.UTC(),
		DecisionID:    decisionID,
		Payload:       string(p.Payload),
		SchemaVersion: uint16(p.SchemaVersion),
	}, nil
}

func ExposureRowFromEnvelope(env outbox.Envelope) (pkgclickhouse.ExposureRow, error) {
	var p events.KafkaExposurePayload

	if err := json.Unmarshal(env.Data, &p); err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	eventID, err := uuid.Parse(p.EventID)
	if err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	decisionID, err := uuid.Parse(p.DecisionID)
	if err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	experimentID, err := uuid.Parse(p.ExperimentID)
	if err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	versionID, err := uuid.Parse(p.ExperimentVersionID)
	if err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	variantID, err := uuid.Parse(p.VariantID)
	if err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	flagID, err := uuid.Parse(p.FlagID)
	if err != nil {
		return pkgclickhouse.ExposureRow{}, err
	}

	return pkgclickhouse.ExposureRow{
		EventID:             eventID,
		DecisionID:          decisionID,
		SubjectID:           p.SubjectID,
		SaltVersion:         p.SaltVersion,
		ExperimentID:        experimentID,
		ExperimentVersionID: versionID,
		VariantID:           variantID,
		FlagID:              flagID,
		OccurredAt:          p.OccurredAt.UTC(),
		ReceivedAt:          p.ReceivedAt.UTC(),
	}, nil
}

func parseOptionalUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(s)
}
