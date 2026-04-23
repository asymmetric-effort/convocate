package agentserver

import (
	"encoding/json"
	"time"
)

// PingResult is what callers get back from the "ping" op — a lightweight
// proof that the RPC plumbing is working end-to-end. The agent ID and
// version are stable; the server time is a liveness signal.
type PingResult struct {
	AgentID    string `json:"agent_id"`
	Version    string `json:"version"`
	ServerTime string `json:"server_time"`
}

// RegisterCoreOps wires up the ops that are always available regardless of
// what else the agent runs. For 2a this is just "ping"; later commits
// register the CRUD op set against the same dispatcher.
func RegisterCoreOps(d *Dispatcher, agentID, version string) {
	d.Register("ping", func(_ json.RawMessage) (any, error) {
		return PingResult{
			AgentID:    agentID,
			Version:    version,
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		}, nil
	})
}
