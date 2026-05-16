package auth

import (
	"net/http"
	"strings"
)

// skipPaths are path prefixes that do not require authentication.
var skipPaths = []string{
	"/auth/",
	"/v1/",
	"/health",
	"/favicon.svg",
	"/assets/",
}

// Middleware returns an HTTP middleware that enforces session authentication.
// If no valid session cookie is found, the request is redirected to /auth/login.
// Paths in skipPaths are exempt from authentication.
func Middleware(sessions *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipAuth(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				redirectToLogin(w, r)
				return
			}

			session, err := sessions.Get(cookie.Value)
			if err != nil || session == nil {
				redirectToLogin(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func shouldSkipAuth(path string) bool {
	for _, prefix := range skipPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	// For XHR/API requests, return 401 instead of redirect.
	if r.Header.Get("Accept") == "application/json" ||
		strings.HasPrefix(r.URL.Path, "/ui/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		return
	}
	http.Redirect(w, r, "/auth/login", http.StatusFound)
}
