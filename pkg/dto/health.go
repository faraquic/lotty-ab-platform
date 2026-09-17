package dto

import "time"

const (
	StatusOK          = "ok"
	StatusUnavailable = "unavailable"
)

type Criticality string

const (
	CriticalityRequired Criticality = "required"
	CriticalityOptional Criticality = "optional"
)

func (c Criticality) Valid() bool {
	switch c {
	case CriticalityRequired, CriticalityOptional:
		return true
	default:
		return false
	}
}

type ComponentStatus struct {
	Status      string      `json:"status"`
	Criticality Criticality `json:"criticality"`
	Message     string      `json:"message,omitempty"`
}

type ReadyResponse struct {
	Service     string                     `json:"service"`
	Version     string                     `json:"version"`
	Environment string                     `json:"environment"`
	Timestamp   time.Time                  `json:"timestamp"`
	Components  map[string]ComponentStatus `json:"components"`
}
