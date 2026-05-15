//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
)

var routerURL string

func TestMain(m *testing.M) {
	routerURL = os.Getenv("CONVOCATE_ROUTER_URL")
	if routerURL == "" {
		routerURL = "https://localhost:8443"
	}
	os.Exit(m.Run())
}

func TestE2EHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(routerURL + "/v1/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status: got %d, want 200", resp.StatusCode)
	}

	var health protocol.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&health)
	if err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("health.Status: got %q, want %q", health.Status, "ok")
	}
}

func TestE2EJobSubmissionWithoutToken(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	data, _ := json.Marshal(protocol.JobSubmissionRequest{
		Repository: "test/repo",
		RunID:      1,
	})
	resp, err := client.Post(routerURL+"/v1/jobs", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("job submission failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestE2EJobSubmissionUnknownRepo(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	data, _ := json.Marshal(protocol.JobSubmissionRequest{
		Repository: "unknown/repo",
		RunID:      1,
	})
	req, _ := http.NewRequest("POST", routerURL+"/v1/jobs", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer bad-token")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("job submission failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}
