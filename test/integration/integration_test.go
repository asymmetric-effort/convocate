//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/router"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func setupIntegrationServer(t *testing.T) (*router.Server, *httptest.Server, *redis.RouterStore) {
	t.Helper()

	mockConn := redis.NewMockConn()
	store := redis.NewRouterStore(mockConn)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	t.Cleanup(baoMock.Close)

	baoClient := openbao.NewClient(openbao.Config{
		Address: baoMock.URL,
		Token:   "test",
	})

	srv := router.NewServer(router.Config{
		Store:   store,
		Bao:     baoClient,
		Version: "integration-test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return srv, ts, store
}

func TestRouterToRedisIntegration(t *testing.T) {
	_, ts, store := setupIntegrationServer(t)

	// Create a project via the API.
	resp := doPost(t, ts.URL+"/ui/api/projects/create", nil, protocol.CreateProjectRequest{
		Repository:    "integ/repo",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create project: got %d, body: %s", resp.StatusCode, body)
	}

	var createResp protocol.CreateProjectResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	resp.Body.Close()

	// Verify allowlist.
	allowed, err := store.AllowlistContains("integ/repo")
	if err != nil {
		t.Fatalf("allowlist check: %v", err)
	}
	if !allowed {
		t.Error("repo should be in allowlist after create")
	}

	// Verify route.
	route, err := store.GetRouteByRepo("integ/repo")
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if route == nil {
		t.Fatal("route should exist after create")
	}
}

func TestDispatchToRouterIntegration(t *testing.T) {
	_, ts, store := setupIntegrationServer(t)

	jobID := uuid.MustNew()
	store.SetJobMetadata(protocol.JobMetadata{
		JobID:       jobID,
		Repository:  "test/repo",
		Status:      protocol.JobClaimed,
		HostID:      "host-1",
		ContainerID: "c1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	resp := doPost(t, ts.URL+"/v1/status", nil, protocol.StatusTransitionRequest{
		HostID:      "host-1",
		ContainerID: "c1",
		JobID:       jobID,
		FromState:   protocol.JobClaimed,
		ToState:     protocol.JobRunning,
		Timestamp:   time.Now(),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status transition: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	meta, err := store.GetJobMetadata(jobID)
	if err != nil {
		t.Fatalf("get job metadata: %v", err)
	}
	if meta.Status != protocol.JobRunning {
		t.Errorf("status: got %q, want %q", meta.Status, protocol.JobRunning)
	}
}

func TestHeartbeatIntegration(t *testing.T) {
	_, ts, store := setupIntegrationServer(t)

	resp := doPost(t, ts.URL+"/v1/heartbeat", nil, protocol.HeartbeatRequest{
		HostID:         "host-1",
		ContainerCount: 3,
		CPUPercent:     50.0,
		MemoryPercent:  70.0,
		Timestamp:      time.Now(),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	hb, err := store.GetHeartbeat("host-1")
	if err != nil {
		t.Fatalf("get heartbeat: %v", err)
	}
	if hb == nil {
		t.Fatal("heartbeat not cached")
	}
	if hb.ContainerCount != 3 {
		t.Errorf("container count: got %d, want 3", hb.ContainerCount)
	}
}

func doPost(t *testing.T, url string, headers map[string]string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}
