package decide

type CreateDecisionRequest struct {
	SubjectID  string         `json:"subject_id" binding:"required,min=1,max=128"`
	Attributes map[string]any `json:"attributes" binding:"omitempty"`
	Flags      []string       `json:"flags" binding:"omitempty,min=1,max=128,dive,min=3,max=128"`
}

const (
	SourceDefault    = "default"
	SourceExperiment = "experiment"
)

type FlagDecision struct {
	Value             any    `json:"value"`
	Source            string `json:"source"`
	Reason            string `json:"reason,omitempty"`
	ExperimentID      string `json:"experiment_id,omitempty"`
	ExperimentVersion int    `json:"experiment_version,omitempty"`
	VariantID         string `json:"variant_id,omitempty"`
	DecisionID        string `json:"decision_id,omitempty"`
}

type DecisionResponse struct {
	RequestID      string                  `json:"request_id"`
	ConfigRevision string                  `json:"config_revision"`
	Degraded       bool                    `json:"degraded"`
	Flags          map[string]FlagDecision `json:"flags"`
}
