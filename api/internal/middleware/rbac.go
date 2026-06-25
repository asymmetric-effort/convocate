package middleware

import "net/http"

func RBAC(_ string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock: always permits. The requiredRole parameter is captured
			// so the real implementation can be dropped in later.
			next.ServeHTTP(w, r)
		})
	}
}
