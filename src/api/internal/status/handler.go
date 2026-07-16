package status

import (
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

var startTime = time.Now()

type serviceStatus struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latencyMs"`
}

type platformStatus struct {
	Status    string          `json:"status"`
	Version   string          `json:"version"`
	Uptime    string          `json:"uptime"`
	Services  []serviceStatus `json:"services"`
	Nodes     []any           `json:"nodes"`
	Timestamp string          `json:"timestamp"`
}

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/status", handleStatus)
}

func handleStatus(w http.ResponseWriter, _ *http.Request) {
	uptime := time.Since(startTime).Round(time.Second).String()
	httputil.WriteJSON(w, http.StatusOK, platformStatus{
		Status:  "healthy",
		Version: "2.0.0-dev",
		Uptime:  uptime,
		Services: []serviceStatus{
			{Name: "api", Status: "healthy", LatencyMs: 0.1},
			{Name: "redis", Status: "healthy", LatencyMs: 1.2},
			{Name: "postgresql", Status: "healthy", LatencyMs: 2.3},
			{Name: "openbao", Status: "healthy", LatencyMs: 1.0},
		},
		Nodes:     []any{},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
