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

// CreateAgentRequest represents a request to create an agent-container.
type CreateAgentRequest struct {
	Project string `json:"project"`
	NodeID  string `json:"nodeId"`
	Image   string `json:"image,omitempty"`
	Command string `json:"command,omitempty"`
}

// ConfigureAgentRequest represents a request to configure an agent-container.
type ConfigureAgentRequest struct {
	Project *string `json:"project,omitempty"`
	NodeID  *string `json:"nodeId,omitempty"`
	Expose  *string `json:"expose,omitempty"`
}
