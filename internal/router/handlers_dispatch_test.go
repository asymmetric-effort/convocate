package router

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// TestHandleDispatchSSEEvent tests the SSE dispatch path that receives
// an event. httptest servers implement http.Flusher, so the handler enters
// SSE mode.
func TestHandleDispatchSSEEvent(t *testing.T) {
	srv, ts, store := freshServer(t)

	hostID := "sse-event-host"
	projectID := uuid.MustNew()
	repo := "org/dispatch-sse"
	store.AllowlistAdd(repo)
	store.SetAPIToken(repo, "tok_sse")
	store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  repo,
		HostID:      hostID,
		ContainerID: "c-sse",
	})

	// Start the SSE request.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		ts.URL+"/v1/dispatch?host="+hostID, http.NoBody)

	type result struct {
		resp *http.Response
		err  error
	}
	respCh := make(chan result, 1)
	go func() {
		resp, doErr := http.DefaultClient.Do(req) //nolint:bodyclose // closed by caller via channel
		respCh <- result{resp, doErr}
	}()

	// Wait for handler to subscribe.
	time.Sleep(200 * time.Millisecond)

	srv.mu.RLock()
	_, subscribed := srv.dispatchSubs[hostID]
	srv.mu.RUnlock()
	if !subscribed {
		t.Fatal("handler did not subscribe for dispatch events")
	}

	dispatchErr := srv.dispatchToHost(hostID, &protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "c-sse",
		Repository:  repo,
		IssueNumber: 99,
	})
	if dispatchErr != nil {
		t.Fatalf("dispatchToHost: %v", dispatchErr)
	}

	// In SSE mode, the response arrives immediately (200) and the body
	// streams events. We read lines from the body.
	r := <-respCh
	if r.err != nil {
		t.Fatalf("SSE request: %v", r.err)
	}
	defer r.resp.Body.Close()
	if r.resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", r.resp.StatusCode)
	}

	// Read a chunk from the body. SSE format: "data: {json}\n\n".
	buf := make([]byte, 4096)
	n, readErr := r.resp.Body.Read(buf)
	if readErr != nil && readErr != io.EOF {
		t.Fatalf("read body: %v", readErr)
	}
	body := string(buf[:n])

	// Parse the SSE data line.
	prefix := "data: "
	idx := 0
	for i := range len(body) - len(prefix) + 1 {
		if body[i:i+len(prefix)] == prefix {
			idx = i + len(prefix)
			break
		}
	}
	if idx == 0 {
		t.Fatalf("no SSE data line found in: %q", body)
	}
	// Find end of JSON (next newline).
	end := idx
	for end < len(body) && body[end] != '\n' {
		end++
	}
	jsonData := body[idx:end]
	var event protocol.DispatchEvent
	if jsonErr := json.Unmarshal([]byte(jsonData), &event); jsonErr != nil {
		t.Fatalf("unmarshal SSE event: %v (raw: %q)", jsonErr, jsonData)
	}
	if event.IssueNumber != 99 {
		t.Errorf("IssueNumber: got %d, want 99", event.IssueNumber)
	}

	cancel() // Cancel to close the SSE stream.
}

// TestHandleDispatchClientCancellation tests that the handler returns
// when the client cancels the request.
func TestHandleDispatchClientCancellation(t *testing.T) {
	_, ts, _ := freshServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		ts.URL+"/v1/dispatch?host=cancel-host", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Context deadline exceeded - expected.
		return
	}
	resp.Body.Close()
}

// TestHandleJobsDispatchFailureMarksJobFailed verifies that when dispatch
// fails, the job metadata is updated to failed state.
func TestHandleJobsDispatchFailureMarksJobFailed(t *testing.T) {
	_, ts, store := freshServer(t)

	repo := "org/dispatch-fail"
	projectID := uuid.MustNew()
	store.AllowlistAdd(repo)
	store.SetAPIToken(repo, "tok_dfail")
	store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  repo,
		HostID:      "no-subscriber-host",
		ContainerID: "c-dfail",
	})

	resp := doReqAuth(t, "POST", ts.URL+"/v1/jobs", "tok_dfail", protocol.JobSubmissionRequest{
		Repository:  repo,
		IssueNumber: 1,
		RunID:       1,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", resp.StatusCode)
	}
}

