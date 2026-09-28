package events

import (
	"time"

	"github.com/goccy/go-json"
)

type EventItem struct {
	EventID       string          `json:"event_id" binding:"required,uuid"`
	EventType     string          `json:"event_type" binding:"required,min=1,max=128"`
	SubjectID     string          `json:"subject_id" binding:"required,min=1,max=128"`
	OccurredAt    time.Time       `json:"occurred_at" binding:"required"`
	DecisionID    *string         `json:"decision_id" binding:"omitempty,uuid"`
	SchemaVersion *int            `json:"schema_version" binding:"omitempty,min=1"`
	Payload       json.RawMessage `json:"payload" binding:"required"`
}

func (i EventItem) DecisionIDValue() string {
	if i.DecisionID == nil {
		return ""
	}

	return *i.DecisionID
}

func (i EventItem) SchemaVersionValue() int {
	if i.SchemaVersion == nil {
		return 0
	}

	return *i.SchemaVersion
}

type EventBatchRequest struct {
	Events []EventItem `json:"events" binding:"required,min=1,max=500,dive"`
}

type ExposureItem struct {
	EventID             string    `json:"event_id" binding:"required,uuid"`
	DecisionID          string    `json:"decision_id" binding:"required,uuid"`
	SubjectID           string    `json:"subject_id" binding:"required,min=1,max=128"`
	ExperimentID        string    `json:"experiment_id" binding:"required,uuid"`
	ExperimentVersionID string    `json:"experiment_version_id" binding:"required,uuid"`
	VariantID           string    `json:"variant_id" binding:"required,uuid"`
	FlagID              string    `json:"flag_id" binding:"required,uuid"`
	OccurredAt          time.Time `json:"occurred_at" binding:"required"`
}

type ExposureBatchRequest struct {
	Exposures []ExposureItem `json:"exposures" binding:"required,min=1,max=500,dive"`
}

type ItemResult struct {
	EventID string  `json:"event_id"`
	Status  string  `json:"status"`
	Error   *string `json:"error,omitempty"`
}

type BatchData struct {
	Results    []ItemResult `json:"results"`
	Accepted   int          `json:"accepted"`
	Duplicates int          `json:"duplicates"`
	Rejected   int          `json:"rejected"`
}

type EventTypeResponse struct {
	Key             string  `json:"key"`
	Name            string  `json:"name"`
	Description     *string `json:"description,omitempty"`
	SchemaVersion   int     `json:"schema_version"`
	RequireExposure bool    `json:"require_exposure"`
	Status          string  `json:"status"`
}
