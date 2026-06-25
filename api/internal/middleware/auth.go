package middleware

import (
	"net/http"
	"strings"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

var mockPrincipal = &httputil.Principal{
	ID:       "usr-mock-admin",
	Username: "admin",
	Name:     "Mock Admin",
	Email:    "admin@convocate.local",
	Groups:   []string{"admins"},
	Roles:    []string{"admin"},
	IDP:      "local",
	AuthorizedApplets: []string{
		"nmgr", "amgr", "pb", "ide", "repo", "ac", "sup",
	},
}

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || len(auth) <= 7 {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
			return
		}
		ctx := httputil.ContextWithPrincipal(r.Context(), mockPrincipal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
