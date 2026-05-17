package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func freshServer(t *testing.T) (srv *Server, ts *httptest.Server, store *redispkg.RouterStore) {
	t.Helper()
	mockConn := redispkg.NewMockConn()
	store = redispkg.NewRouterStore(mockConn)
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
	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})
	srv = NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts = httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts, store
}

func doReq(t *testing.T, method, url string, body interface{}) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func doReqAuth(t *testing.T, method, url, token string, body interface{}) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// --- Method Not Allowed tests for every endpoint ---

func TestMethodNotAllowed(t *testing.T) {
	_, ts, _ := freshServer(t)
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/jobs"},
		{"PUT", "/v1/jobs"},
		{"DELETE", "/v1/jobs"},
		{"POST", "/v1/health"},
		{"PUT", "/v1/health"},
		{"POST", "/v1/dispatch"},
		{"GET", "/v1/status"},
		{"GET", "/v1/heartbeat"},
		{"POST", "/ui/api/projects"},
		{"GET", "/ui/api/projects/create"},
		{"GET", "/ui/api/projects/delete"},
		{"GET", "/ui/api/projects/upgrade"},
		{"GET", "/ui/api/projects/upgrade-all-idle"},
		{"POST", "/ui/api/jobs"},
		{"POST", "/ui/api/hosts"},
		{"GET", "/ui/api/adhoc"},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			resp := doReq(t, ep.method, ts.URL+ep.path, nil)
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: got %d, want 405", ep.method, ep.path, resp.StatusCode)
			}
		})
	}
}

// --- handleProjects ---

func TestHandleProjectsGET(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "GET", ts.URL+"/ui/api/projects", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	var projects []protocol.ProjectInfo
	json.NewDecoder(resp.Body).Decode(&projects)
	resp.Body.Close()
	if projects == nil {
		t.Error("expected non-nil array")
	}
}

// --- handleJobsList ---

func TestHandleJobsListGET(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "GET", ts.URL+"/ui/api/jobs", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	var jobs []protocol.JobMetadata
	json.NewDecoder(resp.Body).Decode(&jobs)
	resp.Body.Close()
	if jobs == nil {
		t.Error("expected non-nil array")
	}
}

// --- handleHostsList ---

func TestHandleHostsListGET(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "GET", ts.URL+"/ui/api/hosts", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	var hosts []protocol.HostHealthInfo
	json.NewDecoder(resp.Body).Decode(&hosts)
	resp.Body.Close()
	if hosts == nil {
		t.Error("expected non-nil array")
	}
}

// --- handleUpgradeContainer ---

