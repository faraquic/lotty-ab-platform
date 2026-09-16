package decide

import (
	"encoding/hex"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	repo     *Repository
	maxStale time.Duration
	log      *zap.Logger
}

func NewService(repo *Repository, maxStale time.Duration, log *zap.Logger) *Service {
	if maxStale <= 0 {
		maxStale = 5 * time.Minute
	}
	return &Service{repo: repo, maxStale: maxStale, log: log.Named("decide")}
}

func (s *Service) Decide(req CreateDecisionRequest, requestID string) (*DecisionResponse, error) {
	snap, err := s.repo.GetSnapshot()
	if err != nil {
		return nil, err
	}

	for _, key := range req.Flags {
		if _, ok := s.repo.FindFlag(key); !ok {
			return nil, ErrUnknownFlag
		}
	}

	rev := ""
	if snap.Revision != nil {
		rev = hex.EncodeToString(snap.Revision[:])
	}
	degraded := s.repo.Stale(s.maxStale)

	result := make(map[string]FlagDecision, len(req.Flags))
	for _, key := range req.Flags {
		result[key] = s.decideFlag(key, req.SubjectID)
	}

	return &DecisionResponse{
		RequestID:      requestID,
		ConfigRevision: rev,
		Degraded:       degraded,
		Flags:          result,
	}, nil
}

func (s *Service) decideFlag(key, subjectID string) FlagDecision {
	flag, ok := s.repo.FindFlag(key)
	if !ok {
		return FlagDecision{
			Value:  nil,
			Source: SourceDefault,
			Reason: "flag not found",
		}
	}

	decisionID, err := uuid.NewV7()
	if err != nil {
		return FlagDecision{
			Value:  snapshotRawToValue(flag.Value),
			Source: SourceDefault,
			Reason: "decision id failed",
		}
	}
	id := decisionID.String()

	exp, ok := s.repo.FindExperiment(key)
	if !ok {
		return FlagDecision{
			Value:      snapshotRawToValue(flag.Value),
			Source:     SourceDefault,
			Reason:     "no experiment",
			DecisionID: id,
		}
	}

	if !targetingMatch(exp.Targeting) {
		return FlagDecision{
			Value:      snapshotRawToValue(flag.Value),
			Source:     SourceDefault,
			Reason:     "targeting mismatch",
			DecisionID: id,
		}
	}

	variant, ok := selectVariant(*exp, bucketPos(exp.ID, exp.Salt, subjectID))
	if !ok {
		return FlagDecision{
			Value:      snapshotRawToValue(flag.Value),
			Source:     SourceDefault,
			Reason:     "outside allocation",
			DecisionID: id,
		}
	}

	return FlagDecision{
		Value:             snapshotRawToValue(variant.Value),
		Source:            SourceExperiment,
		ExperimentID:      exp.ID,
		ExperimentVersion: exp.VersionNum,
		VariantID:         variant.ID,
		DecisionID:        id,
	}
}

func snapshotRawToValue(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		return v
	}
	return string(raw)
}
