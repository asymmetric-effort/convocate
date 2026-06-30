package types

// Agent represents an agent-container on a node.
type Agent struct {
	ID      string      `json:"id"`
	Project string      `json:"project"`
	NodeID  string      `json:"nodeId"`
	Status  AgentStatus `json:"status"`
	Expose  string      `json:"expose,omitempty"`
	Owner   string      `json:"owner"`
}

// AgentResources defines CPU, memory, and storage limits for an agent pod.
type AgentResources struct {
	CPURequest    string `json:"cpuRequest,omitempty"`    // e.g. "500m"
	CPULimit      string `json:"cpuLimit,omitempty"`      // e.g. "2"
	MemoryRequest string `json:"memoryRequest,omitempty"` // e.g. "512Mi"
	MemoryLimit   string `json:"memoryLimit,omitempty"`   // e.g. "2Gi"
	StorageSize   string `json:"storageSize,omitempty"`   // e.g. "2Gi"
}

// AgentMount defines a host path mount for an agent pod (admin-only).
type AgentMount struct {
	HostPath  string `json:"hostPath"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly"`
}

// AgentSecurity defines security overrides for an agent pod (admin-only).
type AgentSecurity struct {
	Capabilities     []string     `json:"capabilities,omitempty"`
	DockerAccess     bool         `json:"dockerAccess,omitempty"`
	AdditionalMounts []AgentMount `json:"additionalMounts,omitempty"`
}

// AgentNetwork defines network policy overrides for an agent pod.
type AgentNetwork struct {
	AdditionalEgress []string `json:"additionalEgress,omitempty"` // CIDR or hostname list
}

// CreateAgentRequest represents a request to create an agent-container.
type CreateAgentRequest struct {
	Project         string          `json:"project"`
	NodeID          string          `json:"nodeId"`
	Image           string          `json:"image,omitempty"`
	Command         string          `json:"command,omitempty"`
	ClaudeFlags     []string        `json:"claudeFlags,omitempty"`
	Resources       *AgentResources `json:"resources,omitempty"`
	Security        *AgentSecurity  `json:"security,omitempty"`  // admin-only
	Network         *AgentNetwork   `json:"network,omitempty"`
	Logging         bool            `json:"logging,omitempty"`
	AnthropicApiKey string          `json:"anthropicApiKey,omitempty"`
	ClaudeMd        string          `json:"claudeMd,omitempty"`
}

// ConfigureAgentRequest represents a request to update an agent-container.
type ConfigureAgentRequest struct {
	Project         *string         `json:"project,omitempty"`
	NodeID          *string         `json:"nodeId,omitempty"`
	Expose          *string         `json:"expose,omitempty"`
	ClaudeFlags     []string        `json:"claudeFlags,omitempty"`
	Resources       *AgentResources `json:"resources,omitempty"`
	Security        *AgentSecurity  `json:"security,omitempty"`  // admin-only
	Network         *AgentNetwork   `json:"network,omitempty"`
	Logging         *bool           `json:"logging,omitempty"`
	ClaudeMd        *string         `json:"claudeMd,omitempty"`
}
