package types

// User represents a Convocate user.
type User struct {
	ID     string     `json:"id"`
	Email  string     `json:"email"`
	Name   string     `json:"name"`
	Status UserStatus `json:"status"`
	Groups []string   `json:"groups"`
}

// UserInput is the write model for user create/update.
type UserInput struct {
	Email    string     `json:"email,omitempty"`
	Name     string     `json:"name,omitempty"`
	Password string     `json:"password,omitempty"`
	Status   UserStatus `json:"status,omitempty"`
	Groups   []string   `json:"groups,omitempty"`
}

// Group represents a user group with role mappings.
type Group struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Builtin   bool     `json:"builtin"`
	UserCount int      `json:"userCount"`
	Roles     []string `json:"roles"`
}

// Role represents an assignable RBAC role.
type Role struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Applet      string `json:"applet"`
}

// GlobalSettings represents platform-wide security settings.
type GlobalSettings struct {
	RequireMFA           bool `json:"requireMfa"`
	SessionTimeoutMin    int  `json:"sessionTimeoutMinutes"`
	PasswordMinLength    int  `json:"passwordMinLength"`
	PasswordRotationDays int  `json:"passwordRotationDays"`
}
