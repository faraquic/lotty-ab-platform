package health

import "time"

const (
	StatusOK          = "ok"
	StatusUnavailable = "unavailable"
)

type ComponentStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ReadyResponse struct {
	Service     string                     `json:"service"`
	Version     string                     `json:"version"`
	Environment string                     `json:"environment"`
	Timestamp   time.Time                  `json:"timestamp"`
	Components  map[string]ComponentStatus `json:"components"`
}
