package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// metricsAPIKey is loaded once at startup from the file pointed to by METRICS_API_KEY_FILE.
var metricsAPIKey = func() string {
	if f := os.Getenv("METRICS_API_KEY_FILE"); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}()

// InternalAuth validates requests from internal services (e.g. the
// node-metrics DaemonSet) using a shared API key.  If METRICS_API_KEY_FILE
// is empty or unreadable, all requests are denied (fail closed).
func InternalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if metricsAPIKey == "" {
			// No key configured — deny all (fail closed)
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "METRICS_API_KEY_FILE not configured")
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != metricsAPIKey {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid metrics API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}
