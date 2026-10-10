package registry

import (
	"encoding/json"
	"strings"
)

// Event is a source lifecycle notification (JSON or mapped from proto).
type Event struct {
	Event    string            `json:"event"`     // up | down | heartbeat
	SourceID string            `json:"source_id"` // preferred
	Source   string            `json:"source"`    // alias accepted from edge
	RoleHint string            `json:"role_hint,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// ParseJSON unmarshals a SourceEvent-shaped JSON payload.
func ParseJSON(data []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(data, &e); err != nil {
		return Event{}, err
	}
	e.Normalize()
	return e, nil
}

// Normalize lowercases the event type and fills SourceID from Source when needed.
func (e *Event) Normalize() {
	e.Event = strings.ToLower(strings.TrimSpace(e.Event))
	if e.SourceID == "" {
		e.SourceID = strings.TrimSpace(e.Source)
	}
	e.SourceID = strings.TrimSpace(e.SourceID)
}

// ID returns the logical source id.
func (e Event) ID() string { return e.SourceID }

// IsUp reports whether the source should be treated as live.
func (e Event) IsUp() bool {
	switch e.Event {
	case "up", "heartbeat", "online", "register":
		return true
	default:
		return false
	}
}

// IsDown reports offline / deregister.
func (e Event) IsDown() bool {
	switch e.Event {
	case "down", "offline", "deregister":
		return true
	default:
		return false
	}
}
