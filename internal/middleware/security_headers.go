// Package middleware provides reusable HTTP middleware for the convocate services.
package middleware

import "net/http"

// SecurityHeaders returns a middleware that sets security-related HTTP response
// headers on every response. It should wrap the outermost handler so that all
// responses—including error pages—carry the headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		h.Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}
