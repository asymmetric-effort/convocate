package types

// Page represents a paginated response.
type Page[T any] struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
	Items  []T `json:"items"`
}

// Error represents an API error response.
type Error struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []FieldDetail `json:"details,omitempty"`
}

// FieldDetail describes a validation error on a specific field.
type FieldDetail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// NodeStatus enumerates valid node states.
type NodeStatus string

const (
	NodeOnline   NodeStatus = "online"
	NodeDraining NodeStatus = "draining"
	NodeOffline  NodeStatus = "offline"
)

// AgentStatus enumerates valid agent-container states.
type AgentStatus string

const (
	AgentRunning    AgentStatus = "running"
	AgentConnected  AgentStatus = "connected"
	AgentStopped    AgentStatus = "stopped"
	AgentMigrating  AgentStatus = "migrating"
	AgentStopping   AgentStatus = "stopping"
)

// CardStatus enumerates valid card states.
type CardStatus string

const (
	CardTodo   CardStatus = "todo"
	CardActive CardStatus = "active"
	CardDone   CardStatus = "done"
	CardFail   CardStatus = "fail"
	CardNote   CardStatus = "note"
)

// EdgeType enumerates valid edge relationship types.
type EdgeType string

const (
	EdgeDependsOn EdgeType = "DependsOn"
	EdgeRelatesTo EdgeType = "RelatesTo"
)

// UserStatus enumerates valid user states.
type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// Visibility enumerates repository visibility levels.
type Visibility string

const (
	VisibilityPrivate  Visibility = "private"
	VisibilityInternal Visibility = "internal"
	VisibilityPublic   Visibility = "public"
)

// PrStatus enumerates pull request states.
type PrStatus string

const (
	PrOpen   PrStatus = "open"
	PrMerged PrStatus = "merged"
	PrClosed PrStatus = "closed"
)

// CheckStatus enumerates CI check states.
type CheckStatus string

const (
	CheckPassing CheckStatus = "passing"
	CheckRunning CheckStatus = "running"
	CheckFailed  CheckStatus = "failed"
)

// TicketStatus enumerates support ticket states.
type TicketStatus string

const (
	TicketOpen       TicketStatus = "open"
	TicketInProgress TicketStatus = "in-progress"
	TicketResolved   TicketStatus = "resolved"
	TicketClosed     TicketStatus = "closed"
)

// TicketPriority enumerates support ticket priorities.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "low"
	PriorityMedium TicketPriority = "medium"
	PriorityHigh   TicketPriority = "high"
)

// IDP enumerates identity providers.
type IDP string

const (
	IDPLocal  IDP = "local"
	IDPGitHub IDP = "github"
)
