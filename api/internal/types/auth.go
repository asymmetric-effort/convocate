package types

// LoginRequest represents a login attempt.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	MFAToken string `json:"mfaToken"`
}

// Session represents an authenticated session.
type Session struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    string    `json:"expiresAt"`
	Principal    Principal `json:"principal"`
}

// Principal represents the authenticated user's identity and permissions.
type Principal struct {
	ID                string   `json:"id"`
	Username          string   `json:"username"`
	Name              string   `json:"name"`
	Email             string   `json:"email,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	Roles             []string `json:"roles"`
	IDP               IDP      `json:"idp"`
	AuthorizedApplets []string `json:"authorizedApplets"`
}
