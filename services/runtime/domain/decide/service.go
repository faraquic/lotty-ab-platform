package decide

import (
	"encoding/hex"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type Service struct {
	repo *Repository
	log  *zap.Logger
}

func NewService(repo *Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log.Named("decide")}
}

func (s *Service) Decide(req CreateDecisionRequest, requestID string) (*DecisionResponse, error) {
	snap, err := s.repo.GetSnapshot()
	if err != nil {
		return nil, err
	}

	rev := ""
	if snap.Revision != nil {
		rev = hex.EncodeToString(snap.Revision[:])
	}

	result := make(map[string]FlagDecision, len(req.Flags))
	for _, key := range req.Flags {
		flag, ok := s.repo.FindFlag(key)
		if !ok {
			result[key] = FlagDecision{
				Value:  nil,
				Source: SourceMissing,
				Reason: "flag not found",
			}
			continue
		}

		result[key] = FlagDecision{
			Value:  snapshotRawToValue(flag.Value),
			Source: SourceDefault,
		}
	}

	return &DecisionResponse{
		RequestID:      requestID,
		ConfigRevision: rev,
		Flags:          result,
	}, nil
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
