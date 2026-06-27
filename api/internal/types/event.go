package types

// Event represents a real-time event pushed over WebSocket.
type Event struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Payload   any    `json:"payload"`
}

// ServiceStatus represents the health of a backing service.
type ServiceStatus struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latencyMs"`
	Message   string  `json:"message,omitempty"`
}

// NodeHealthSummary represents a node's health in the status endpoint.
type NodeHealthSummary struct {
	NodeID    string     `json:"nodeId"`
	Status    NodeStatus `json:"status"`
	Reachable bool       `json:"reachable"`
	Agents    int        `json:"agents"`
}

// PlatformStatus represents the full platform health check response.
type PlatformStatus struct {
	Status    string              `json:"status"`
	Version   string              `json:"version"`
	Uptime    string              `json:"uptime"`
	Services  []ServiceStatus     `json:"services"`
	Nodes     []NodeHealthSummary `json:"nodes"`
	Timestamp string              `json:"timestamp"`
}
