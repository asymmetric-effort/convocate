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
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatal("NODE_NAME environment variable is required")
	}

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://convocate-api.convocate.svc:8443"
	}
	metricsKey := os.Getenv("METRICS_API_KEY")

	endpoint := apiURL + "/api/v1/nmgr/metrics"

	log.Printf("[node-metrics] starting on node=%s api=%s", nodeName, apiURL)

	// Collect and post metrics every 3 seconds
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Collect immediately on startup, then on each tick
	for {
		report := collectAll(nodeName)
		report.Timestamp = time.Now().UTC().Format(time.RFC3339)

		body, err := json.Marshal(report)
		if err != nil {
			log.Printf("[node-metrics] marshal error: %v", err)
			<-ticker.C
			continue
		}

		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		if err != nil {
			log.Printf("[node-metrics] request error: %v", err)
			<-ticker.C
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if metricsKey != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", metricsKey))
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[node-metrics] post error: %v", err)
			<-ticker.C
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			log.Printf("[node-metrics] unexpected status: %d", resp.StatusCode)
		}

		<-ticker.C
	}
}
