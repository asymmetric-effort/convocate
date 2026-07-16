package httputil

import "context"

type contextKey int

const principalKey contextKey = iota

type Principal struct {
	ID                string   `json:"id"`
	Username          string   `json:"username"`
	Name              string   `json:"name"`
	Email             string   `json:"email,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	Roles             []string `json:"roles"`
	IDP               string   `json:"idp"`
	AuthorizedApplets []string `json:"authorizedApplets"`
}

func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}
