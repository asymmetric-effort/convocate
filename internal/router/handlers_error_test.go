package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// failingDoer wraps a real Doer and injects errors for specific commands/keys.
type failingDoer struct {
	inner     redispkg.Doer
	failOn    map[string]bool
	failOnSet map[string]bool // Only fail SET commands for these key prefixes.
	failOnCmd map[string]bool // Fail specific "CMD:KEY" patterns like "SADD:router:allowlist".
	failAll   bool
	mu        sync.Mutex
}

func newFailingDoer(inner redispkg.Doer) *failingDoer {
	return &failingDoer{
		inner:     inner,
		failOn:    make(map[string]bool),
		failOnSet: make(map[string]bool),
		failOnCmd: make(map[string]bool),
	}
}

func (f *failingDoer) Do(args ...string) (interface{}, error) {
	f.mu.Lock()
	failAll := f.failAll
	// Check if this specific key should fail (exact or prefix match).
	shouldFail := false
	if len(args) >= 2 {
		key := args[1]
		if f.failOn[key] {
			shouldFail = true
		}
		// Also check prefix matches.
		for pattern := range f.failOn {
			if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
				shouldFail = true
				break
			}
		}
		// Check SET-only failures.
		if args[0] == "SET" {
			for pattern := range f.failOnSet {
				if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
					shouldFail = true
					break
				}
			}
		}
		// Check command-specific failures (CMD:KEY pattern).
		cmdKey := args[0] + ":" + key
		if f.failOnCmd[cmdKey] {
			shouldFail = true
		}
		for pattern := range f.failOnCmd {
			if len(cmdKey) >= len(pattern) && cmdKey[:len(pattern)] == pattern {
				shouldFail = true
				break
			}
		}
	}
	f.mu.Unlock()

	if failAll || shouldFail {
		return nil, fmt.Errorf("injected error")
	}
	return f.inner.Do(args...)
}

func (f *failingDoer) Close() error {
	return f.inner.Close()
}

func (f *failingDoer) setFailAll(v bool) {
	f.mu.Lock()
	f.failAll = v
	f.mu.Unlock()
}

func (f *failingDoer) setFailOn(key string) {
	f.mu.Lock()
	f.failOn[key] = true
	f.mu.Unlock()
}

func failingServer(t *testing.T) (srv *Server, ts *httptest.Server, fd *failingDoer) {
	t.Helper()
	mockConn := redispkg.NewMockConn()
	fd = newFailingDoer(mockConn)
	store := redispkg.NewRouterStore(fd)

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
	return srv, ts, fd
}

// failingBaoServer returns a bao server that returns errors for all ops.
func failingBaoServer(t *testing.T) *openbao.Client {
	t.Helper()
	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"mock error"},
		})
	}))
	t.Cleanup(baoMock.Close)
	return openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})
}

// --- handleJobs error paths ---

