package experiments

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	"github.com/goccy/go-json"
)

var (
	ErrNotFound           = errors.New("experiment not found")
	ErrVersionNotFound    = errors.New("experiment version not found")
	ErrConflictName       = errors.New("experiment with this name already exists")
	ErrFlagBusy           = errors.New("flag already has a running or paused experiment")
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrVersionConflict    = errors.New("experiment version conflict")
	ErrForbidden          = errors.New("not experiment owner")
	ErrInvalidVariants    = errors.New("invalid variants")
	ErrInvalidTargeting   = errors.New("invalid targeting")
	ErrInvalidWeights     = errors.New("invalid weights_total")
	ErrVersionNotReady    = errors.New("current version is not ready for review")
	ErrFlagNotFound       = errors.New("flag not found")
	ErrWinnerRequired     = errors.New("rollout winner variant is required")
	ErrReasonRequired     = errors.New("completion reason is required")
	ErrVersionImmutable   = errors.New("published version is immutable")
	ErrVersionNotApproved = errors.New("current version is not approved")
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusReview    Status = "review"
	StatusApproved  Status = "approved"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusArchived  Status = "archived"
	StatusRejected  Status = "rejected"
)

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusReview, StatusApproved, StatusRunning,
		StatusPaused, StatusCompleted, StatusArchived, StatusRejected:
		return true
	default:
		return false
	}
}

func ValidateTransition(from, to Status) error {
	switch from {
	case StatusDraft:
		if to == StatusReview {
			return nil
		}
	case StatusReview:
		if to == StatusApproved || to == StatusDraft || to == StatusRejected {
			return nil
		}
	case StatusApproved:
		if to == StatusRunning {
			return nil
		}
	case StatusRunning:
		if to == StatusPaused || to == StatusCompleted {
			return nil
		}
	case StatusPaused:
		if to == StatusRunning || to == StatusCompleted {
			return nil
		}
	case StatusCompleted:
		if to == StatusArchived {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

type CompletionDecision string

const (
	DecisionRolloutWinner CompletionDecision = "rollout_winner"
	DecisionRollback      CompletionDecision = "rollback"
	DecisionNoEffect      CompletionDecision = "no_effect"
)

func (d CompletionDecision) Valid() bool {
	switch d {
	case DecisionRolloutWinner, DecisionRollback, DecisionNoEffect:
		return true
	default:
		return false
	}
}

type VariantValue []byte

func (v VariantValue) MarshalJSON() ([]byte, error) {
	return v, nil
}

func (v *VariantValue) UnmarshalJSON(data []byte) error {
	out := make([]byte, len(data))
	copy(out, data)
	*v = out
	return nil
}

func (v VariantValue) Value() (driver.Value, error) {
	return string(v), nil
}

func (v *VariantValue) Scan(src any) error {
	switch s := src.(type) {
	case []byte:
		return v.UnmarshalJSON(s)
	case string:
		*v = []byte(s)
		return nil
	}
	return ErrInvalidVariants
}

type VariantInput struct {
	Name      string       `json:"name"`
	Value     VariantValue `json:"value"`
	WeightBP  int          `json:"weight_bp"`
	IsControl bool         `json:"is_control"`
}

func ValidateVariants(inputs []VariantInput, weightsTotal int) error {
	if len(inputs) < 2 {
		return fmt.Errorf("%w: at least 2 variants required", ErrInvalidVariants)
	}
	seen := make(map[string]struct{}, len(inputs))
	controls := 0
	sum := 0
	for _, in := range inputs {
		if strings.TrimSpace(in.Name) == "" || len(in.Name) > 128 {
			return fmt.Errorf("%w: variant name must be 1..128 chars", ErrInvalidVariants)
		}
		if _, dup := seen[in.Name]; dup {
			return fmt.Errorf("%w: duplicate variant name %q", ErrInvalidVariants, in.Name)
		}
		seen[in.Name] = struct{}{}
		if len(in.Value) == 0 || !json.Valid(in.Value) {
			return fmt.Errorf("%w: variant %q value must be valid JSON", ErrInvalidVariants, in.Name)
		}
		if in.WeightBP <= 0 {
			return fmt.Errorf("%w: variant %q weight must be positive", ErrInvalidVariants, in.Name)
		}
		if in.IsControl {
			controls++
		}
		sum += in.WeightBP
	}
	if controls != 1 {
		return fmt.Errorf("%w: exactly one control variant required", ErrInvalidVariants)
	}
	if sum != weightsTotal {
		return fmt.Errorf("%w: weights sum %d does not equal %d", ErrInvalidVariants, sum, weightsTotal)
	}
	return nil
}

type Targeting []byte

func (t Targeting) Valid() bool {
	if len(t) == 0 {
		return false
	}
	if !json.Valid(t) {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(t, &m); err != nil {
		return false
	}
	return true
}

func (t Targeting) MarshalJSON() ([]byte, error) {
	return t, nil
}

func (t *Targeting) UnmarshalJSON(data []byte) error {
	out := make([]byte, len(data))
	copy(out, data)
	*t = out
	return nil
}

func (t Targeting) Value() (driver.Value, error) {
	return string(t), nil
}

func (t *Targeting) Scan(src any) error {
	switch s := src.(type) {
	case []byte:
		return t.UnmarshalJSON(s)
	case string:
		*t = []byte(s)
		return nil
	}
	return ErrInvalidTargeting
}

func NewSalt() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type Experiment struct {
	ID                 string
	FlagID             string
	Name               string
	Description        *string
	Status             Status
	CurrentVersionID   *string
	OwnerID            string
	Version            int
	GuardrailPaused    bool
	CompletionDecision *CompletionDecision
	CompletionReason   *string
	CreatedBy          string
	UpdatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ExperimentVersion struct {
	ID               string
	ExperimentID     string
	VersionNum       int
	ReviewID         *string
	WeightsTotal     int
	Targeting        *Targeting
	DistributionSalt string
	CreatedAt        time.Time
	CreatedBy        string
}

type Variant struct {
	ID        string
	VersionID string
	Name      string
	Value     VariantValue
	WeightBP  int
	IsControl bool
}

type ExperimentDetail struct {
	Experiment     Experiment
	CurrentVersion *ExperimentVersion
	Variants       []Variant
	CreatedBy      *users.User
	UpdatedBy      *users.User
}
