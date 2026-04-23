package statusproto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent_StampsTimestamp(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	ev := NewEvent(TypeAgentStarted, "agent-id", "sess")
	after := time.Now().UTC().Add(time.Second)
	if ev.Timestamp.Before(before) || ev.Timestamp.After(after) {
		t.Errorf("timestamp %v outside [%v, %v]", ev.Timestamp, before, after)
	}
	if ev.Type != TypeAgentStarted {
		t.Errorf("Type = %q", ev.Type)
	}
	if ev.AgentID != "agent-id" {
		t.Errorf("AgentID = %q", ev.AgentID)
	}
	if ev.SessionID != "sess" {
		t.Errorf("SessionID = %q", ev.SessionID)
	}
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	in := NewEvent(TypeContainerStarted, "a", "s")
	in.Data = json.RawMessage(`{"port":8080}`)
	enc, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Event
	if err := json.Unmarshal(enc, &out); err != nil {
		t.Fatal(err)
	}
	if out.Type != in.Type || out.AgentID != in.AgentID || out.SessionID != in.SessionID {
		t.Errorf("mismatch: in=%+v out=%+v", in, out)
	}
	if string(out.Data) != `{"port":8080}` {
		t.Errorf("data = %s", out.Data)
	}
}

func TestEventConstants_AreStable(t *testing.T) {
	// If these names ever change they're a wire break — callers write the
	// literal strings to logs and dashboards. Catch any rename here.
	cases := map[string]string{
		TypeAgentStarted:     "agent.started",
		TypeAgentShutdown:    "agent.shutdown",
		TypeAgentHeartbeat:   "agent.heartbeat",
		TypeContainerCreated: "container.created",
		TypeContainerEdited:  "container.edited",
		TypeContainerStarted: "container.started",
		TypeContainerStopped: "container.stopped",
		TypeContainerDeleted: "container.deleted",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant mismatch: %q != %q", got, want)
		}
	}
	if Subsystem != "claude-shell-status" {
		t.Errorf("Subsystem = %q", Subsystem)
	}
}
