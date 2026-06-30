package types

// Node represents a compute host running Convocate.
type Node struct {
	ID              string     `json:"id"`
	Location        string     `json:"location"`
	IP              string     `json:"ip"`
	Status          NodeStatus `json:"status"`
	Agents          int        `json:"agents"`
	LoadAvg         LoadAvg    `json:"loadAvg"`
	MemUsedGB       float64    `json:"memUsedGB"`
	MemTotalGB      float64    `json:"memTotalGB"`
	SwapUsedGB      float64    `json:"swapUsedGB"`
	SwapTotalGB     float64    `json:"swapTotalGB"`
	DiskUsedGB      float64    `json:"diskUsedGB"`
	DiskTotalGB     float64    `json:"diskTotalGB"`
	UptimeSeconds   int64      `json:"uptimeSeconds"`
	KubeletVersion  string     `json:"kubeletVersion"`
	CPUCount        int        `json:"cpuCount"`
	Tags            []string   `json:"tags"`
}

// LoadAvg represents CPU load averages.
type LoadAvg struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

// NodeDetail extends Node with agent list and notes.
type NodeDetail struct {
	Node
	AgentList []Agent `json:"agentList"`
	Notes     []Note  `json:"notes"`
}

// Note represents a write-once note on a node.
type Note struct {
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
	Text      string `json:"text"`
}

// NodeMetricsReport is the payload sent by the node-metrics DaemonSet
// every 3 seconds.  Values are in raw units (bytes, seconds) and the
// API converts them to GB before publishing to subscribers.
type NodeMetricsReport struct {
	NodeName       string  `json:"nodeName"`
	LoadAvg        LoadAvg `json:"loadAvg"`
	MemUsedBytes   int64   `json:"memUsedBytes"`
	MemTotalBytes  int64   `json:"memTotalBytes"`
	SwapUsedBytes  int64   `json:"swapUsedBytes"`
	SwapTotalBytes int64   `json:"swapTotalBytes"`
	DiskUsedBytes  int64   `json:"diskUsedBytes"`
	DiskTotalBytes int64   `json:"diskTotalBytes"`
	UptimeSeconds  int64   `json:"uptimeSeconds"`
	KubeletVersion string  `json:"kubeletVersion"`
	CPUCount       int     `json:"cpuCount"`
	Timestamp      string  `json:"timestamp"`
}

// ProvisionNodeRequest represents a request to provision a new node.
type ProvisionNodeRequest struct {
	Host     string   `json:"host"`
	User     string   `json:"user"`
	Password string   `json:"password,omitempty"`
	Location string   `json:"location"`
	Tags     []string `json:"tags"`
}