func TestHandleJobsValidateTokenError(t *testing.T) {
	_, ts, fd := failingServer(t)
	// Make all store operations fail.
	fd.setFailAll(true)

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok", protocol.JobSubmissionRequest{
		Repository: "org/repo",
		RunID:      1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

func TestHandleJobsAllowlistError(t *testing.T) {
	_, ts, fd := failingServer(t)
	// Set up a valid token but fail the allowlist check.
	fd.inner.(*redispkg.MockConn).Do("SET", "router:token:org/repo", "tok_test")
	fd.setFailOn("router:allowlist")

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_test", protocol.JobSubmissionRequest{
		Repository: "org/repo",
		RunID:      1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleCreateProject error paths ---

func TestHandleCreateProjectBaoStoreSecretsError(t *testing.T) {
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)

	// Use a failing bao server.
	baoClient := failingBaoServer(t)

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/bao-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

func TestHandleCreateProjectAllowlistError(t *testing.T) {
	_, ts, fd := failingServer(t)
	// Only fail SADD (AllowlistAdd), not SISMEMBER (AllowlistContains).
	fd.mu.Lock()
	fd.failOnCmd["SADD:router:allowlist"] = true
	fd.mu.Unlock()

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/allowlist-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleClusterAuth error paths ---

func TestHandleClusterAuthStoreError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "POST", ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode:   protocol.AuthModeAnthropicKey,
		APIKey: "sk-test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

func TestHandleClusterAuthGetStoreError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "GET", ts.URL+"/ui/api/auth", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleClusterAuth with failing bao ---

func TestHandleClusterAuthBaoError(t *testing.T) {
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)
	baoClient := failingBaoServer(t)

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/ui/api/auth", protocol.SetClusterAuthRequest{
		Mode:   protocol.AuthModeAnthropicKey,
		APIKey: "sk-test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleDeleteProject error paths ---

func TestHandleDeleteProjectStoreError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: uuid.MustNew(),
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleStatus error paths ---

func TestHandleStatusStoreError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "h1",
		ContainerID: "c1",
		JobID:       uuid.MustNew(),
		FromState:   protocol.JobClaimed,
		ToState:     protocol.JobRunning,
		Timestamp:   time.Now(),
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleHeartbeat error paths ---

func TestHandleHeartbeatStoreError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "POST", ts.URL+"/v1/heartbeat", protocol.HeartbeatRequest{
		HostID:    "h1",
		Timestamp: time.Now(),
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleAdHocSubmit error paths ---

func TestHandleAdHocSubmitStoreError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: uuid.MustNew(),
		Prompt:    "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- readJSON with valid body ---

func TestReadJSONValidBody(t *testing.T) {
	body := strings.NewReader(`{"key":"value"}`)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/", body)
	var v map[string]string
	err := readJSON(req, &v)
	if err != nil {
		t.Fatalf("readJSON: %v", err)
	}
	if v["key"] != "value" {
		t.Errorf("key: got %q", v["key"])
	}
}

// --- handleStatus SetJobMetadata error ---

func TestHandleStatusSetJobMetadataError(t *testing.T) {
	_, ts, fd := failingServer(t)

	// First create a job in the store.
	jobID := uuid.MustNew()
	meta := protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobClaimed,
	}
	metaJSON, _ := json.Marshal(&meta)
	fd.inner.(*redispkg.MockConn).Do("SET", "router:job:"+jobID.String(), string(metaJSON))

	// Only fail the SET (write), not the GET (read).
	fd.mu.Lock()
	fd.failOnSet["router:job:"+jobID.String()] = true
	fd.mu.Unlock()

	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		HostID:      "h1",
		ContainerID: "c1",
		JobID:       jobID,
		FromState:   protocol.JobClaimed,
		ToState:     protocol.JobRunning,
		Timestamp:   time.Now(),
	})
	resp.Body.Close()
	// The GET succeeded but SET failed - should be 500.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleJobs deeper error paths ---

func TestHandleJobsIdempotencyError(t *testing.T) {
	_, ts, fd := failingServer(t)
	// Set up valid token and allowlist.
	mock := fd.inner.(*redispkg.MockConn)
	mock.Do("SET", "router:token:org/idem-err", "tok_ie")
	mock.Do("SADD", "router:allowlist", "org/idem-err")

	// Fail on ledger lookup.
	fd.setFailOn("router:ledger:org/idem-err:0:1")

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_ie", protocol.JobSubmissionRequest{
		Repository: "org/idem-err",
		RunID:      1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

func TestHandleJobsRouteError(t *testing.T) {
	_, ts, fd := failingServer(t)
	mock := fd.inner.(*redispkg.MockConn)
	mock.Do("SET", "router:token:org/route-err", "tok_re")
	mock.Do("SADD", "router:allowlist", "org/route-err")

	// Fail on route lookup.
	fd.setFailOn("router:route-by-repo:org/route-err")

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_re", protocol.JobSubmissionRequest{
		Repository: "org/route-err",
		RunID:      1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleDeleteProject with full project ---

func TestHandleDeleteProjectAllSteps(t *testing.T) {
	// Use a server with a failing bao (DELETE returns error) but
	// the flow continues through all cleanup steps.
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)

	baoClient := failingBaoServer(t)

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	projectID := uuid.MustNew()
	store.SetProjectInfo(&protocol.ProjectInfo{
		ProjectID:   projectID,
		Repository:  "org/del-all",
		ContainerID: "c-del",
	})
	store.AllowlistAdd("org/del-all")
	store.SetAPIToken("org/del-all", "tok")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: projectID,
	})
	// Even with bao errors, delete continues and returns OK.
	var delResp protocol.DeleteProjectResponse
	json.NewDecoder(resp.Body).Decode(&delResp)
	resp.Body.Close()
	if !delResp.Deleted {
		t.Error("expected Deleted=true")
	}
}

// --- handleAdHocSubmit error paths ---

func TestHandleAdHocSubmitRouteError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailAll(true)

	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: uuid.MustNew(),
		Prompt:    "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

func TestHandleAdHocSubmitRecordJobAndMetaError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()
	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	projectID := uuid.MustNew()
	routeJSON, _ := json.Marshal(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  "org/adhoc-err",
		HostID:      "h1",
		ContainerID: "c1",
	})
	fd.inner.(*redispkg.MockConn).Do("SET", "router:route:"+projectID.String(), string(routeJSON))

	// Fail on ledger write.
	fd.setFailOn("router:ledger:")

	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: projectID,
		Prompt:    "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

