// node-metrics — lightweight DaemonSet agent that collects system
// metrics from /proc and filesystem and pushes them to the Convocate
// API every 3 seconds.
//
// Environment variables:
//   NODE_NAME        — K8s node name (from downward API)
//   KUBELET_VERSION  — kubelet version (from downward API)
//   API_URL          — Convocate API base URL (e.g. http://convocate-api.convocate.svc:8443)
//   METRICS_API_KEY  — shared secret for internal auth
//   PROC_PREFIX      — host /proc mount (default: /host/proc)
//   ROOT_PREFIX      — host root mount (default: /host/root)

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	runFromEnv(3*time.Second, nil)
}

// runFromEnv parses config from environment and runs the metrics loop.
func runFromEnv(interval time.Duration, stopCh <-chan struct{}) {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatal(err)
	}
	run(cfg, interval, stopCh)
}

// run executes the metrics collection loop. The loop exits when stopCh is
// closed (or receives). Pass a nil channel for the production infinite loop.
func run(cfg metricsConfig, interval time.Duration, stopCh <-chan struct{}) {
	log.Printf("[node-metrics] starting on node=%s api=%s", cfg.nodeName, cfg.apiURL)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		postMetrics(cfg)
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}
	}
}

// metricsConfig holds parsed configuration for the metrics agent.
type metricsConfig struct {
	nodeName   string
	apiURL     string
	metricsKey string
	endpoint   string
}

// parseConfig reads environment variables and returns a validated config.
// Returns an error if required variables are missing.
func parseConfig() (metricsConfig, error) {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return metricsConfig{}, fmt.Errorf("NODE_NAME environment variable is required")
	}

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://convocate-api.convocate.svc:8443"
	}
	metricsKey := os.Getenv("METRICS_API_KEY")

	return metricsConfig{
		nodeName:   nodeName,
		apiURL:     apiURL,
		metricsKey: metricsKey,
		endpoint:   apiURL + "/api/v1/nmgr/metrics",
	}, nil
}

// postMetrics collects system metrics and posts them to the API.
func postMetrics(cfg metricsConfig) {
	report := collectAll(cfg.nodeName)
	report.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// json.Marshal cannot fail on MetricsReport (all basic types).
	body, _ := json.Marshal(report)

	req, err := http.NewRequest("POST", cfg.endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("[node-metrics] request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.metricsKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.metricsKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[node-metrics] post error: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		log.Printf("[node-metrics] unexpected status: %d", resp.StatusCode)
	}
}