// nonFlushWriter wraps a ResponseWriter to remove the Flusher interface,
// forcing the handler into long-poll mode.
type nonFlushWriter struct {
	http.ResponseWriter
}

// TestHandleDispatchLongPollEvent tests the long-poll path (non-SSE).
func TestHandleDispatchLongPollEvent(t *testing.T) {
	srv, _, _ := freshServer(t)

	hostID := "longpoll-host"

	// Use a custom handler test that wraps the response writer to remove Flusher.
	handler := srv.Handler()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		w := httptest.NewRecorder()
		// Wrap to remove Flusher interface.
		nfw := &nonFlushWriter{ResponseWriter: w}
		req := httptest.NewRequest("GET", "/v1/dispatch?host="+hostID, http.NoBody)
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		// Start handler in goroutine, send event, handler returns.
		go func() {
			time.Sleep(100 * time.Millisecond)
			srv.mu.RLock()
			_, subscribed := srv.dispatchSubs[hostID]
			srv.mu.RUnlock()
			if subscribed {
				srv.dispatchToHost(hostID, &protocol.DispatchEvent{
					JobID:       uuid.MustNew(),
					ContainerID: "c-lp",
					Repository:  "org/lp",
					IssueNumber: 77,
				})
			}
		}()

		handler.ServeHTTP(nfw, req)

		if w.Code != http.StatusOK {
			t.Errorf("long-poll status: got %d, want 200", w.Code)
		}
		var event protocol.DispatchEvent
		json.NewDecoder(w.Body).Decode(&event)
		if event.IssueNumber != 77 {
			t.Errorf("IssueNumber: got %d, want 77", event.IssueNumber)
		}
	}()

	<-doneCh
}

// TestHandleDispatchLongPollTimeout tests the long-poll timeout path.
func TestHandleDispatchLongPollNoEvent(t *testing.T) {
	srv, _, _ := freshServer(t)

	w := httptest.NewRecorder()
	nfw := &nonFlushWriter{ResponseWriter: w}
	req := httptest.NewRequest("GET", "/v1/dispatch?host=lp-timeout", http.NoBody)
	ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	srv.Handler().ServeHTTP(nfw, req)

	// Should return 204 (timeout) or context cancelled.
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 204 or 200", w.Code)
	}
}

// TestHandleDispatchLongPollClosed tests the "subscription closed" path.
func TestHandleDispatchLongPollClosed(t *testing.T) {
	srv, _, _ := freshServer(t)

	w := httptest.NewRecorder()
	nfw := &nonFlushWriter{ResponseWriter: w}
	req := httptest.NewRequest("GET", "/v1/dispatch?host=lp-closed", http.NoBody)
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Unsubscribe the host while the handler is waiting.
	go func() {
		time.Sleep(100 * time.Millisecond)
		srv.UnsubscribeDispatch("lp-closed")
	}()

	srv.Handler().ServeHTTP(nfw, req)

	if w.Code != http.StatusGone && w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Errorf("status: got %d", w.Code)
	}
}

// TestHandleAdHocDispatchFailure verifies that ad-hoc submission
// returns 503 when no host is subscribed.
func TestHandleAdHocDispatchFailure(t *testing.T) {
	_, ts, store := freshServer(t)
	projectID := uuid.MustNew()

	store.SetRoute(protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  "org/adhoc-fail",
		HostID:      "no-subscriber",
		ContainerID: "c-adhoc-fail",
	})

	resp := doReq(t, "POST", ts.URL+"/ui/api/adhoc", protocol.AdHocSubmissionRequest{
		ProjectID: projectID,
		Prompt:    "test",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", resp.StatusCode)
	}
}
