package middleware

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// RBAC returns a middleware that enforces role-based access control.
// The principal must have the required role or the "admin" role.
func RBAC(requiredRole string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := httputil.PrincipalFromContext(r.Context())
			if !ok || p == nil {
				httputil.WriteError(w, http.StatusForbidden, "forbidden", "no principal in context")
				return
			}

			// Admin role implies all permissions
			for _, role := range p.Roles {
				if role == "admin" || role == "admin-policy" || role == requiredRole {
					next.ServeHTTP(w, r)
					return
				}
			}

			httputil.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions: requires "+requiredRole)
		})
	}
}