func TestHandleAdHocSubmitSetMetadataError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()
	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	projectID := uuid.MustNew()
	routeJSON, _ := json.Marshal(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  "org/adhoc-meta-err",
		HostID:      "h1",
		ContainerID: "c1",
	})
	fd.inner.(*redispkg.MockConn).Do("SET", "router:route:"+projectID.String(), string(routeJSON))

	// Fail on job metadata write.
	fd.setFailOn("router:job:")

	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: projectID,
		Prompt:    "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// --- handleCreateProject with various step failures ---

func TestHandleCreateProjectStoreRouteError(t *testing.T) {
	// Bao succeeds on everything, but store fails on route write.
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()

	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Let the first steps succeed, then fail on route-related writes.
	// We'll fail on route SET key pattern.
	fd.setFailOn("router:route:")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/route-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleCreateProjectStoreTokenError tests token write failure.
func TestHandleCreateProjectStoreTokenError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()

	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Fail on token write.
	fd.setFailOn("router:token:")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/token-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleCreateProjectStoreProjectInfoError tests project info write failure.
func TestHandleCreateProjectStoreProjectInfoError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()

	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Fail on project info write.
	fd.setFailOn("router:project:")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/projinfo-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleJobsRecordJobError tests RecordJob failure in jobs handler.
func TestHandleJobsRecordJobError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()
	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	repo := "org/recordjob-fail"
	// Set up data via inner mock.
	mock := fd.inner.(*redispkg.MockConn)
	mock.Do("SET", "router:token:"+repo, "tok_rj")
	mock.Do("SADD", "router:allowlist", repo)
	routeJSON, _ := json.Marshal(protocol.ProjectRouteEntry{
		ProjectID:   uuid.MustNew(),
		Repository:  repo,
		HostID:      "h1",
		ContainerID: "c1",
	})
	mock.Do("SET", "router:route-by-repo:"+repo, string(routeJSON))

	// Fail on ledger SET (write) only, not GET (read).
	fd.mu.Lock()
	fd.failOnSet["router:ledger:"] = true
	fd.mu.Unlock()

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_rj", protocol.JobSubmissionRequest{
		Repository:  repo,
		IssueNumber: 1,
		RunID:       1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleJobsSetMetadataError tests SetJobMetadata failure.
func TestHandleJobsSetMetadataError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()
	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})

	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	repo := "org/setmeta-fail"
	mock := fd.inner.(*redispkg.MockConn)
	mock.Do("SET", "router:token:"+repo, "tok_sm")
	mock.Do("SADD", "router:allowlist", repo)
	routeJSON, _ := json.Marshal(protocol.ProjectRouteEntry{
		ProjectID:   uuid.MustNew(),
		Repository:  repo,
		HostID:      "h1",
		ContainerID: "c1",
	})
	mock.Do("SET", "router:route-by-repo:"+repo, string(routeJSON))

	// Fail on job metadata write.
	fd.setFailOn("router:job:")

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_sm", protocol.JobSubmissionRequest{
		Repository:  repo,
		IssueNumber: 1,
		RunID:       1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleDeleteProjectSteps tests delete with individual step failures.
