package heartbeat

import (
	"time"
)

// HeartbeatEvent represents a heartbeat event
type HeartbeatEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}
