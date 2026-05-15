package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func testServer(t *testing.T) (srv *Server, ts *httptest.Server) {
	t.Helper()

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "POST" || r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(baoMock.Close)

	baoClient := openbao.NewClient(openbao.Config{
		Address: baoMock.URL,
		Token:   "test",
	})

	srv = NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test-v0.2.0",
		Logger:  log.New(io.Discard, "", 0),
	})

	ts = httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return srv, ts
}

func postJSON(url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func postJSONWithAuth(url, token string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(v)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- Health ---

func TestHealthEndpoint(t *testing.T) {
	_, ts := testServer(t)

	req, err := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/v1/health", http.NoBody)
	if err != nil {
		t.Fatalf("GET /v1/health request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/health error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}

	var health protocol.HealthResponse
	decodeJSON(t, resp, &health)
	if health.Status != "ok" {
		t.Errorf("status: got %q, want %q", health.Status, "ok")
	}
	if health.Version != "test-v0.2.0" {
		t.Errorf("version: got %q, want %q", health.Version, "test-v0.2.0")
	}
}

func TestHealthMethodNotAllowed(t *testing.T) {
	_, ts := testServer(t)
	postReq, err := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/v1/health", http.NoBody)
	if err != nil {
		t.Fatalf("POST /v1/health request: %v", err)
	}
	postReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatalf("POST /v1/health error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", resp.StatusCode)
	}
}

// --- Job Submission ---

func TestJobSubmissionNoToken(t *testing.T) {
	_, ts := testServer(t)
	resp, err := postJSON(ts.URL+"/v1/jobs", protocol.JobSubmissionRequest{
		Repository: "org/repo",
		RunID:      123,
	})
	if err != nil {
		t.Fatalf("POST /v1/jobs error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestJobSubmissionRepoNotFound(t *testing.T) {
	_, ts := testServer(t)
	resp, err := postJSONWithAuth(ts.URL+"/v1/jobs", "bad-token", protocol.JobSubmissionRequest{
		Repository: "org/repo",
		RunID:      123,
	})
	if err != nil {
		t.Fatalf("POST /v1/jobs error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestJobSubmissionFullFlow(t *testing.T) {
	srv, ts := testServer(t)

	repo := "org/test-repo"
	projectID := uuid.MustNew()

	// Set up: allowlist, token, route.
	srv.store.AllowlistAdd(repo)
	srv.store.SetAPIToken(repo, "tok_test")
	srv.store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  repo,
		HostID:      "host-1",
		ContainerID: "container-abc",
	})

	// Subscribe for dispatch events.
	ch := srv.SubscribeDispatch("host-1")
	defer srv.UnsubscribeDispatch("host-1")

	resp, err := postJSONWithAuth(ts.URL+"/v1/jobs", "tok_test", protocol.JobSubmissionRequest{
		Repository:  repo,
		IssueNumber: 42,
		IssueTitle:  "Fix bug",
		IssueBody:   "It crashes",
		IssueAuthor: "alice",
		RunID:       12345,
	})
	if err != nil {
		t.Fatalf("POST /v1/jobs error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, body)
	}

	var jobResp protocol.JobSubmissionResponse
	decodeJSON(t, resp, &jobResp)

	if jobResp.JobID.IsZero() {
		t.Error("JobID is zero")
	}
	if jobResp.Duplicate {
		t.Error("Duplicate should be false for first submission")
	}
	if jobResp.Repository != repo {
		t.Errorf("Repository: got %q, want %q", jobResp.Repository, repo)
	}

	// Check dispatch event was received.
	select {
	case event := <-ch:
		if event.JobID != jobResp.JobID {
			t.Errorf("dispatch event JobID: got %s, want %s", event.JobID, jobResp.JobID)
		}
		if event.IssueNumber != 42 {
			t.Errorf("dispatch event IssueNumber: got %d, want 42", event.IssueNumber)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for dispatch event")
	}
}

func TestJobSubmissionIdempotency(t *testing.T) {
	srv, ts := testServer(t)

	repo := "org/idem-repo"
	projectID := uuid.MustNew()
	srv.store.AllowlistAdd(repo)
	srv.store.SetAPIToken(repo, "tok_idem")
	srv.store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  repo,
		HostID:      "host-1",
		ContainerID: "container-idem",
	})
	srv.SubscribeDispatch("host-1")
	defer srv.UnsubscribeDispatch("host-1")

	req := protocol.JobSubmissionRequest{
		Repository:  repo,
		IssueNumber: 1,
		RunID:       999,
	}

	// First submission.
	resp1, err := postJSONWithAuth(ts.URL+"/v1/jobs", "tok_idem", req)
	if err != nil {
		t.Fatalf("first submit error: %v", err)
	}
	var result1 protocol.JobSubmissionResponse
	decodeJSON(t, resp1, &result1)
	if result1.Duplicate {
		t.Error("first submission should not be duplicate")
	}

	// Second submission with same run_id — should deduplicate.
	resp2, err := postJSONWithAuth(ts.URL+"/v1/jobs", "tok_idem", req)
	if err != nil {
		t.Fatalf("second submit error: %v", err)
	}
	var result2 protocol.JobSubmissionResponse
	decodeJSON(t, resp2, &result2)
	if !result2.Duplicate {
		t.Error("second submission should be duplicate")
	}
	if result2.JobID != result1.JobID {
		t.Errorf("duplicate should return same JobID: got %s, want %s", result2.JobID, result1.JobID)
	}
}

// --- Status Transitions ---

func TestStatusTransition(t *testing.T) {
	_, ts := testServer(t)
	srv, _ := testServer(t) // Fresh server for metadata.
	_ = srv

	// We'll use the same ts since it has a fresh store.
	// Set up a job directly in the store.
	srvDirect, tsDirect := testServer(t)
	jobID := uuid.MustNew()
	srvDirect.store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobClaimed,
	})

	resp, err := postJSON(tsDirect.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "host-1",
		ContainerID: "container-abc",
		JobID:       jobID,
		FromState:   protocol.JobClaimed,
		ToState:     protocol.JobRunning,
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("POST /v1/status error: %v", err)
	}

	var statusResp protocol.StatusTransitionResponse
	decodeJSON(t, resp, &statusResp)
	if !statusResp.Accepted {
		t.Errorf("expected accepted, got error: %s", statusResp.Error)
	}

	_ = ts // suppress unused
}

func TestStatusTransitionInvalidTransition(t *testing.T) {
	srvDirect, tsDirect := testServer(t)
	jobID := uuid.MustNew()
	srvDirect.store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobComplete,
	})

	resp, err := postJSON(tsDirect.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "host-1",
		ContainerID: "container-abc",
		JobID:       jobID,
		FromState:   protocol.JobComplete,
		ToState:     protocol.JobRunning,
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("POST /v1/status error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestStatusTransitionStateMismatch(t *testing.T) {
	srvDirect, tsDirect := testServer(t)
	jobID := uuid.MustNew()
	srvDirect.store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobRunning,
	})

	resp, err := postJSON(tsDirect.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "host-1",
		ContainerID: "c1",
		JobID:       jobID,
		FromState:   protocol.JobClaimed, // Wrong: job is running, not claimed.
		ToState:     protocol.JobRunning, // claimed->running is valid transition, but from_state doesn't match.
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("POST /v1/status error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status: got %d, want 409", resp.StatusCode)
	}
}

// --- Heartbeat ---

func TestHeartbeat(t *testing.T) {
	_, ts := testServer(t)

	resp, err := postJSON(ts.URL+"/v1/heartbeat", protocol.HeartbeatRequest{
		HostID:         "host-1",
		ContainerCount: 3,
		CPUPercent:     45.5,
		MemoryPercent:  60.0,
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("POST /v1/heartbeat error: %v", err)
	}

	var hbResp protocol.HeartbeatResponse
	decodeJSON(t, resp, &hbResp)
	if !hbResp.Accepted {
		t.Error("heartbeat not accepted")
	}
}

func TestHeartbeatInvalid(t *testing.T) {
	_, ts := testServer(t)

	resp, err := postJSON(ts.URL+"/v1/heartbeat", protocol.HeartbeatRequest{
		HostID: "", // Missing required field.
	})
	if err != nil {
		t.Fatalf("POST /v1/heartbeat error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- Create Project ---

func TestCreateProject(t *testing.T) {
	_, ts := testServer(t)

	resp, err := postJSON(ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/new-repo",
		SSHPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----",
		GitHubPAT:     "ghp_abc123",
	})
	if err != nil {
		t.Fatalf("POST create project error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status: got %d, want 201. body: %s", resp.StatusCode, body)
	}

	var createResp protocol.CreateProjectResponse
	decodeJSON(t, resp, &createResp)

	if createResp.ProjectID.IsZero() {
		t.Error("ProjectID is zero")
	}
	if createResp.Repository != "org/new-repo" {
		t.Errorf("Repository: got %q, want %q", createResp.Repository, "org/new-repo")
	}
	if createResp.APIToken == "" {
		t.Error("APIToken is empty")
	}
}

func TestCreateProjectDuplicate(t *testing.T) {
	srv, ts := testServer(t)

	srv.store.AllowlistAdd("org/existing")

	resp, err := postJSON(ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/existing",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	if err != nil {
		t.Fatalf("POST create project error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status: got %d, want 409", resp.StatusCode)
	}
}

// --- Delete Project ---

func TestDeleteProject(t *testing.T) {
	srv, ts := testServer(t)
	projectID := uuid.MustNew()

	srv.store.SetProjectInfo(&protocol.ProjectInfo{
		ProjectID:   projectID,
		Repository:  "org/to-delete",
		ContainerID: "c1",
	})
	srv.store.AllowlistAdd("org/to-delete")
	srv.store.SetAPIToken("org/to-delete", "tok")

	resp, err := postJSON(ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("POST delete project error: %v", err)
	}

	var deleteResp protocol.DeleteProjectResponse
	decodeJSON(t, resp, &deleteResp)
	if !deleteResp.Deleted {
		t.Error("expected Deleted=true")
	}
}

func TestDeleteProjectNotFound(t *testing.T) {
	_, ts := testServer(t)

	resp, err := postJSON(ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: uuid.MustNew(),
	})
	if err != nil {
		t.Fatalf("POST delete project error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

// --- Cluster Auth ---

func TestClusterAuth(t *testing.T) {
	_, ts := testServer(t)

	resp, err := postJSON(ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode:   protocol.AuthModeAnthropicKey,
		APIKey: "sk-ant-test",
	})
	if err != nil {
		t.Fatalf("POST cluster auth error: %v", err)
	}

	var authResp protocol.SetClusterAuthResponse
	decodeJSON(t, resp, &authResp)
	if !authResp.Updated {
		t.Error("expected Updated=true")
	}
	if authResp.Mode != protocol.AuthModeAnthropicKey {
		t.Errorf("Mode: got %q, want %q", authResp.Mode, protocol.AuthModeAnthropicKey)
	}
}

func TestClusterAuthInvalidMode(t *testing.T) {
	_, ts := testServer(t)

	resp, err := postJSON(ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode: "invalid",
	})
	if err != nil {
		t.Fatalf("POST cluster auth error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- Ad-hoc submission ---

func TestAdHocSubmission(t *testing.T) {
	srv, ts := testServer(t)
	projectID := uuid.MustNew()

	srv.store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  "org/adhoc-repo",
		HostID:      "host-1",
		ContainerID: "c-adhoc",
	})
	ch := srv.SubscribeDispatch("host-1")
	defer srv.UnsubscribeDispatch("host-1")

	resp, err := postJSON(ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: projectID,
		Prompt:    "Add a health check endpoint",
	})
	if err != nil {
		t.Fatalf("POST adhoc error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, body)
	}

	var adhocResp protocol.AdHocSubmissionResponse
	decodeJSON(t, resp, &adhocResp)
	if adhocResp.JobID.IsZero() {
		t.Error("JobID is zero")
	}

	// Check dispatch event.
	select {
	case event := <-ch:
		if !event.AdHoc {
			t.Error("expected AdHoc=true in dispatch event")
		}
		if event.Prompt != "Add a health check endpoint" {
			t.Errorf("Prompt: got %q", event.Prompt)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for dispatch event")
	}
}

// --- Dispatch subscription ---

func TestDispatchSubscription(t *testing.T) {
	srv, _ := testServer(t)

	ch := srv.SubscribeDispatch("host-1")
	defer srv.UnsubscribeDispatch("host-1")

	event := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "c1",
		Repository:  "org/repo",
		IssueNumber: 1,
	}

	err := srv.dispatchToHost("host-1", &event)
	if err != nil {
		t.Fatalf("dispatchToHost error: %v", err)
	}

	select {
	case got := <-ch:
		if got.JobID != event.JobID {
			t.Errorf("JobID: got %s, want %s", got.JobID, event.JobID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestDispatchToUnsubscribedHost(t *testing.T) {
	srv, _ := testServer(t)
	err := srv.dispatchToHost("nonexistent", &protocol.DispatchEvent{})
	if err == nil {
		t.Error("expected error for unsubscribed host")
	}
}

// --- Helper tests ---

func TestExtractBearerToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok_abc")
	got := extractBearerToken(req)
	if got != "tok_abc" {
		t.Errorf("got %q, want %q", got, "tok_abc")
	}
}

func TestExtractBearerTokenMissing(t *testing.T) {
	req := httptest.NewRequest("GET", "/", http.NoBody)
	got := extractBearerToken(req)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestGenerateAPIToken(t *testing.T) {
	token := generateAPIToken()
	if len(token) < 10 {
		t.Errorf("token too short: %q", token)
	}
	if token[:4] != "cvt_" {
		t.Errorf("token prefix: got %q, want cvt_", token[:4])
	}
}