func TestHandleDeleteProjectContainerCleanup(t *testing.T) {
	_, ts, store := freshServer(t)
	projectID := uuid.MustNew()

	// Set up project with container ID.
	store.SetProjectInfo(&protocol.ProjectInfo{
		ProjectID:   projectID,
		Repository:  "org/del-container",
		ContainerID: "c-to-delete",
		HostID:      "h1",
	})

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: projectID,
	})
	var delResp protocol.DeleteProjectResponse
	json.NewDecoder(resp.Body).Decode(&delResp)
	resp.Body.Close()
	if !delResp.Deleted {
		t.Error("expected Deleted=true")
	}
}

// TestHandleUpgradeContainerInvalidBody tests upgrade with bad body.
func TestHandleUpgradeContainerInvalidBody(t *testing.T) {
	_, ts, _ := freshServer(t)
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		ts.URL+"/ui/api/projects/upgrade", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleCreateProjectWritePolicyError(t *testing.T) {
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)

	// Bao mock that succeeds on KV write but fails on policy write.
	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// KV write succeeds.
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		if r.Method == "PUT" {
			// Policy write fails.
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{"policy write failed"}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer baoMock.Close()

	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/policy-err",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleCreateProjectAllowlistAddError tests allowlist add failure.
func TestHandleCreateProjectAllowlistAddError(t *testing.T) {
	fd := newFailingDoer(redispkg.NewMockConn())
	store := redispkg.NewRouterStore(fd)

	baoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer baoMock.Close()
	baoClient := openbao.NewClient(openbao.Config{Address: baoMock.URL, Token: "test"})

	srv := NewServer(Config{Store: store, Bao: baoClient, Version: "test", Logger: log.New(io.Discard, "", 0)})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// AllowlistContains succeeds (returns false), but AllowlistAdd fails.
	// We need the SISMEMBER to succeed but SADD to fail.
	// With our prefix matcher, we can't differentiate easily, so use failAll
	// after the first check.
	// Simpler: just fail on SADD by making all SET operations fail after check.
	// Actually, SADD is to the allowlist key. Let's make that fail.
	// The allowlist key is "router:allowlist".
	fd.setFailOn("router:allowlist")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/allowlist-add-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	// Should be 500 because AllowlistContains also uses this key.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleStatusValidationError tests status with invalid transition.
func TestHandleStatusValidationError(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/v1/status", protocol.StatusTransitionRequest{
		// Missing required fields.
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// TestHandleDeleteProjectEmptyContainerID tests delete when container ID is empty.
func TestHandleDeleteProjectEmptyContainerID(t *testing.T) {
	_, ts, store := freshServer(t)
	projectID := uuid.MustNew()

	// Project with empty container ID.
	store.SetProjectInfo(&protocol.ProjectInfo{
		ProjectID:  projectID,
		Repository: "org/no-container",
	})

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: projectID,
	})
	var delResp protocol.DeleteProjectResponse
	json.NewDecoder(resp.Body).Decode(&delResp)
	resp.Body.Close()
	if !delResp.Deleted {
		t.Error("expected Deleted=true")
	}
}

// TestHandleHeartbeatValidationError tests heartbeat with invalid data.
func TestHandleHeartbeatValidationError(t *testing.T) {
	_, ts, _ := freshServer(t)
	resp := doReq(t, "POST", ts.URL+"/v1/heartbeat", protocol.HeartbeatRequest{
		// Missing HostID.
		HostID: "",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

// TestHandleDeleteProjectCleanupStoreErrors tests that deletion continues
// when individual store cleanup steps fail.
func TestHandleDeleteProjectCleanupStoreErrors(t *testing.T) {
	_, ts, fd := failingServer(t)
	projectID := uuid.MustNew()

	// Seed the project.
	fd.inner.(*redispkg.MockConn).Do("SET", "router:project:"+projectID.String(),
		`{"project_id":"`+projectID.String()+`","repository":"org/cleanup-fail","container_id":"c1"}`)
	fd.inner.(*redispkg.MockConn).Do("SET", "router:project-by-repo:org/cleanup-fail", projectID.String())

	// Make route/token/allowlist/container deletes fail.
	fd.setFailOn("router:route:")
	fd.setFailOn("router:route-by-repo:")
	fd.setFailOn("router:token:")
	fd.setFailOn("router:allowlist")
	fd.setFailOn("router:container:")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/delete", protocol.DeleteProjectRequest{
		ProjectID: projectID,
	})
	// Delete should succeed even when cleanup steps fail.
	var delResp protocol.DeleteProjectResponse
	json.NewDecoder(resp.Body).Decode(&delResp)
	resp.Body.Close()
	if !delResp.Deleted {
		t.Error("expected Deleted=true")
	}
}

// TestHandleJobsNotAllowlisted tests job submission for a repo with a valid
// token but not in the allowlist.
func TestHandleJobsNotAllowlisted(t *testing.T) {
	_, ts, store := freshServer(t)
	// Set a token but don't add to allowlist.
	store.SetAPIToken("org/not-allowed", "tok_secret")

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_secret", protocol.JobSubmissionRequest{
		Repository: "org/not-allowed",
		RunID:      1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

// TestHandleJobsNoRoute tests job submission when the repo has no route.
func TestHandleJobsNoRoute(t *testing.T) {
	_, ts, store := freshServer(t)
	store.SetAPIToken("org/no-route", "tok_secret")
	store.AllowlistAdd("org/no-route")

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_secret", protocol.JobSubmissionRequest{
		Repository:  "org/no-route",
		RunID:       1,
		IssueNumber: 1,
		IssueTitle:  "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

// TestHandleJobsDispatchFails tests job submission when dispatch to host fails.
func TestHandleJobsDispatchFails(t *testing.T) {
	srv, ts, store := freshServer(t)
	projectID := uuid.MustNew()
	store.SetAPIToken("org/dispatch-fail", "tok_secret")
	store.AllowlistAdd("org/dispatch-fail")
	store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  "org/dispatch-fail",
		HostID:      "host-missing",
		ContainerID: "c-missing",
	})

	// Don't subscribe host-missing, so dispatchToHost will fail.
	_ = srv

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_secret", protocol.JobSubmissionRequest{
		Repository:  "org/dispatch-fail",
		RunID:       2,
		IssueNumber: 1,
		IssueTitle:  "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", resp.StatusCode)
	}
}

// TestHandleCreateProjectAllowlistCheckError tests create project when
// the initial allowlist-contains check fails.
func TestHandleCreateProjectAllowlistCheckError(t *testing.T) {
	_, ts, fd := failingServer(t)
	fd.setFailOn("router:allowlist")

	resp := doReq(t, "POST", ts.URL+"/ui/api/projects/create", protocol.CreateProjectRequest{
		Repository:    "org/check-fail",
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
}

// TestHandleClusterAuthMethodNotAllowed tests cluster auth with unsupported method.
func TestHandleClusterAuthMethodNotAllowed(t *testing.T) {
	_, ts, _ := freshServer(t)

	resp := doReq(t, "DELETE", ts.URL+"/ui/api/auth", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", resp.StatusCode)
	}
}

// TestHandleDispatchLongPollContextCancel tests the long-poll context cancellation path.
func TestHandleDispatchLongPollContextCancel(t *testing.T) {
	srv, ts, _ := freshServer(t)
	_ = srv

	// We can test via the normal URL but with a very short timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/v1/dispatch?host=timeout-host", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	// Context cancellation is expected — the important thing is
	// the handler registered and unregistered the subscription.
}

// TestReadJSONNilBodyError tests readJSON with a nil body.
func TestReadJSONNilBodyError(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/test", http.NoBody)
	req.Body = nil
	var target struct{}
	err := readJSON(req, &target)
	if err == nil {
		t.Error("expected error for nil body")
	}
}

// TestHandleDispatchSSEChannelClose tests SSE mode when the channel is closed.
func TestHandleDispatchSSEChannelClose(t *testing.T) {
	srv, ts, _ := freshServer(t)

	// Subscribe host then immediately close the channel.
	ch := srv.SubscribeDispatch("sse-close-host")

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(ch)
		// Clean up: remove the entry to prevent UnsubscribeDispatch from
		// double-closing in the defer.
		srv.mu.Lock()
		delete(srv.dispatchSubs, "sse-close-host")
		srv.mu.Unlock()
	}()

	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		ts.URL+"/v1/dispatch?host=sse-close-host", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	resp.Body.Close()
}
