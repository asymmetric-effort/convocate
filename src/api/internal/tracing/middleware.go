package tracing

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// Middleware returns an HTTP middleware that wraps handlers with
// OpenTelemetry distributed tracing via otelhttp. Each span includes
// the HTTP method, path, status code, and the authenticated principal
// username when available.
func Middleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if p, ok := httputil.PrincipalFromContext(r.Context()); ok && p != nil {
			span.SetAttributes(attribute.String("enduser.id", p.Username))
		}
		next.ServeHTTP(w, r)
	}), "convocate-api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}
