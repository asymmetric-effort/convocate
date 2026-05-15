package protocol

// ClusterAuthMode represents the Claude authentication mode.
type ClusterAuthMode string

const (
	AuthModeAnthropicKey  ClusterAuthMode = "anthropic_api_key"
	AuthModeClaudeSession ClusterAuthMode = "claude_session"
)

// Valid reports whether the auth mode is one of the known values.
func (m ClusterAuthMode) Valid() bool {
	return m == AuthModeAnthropicKey || m == AuthModeClaudeSession
}

// SetClusterAuthRequest is the Web UI payload for setting or switching
// cluster-wide Claude authentication.
type SetClusterAuthRequest struct {
	Mode         ClusterAuthMode `json:"mode"`
	APIKey       string          `json:"api_key,omitempty"`
	SessionToken string          `json:"session_token,omitempty"`
}

// SetClusterAuthResponse is returned after updating cluster auth.
type SetClusterAuthResponse struct {
	Mode    ClusterAuthMode `json:"mode"`
	Updated bool            `json:"updated"`
}

// HostHealthInfo represents agent-fleet health data for the Web UI dashboard.
type HostHealthInfo struct {
	HostID         string  `json:"host_id"`
	ContainerCount int     `json:"container_count"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryPercent  float64 `json:"memory_percent"`
	LastHeartbeat  int64   `json:"last_heartbeat_unix"`
	Healthy        bool    `json:"healthy"`
}
