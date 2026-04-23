// Package statusproto defines the wire format and event types agents use
// to push status to claude-shell over the claude-shell-status SSH subsystem.
//
// Events travel as single-line JSON objects (one per line); the shell side
// reads them sequentially without any additional framing. Any unknown event
// type is preserved on the receiver so older shells can forward/log without
// understanding newer events — forward-compatibility is important because
// agents and shells can run different versions during a rolling upgrade.
package statusproto

import (
	"encoding/json"
	"time"
)

// Subsystem is the SSH subsystem name the shell-side server accepts for
// status push. Any other subsystem on the shell's listener is refused.
const Subsystem = "claude-shell-status"

// Event types — agents MUST use these exact strings for the Type field so
// receivers can route without parsing Data blindly.
const (
	// AgentStarted is emitted once the agent has bound its listener and is
	// ready to accept CRUD/attach traffic. Data is empty.
	TypeAgentStarted = "agent.started"

	// AgentShutdown is emitted during graceful shutdown just before the
	// listener closes. Data is empty.
	TypeAgentShutdown = "agent.shutdown"

	// AgentHeartbeat is emitted on a periodic timer (cadence chosen by the
	// agent, typically 30s) so the shell can detect a dead agent even when
	// nothing else is happening.
	TypeAgentHeartbeat = "agent.heartbeat"

	// ContainerCreated is emitted when create op succeeds.
	TypeContainerCreated = "container.created"

	// ContainerEdited is emitted when edit op successfully updates the
	// session's metadata.
	TypeContainerEdited = "container.edited"

	// ContainerStarted is emitted when restart op brings a container up.
	TypeContainerStarted = "container.started"

	// ContainerStopped is emitted when kill or stop causes the container to
	// exit, or when the agent observes it going from running -> not running.
	TypeContainerStopped = "container.stopped"

	// ContainerDeleted is emitted when delete op removes the session.
	TypeContainerDeleted = "container.deleted"
)

// Event is the envelope every status message travels in. Data is left as
// json.RawMessage so handlers can decode to whatever concrete shape the
// event type implies without forcing a single struct tree.
type Event struct {
	Type      string          `json:"type"`
	AgentID   string          `json:"agent_id"`
	SessionID string          `json:"session_id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewEvent is a small helper to stamp an Event with "now" if the caller
// doesn't already have a time. Returns the event by value so callers can
// easily set Data before encoding.
func NewEvent(typ, agentID, sessionID string) Event {
	return Event{
		Type:      typ,
		AgentID:   agentID,
		SessionID: sessionID,
		Timestamp: time.Now().UTC(),
	}
}