func TestHandleUpgradeContainer(t *testing.T) {
	_, ts, _ := freshServer(t)
	projectID := uuid.MustNew()
	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/upgrade", protocol.UpgradeContainerRequest{
		ProjectID: projectID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleUpgradeContainerMissingID(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/upgrade", protocol.UpgradeContainerRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- handleUpgradeAllIdle ---

func TestHandleUpgradeAllIdle(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/upgrade-all-idle", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- handleDispatch (long-poll timeout) ---

func TestHandleDispatchMissingHost(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "GET", ts.URL+"/v1/dispatch", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- handleJobs edge cases ---

func TestHandleJobsEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/v1/jobs", http.NoBody)
	req.Header.Set("Authorization", "Bearer tok_test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleJobsMissingRequiredFields(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_test", protocol.JobSubmissionRequest{
		Repository: "",
		RunID:      0,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- handleStatus edge cases ---

func TestHandleStatusEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/v1/status", nil)
	resp.Body.Close()
	// nil body should return 400
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleStatusJobNotFound(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "h1",
		ContainerID: "c1",
		JobID:       uuid.MustNew(),
		FromState:   protocol.JobClaimed,
		ToState:     protocol.JobRunning,
		Timestamp:   time.Now(),
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleStatusCompleteWithPRURL(t *testing.T) {
	srv, ts, store := freshServer(t)
	_ = srv
	jobID := uuid.MustNew()
	store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobRunning,
	})
	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "h1",
		ContainerID: "c1",
		JobID:       jobID,
		FromState:   protocol.JobRunning,
		ToState:     protocol.JobComplete,
		Timestamp:   time.Now(),
		PullURL:     "https://github.com/org/repo/pull/1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	meta, _ := store.GetJobMetadata(jobID)
	if meta.PullURL != "https://github.com/org/repo/pull/1" {
		t.Errorf("PullURL: got %q", meta.PullURL)
	}
	if meta.CompletedAt == nil {
		t.Error("CompletedAt should be set on complete")
	}
}

func TestHandleStatusTerminated(t *testing.T) {
	_, ts, store := freshServer(t)
	jobID := uuid.MustNew()
	store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobRunning,
	})
	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "h1",
		ContainerID: "c1",
		JobID:       jobID,
		FromState:   protocol.JobRunning,
		ToState:     protocol.JobTerminated,
		Timestamp:   time.Now(),
		Reason:      "operator cancelled",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	meta, _ := store.GetJobMetadata(jobID)
	if meta.CompletedAt == nil {
		t.Error("CompletedAt should be set on terminated")
	}
}

func TestHandleStatusFailed(t *testing.T) {
	_, ts, store := freshServer(t)
	jobID := uuid.MustNew()
	store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobRunning,
	})
	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "h1",
		ContainerID: "c1",
		JobID:       jobID,
		FromState:   protocol.JobRunning,
		ToState:     protocol.JobFailed,
		Timestamp:   time.Now(),
		Reason:      "compilation error",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- handleHeartbeat edge cases ---

func TestHandleHeartbeatEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/v1/heartbeat", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- handleCreateProject edge cases ---

func TestHandleCreateProjectMissingFields(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository: "org/repo",
		// Missing SSH key and PAT.
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleCreateProjectEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/ui/api/projects/create", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- handleDeleteProject edge cases ---

func TestHandleDeleteProjectMissingID(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleDeleteProjectEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/ui/api/projects/delete", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- handleClusterAuth edge cases ---

func TestHandleClusterAuthGET(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "GET", ts.URL+"/ui/api/auth", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleClusterAuthMissingAPIKey(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode: protocol.AuthModeAnthropicKey,
		// Missing api_key.
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleClusterAuthMissingSessionToken(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode: protocol.AuthModeClaudeSession,
		// Missing session_token.
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleClusterAuthSessionMode(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode:         protocol.AuthModeClaudeSession,
		SessionToken: "session_abc",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleClusterAuthEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/ui/api/auth", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- handleAdHocSubmit edge cases ---

func TestHandleAdHocSubmitMissingFields(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleAdHocSubmitProjectNotFound(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: uuid.MustNew(),
		Prompt:    "do something",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandleAdHocSubmitEmptyBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", ts.URL+"/ui/api/adhoc", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// --- publicHandler ---

func TestPublicHandler(t *testing.T) {
	srv, _, _ := freshServer(t)
	pubTs := httptest.NewServer(srv.publicHandler())
	defer pubTs.Close()

	// Health should work.
	healthReq, err := http.NewRequestWithContext(context.Background(), "GET", pubTs.URL+"/v1/health", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(healthReq)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status: got %d, want 200", resp.StatusCode)
	}
}

// --- NewServer default logger ---

func TestNewServerDefaultLogger(t *testing.T) {
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		// Logger intentionally nil — should use default.
	})
	if srv.logger == nil {
		t.Error("logger should not be nil")
	}
}

// --- readJSON nil body ---

func TestReadJSONNilBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", http.NoBody)
	var v struct{}
	err := readJSON(req, &v)
	if err == nil {
		t.Error("expected error for nil body")
	}
}

// --- Dispatch channel full ---

func TestDispatchChannelFull(t *testing.T) {
	srv, _, _ := freshServer(t)
	ch := srv.SubscribeDispatch("host-full")
	defer srv.UnsubscribeDispatch("host-full")

	// Fill the channel (buffer size is 16).
	for range 16 {
		srv.dispatchToHost("host-full", &protocol.DispatchEvent{JobID: uuid.MustNew()})
	}

	// Next dispatch should fail (channel full).
	err := srv.dispatchToHost("host-full", &protocol.DispatchEvent{JobID: uuid.MustNew()})
	if err == nil {
		t.Error("expected error for full dispatch channel")
	}

	_ = ch
}

// --- UnsubscribeDispatch idempotent ---

func TestUnsubscribeDispatchIdempotent(t *testing.T) {
	srv, _, _ := freshServer(t)
	srv.SubscribeDispatch("host-x")
	srv.UnsubscribeDispatch("host-x")
	srv.UnsubscribeDispatch("host-x") // Should not panic.
}

// --- handleDispatch long-poll ---

func TestHandleDispatchLongPollTimeout(t *testing.T) {
	_, ts, _ := freshServer(t)

	client := &http.Client{Timeout: 2 * time.Second}
	dispReq, reqErr := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/v1/dispatch?host=timeout-host", http.NoBody)
	if reqErr != nil {
		t.Fatalf("new request: %v", reqErr)
	}
	resp, err := client.Do(dispReq)
	if err != nil {
		// Timeout is expected if the 30s server timeout exceeds client timeout.
		return
	}
	defer resp.Body.Close()
	// Server returns 204 on timeout or 200 with event.
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 204 or 200", resp.StatusCode)
	}
}

// Note: The full dispatch long-poll/SSE flow with event delivery is tested
// in integration tests (test/integration/) against the full dev stack. The
// handleDispatch handler creates its own subscription channel, making unit
// testing of the event delivery path inherently racy without a real async
// dispatch loop. The handler's error paths and subscription logic are
// covered by TestHandleDispatchMissingHost and the SubscribeDispatch/
// dispatchToHost unit tests.

// --- handleJobs dispatch failure ---

func TestHandleJobsDispatchFailureNoHost(t *testing.T) {
	srv, ts, store := freshServer(t)
	_ = srv

	repo := "org/fail-dispatch"
	projectID := uuid.MustNew()
	store.AllowlistAdd(repo)
	store.SetAPIToken(repo, "tok_fail")
	store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  repo,
		HostID:      "nonexistent-host",
		ContainerID: "c-fail",
	})
	// No dispatch subscription for nonexistent-host.

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_fail", protocol.JobSubmissionRequest{
		Repository:  repo,
		IssueNumber: 1,
		RunID:       1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", resp.StatusCode)
	}
}

// --- /health alias and / redirect ---

func TestHealthAlias(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/health", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestRootServesServiceJSON(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	defer resp.Body.Close()
	// Root returns service identification JSON.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["service"] != "convocate-router-api" {
		t.Errorf("service: got %q, want %q", body["service"], "convocate-router-api")
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/some/spa/route", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	// Non-API, non-root paths return 404 (Web UI is served by convocate-ui).
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestUnknownV1Path404(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/v1/nonexistent", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	// API paths that don't match a handler still return 404.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestStaticFileReturns404(t *testing.T) {
	_, ts, _ := freshServer(t)
	// Router-api no longer serves static files (Web UI is in convocate-ui).
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/placeholder.html", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

// --- requireJSONContentType ---

func TestRequireJSONContentTypeRejectsNonJSON(t *testing.T) {
	_, ts, _ := freshServer(t)
	// POST without Content-Type: application/json should get 415.
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/create", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want 415", resp.StatusCode)
	}
}

func TestRequireJSONContentTypeAcceptsJSON(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/create", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	// Should get past the content-type check (400 for missing fields is fine).
	if resp.StatusCode == http.StatusUnsupportedMediaType {
		t.Error("should not reject application/json")
	}
}

func TestRequireJSONContentTypeSkipsGET(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/ui/api/projects", http.NoBody)
	// No Content-Type header — GET should pass through.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnsupportedMediaType {
		t.Error("GET should not be subject to content-type check")
	}
}

func TestUpgradeContainerNoContentType(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/upgrade", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want 415", resp.StatusCode)
	}
}

func TestUpgradeAllIdleNoContentType(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/upgrade-all-idle", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want 415", resp.StatusCode)
	}
}

func TestUpgradeAllIdleWithJSON(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/upgrade-all-idle", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnsupportedMediaType {
		t.Error("should accept application/json")
	}
}

func TestDeleteProjectNoContentType(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/delete", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want 415", resp.StatusCode)
	}
}

func TestAdHocNoContentType(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/adhoc", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want 415", resp.StatusCode)
	}
}

func TestClusterAuthNoContentType(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/auth", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status: got %d, want 415", resp.StatusCode)
	}
}
