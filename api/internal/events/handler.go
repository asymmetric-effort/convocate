package events

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

func Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/events/{applet}/{channel...}", middleware.Chain(
		http.HandlerFunc(handleEvents),
		middleware.Auth,
	))
}

func handleEvents(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteError(w, http.StatusNotImplemented, "not_implemented", "websocket event channel stub")
}
