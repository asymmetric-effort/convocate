package status

import (
	"context"
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/db"
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

func checkPostgres() serviceStatus {
	if db.Pool == nil {
		return serviceStatus{Name: "postgresql", Status: "unavailable", LatencyMs: 0}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := db.Pool.Ping(ctx)
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return serviceStatus{Name: "postgresql", Status: "unhealthy", LatencyMs: latency}
	}
	return serviceStatus{Name: "postgresql", Status: "healthy", LatencyMs: latency}
}

func checkRedis() serviceStatus {
	if db.Redis == nil {
		return serviceStatus{Name: "redis", Status: "unavailable", LatencyMs: 0}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := db.Redis.Ping(ctx).Err()
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return serviceStatus{Name: "redis", Status: "unhealthy", LatencyMs: latency}
	}
	return serviceStatus{Name: "redis", Status: "healthy", LatencyMs: latency}
}

func handleStatus(w http.ResponseWriter, _ *http.Request) {
	uptime := time.Since(startTime).Round(time.Second).String()

	services := []serviceStatus{
		{Name: "api", Status: "healthy", LatencyMs: 0.1},
		checkRedis(),
		checkPostgres(),
	}

	overall := "healthy"
	for _, s := range services {
		if s.Status != "healthy" {
			overall = "degraded"
			break
		}
	}

	httputil.WriteJSON(w, http.StatusOK, platformStatus{
		Status:    overall,
		Version:   "2.0.0-dev",
		Uptime:    uptime,
		Services:  services,
		Nodes:     []any{},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
