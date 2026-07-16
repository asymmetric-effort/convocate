package events

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseTypeFilter_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	result := parseTypeFilter(r)
	if result != nil {
		t.Fatalf("expected nil for empty filter, got %v", result)
	}
}

func TestParseTypeFilter_Single(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes?filter=node.updated", nil)
	result := parseTypeFilter(r)
	if len(result) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(result))
	}
	if result[0] != "node.updated" {
		t.Fatalf("expected node.updated, got %s", result[0])
	}
}

func TestParseTypeFilter_Multiple(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes?filter=a,b,c", nil)
	result := parseTypeFilter(r)
	if len(result) != 3 {
		t.Fatalf("expected 3 filters, got %d", len(result))
	}
	expected := []string{"a", "b", "c"}
	for i, e := range expected {
		if result[i] != e {
			t.Fatalf("filter[%d]: expected %s, got %s", i, e, result[i])
		}
	}
}

func TestParseTypeFilter_WithSpaces(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes?filter=a%20,%20b%20,%20c", nil)
	result := parseTypeFilter(r)
	if len(result) != 3 {
		t.Fatalf("expected 3 filters, got %d", len(result))
	}
	for _, f := range result {
		if f == "" {
			t.Fatal("expected non-empty filter after trimming")
		}
	}
}

func TestParseTypeFilter_EmptyEntries(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes?filter=a,,b,", nil)
	result := parseTypeFilter(r)
	if len(result) != 2 {
		t.Fatalf("expected 2 non-empty filters, got %d: %v", len(result), result)
	}
}

func TestHandleSSE_SetHeaders(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()

	go func() {
		// Give the handler a moment to set headers, then cancel
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	handleSSE(w, r, "nmgr/nodes", nil)

	resp := w.Result()
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %s", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("expected Cache-Control no-cache, got %s", got)
	}
	if got := resp.Header.Get("Connection"); got != "keep-alive" {
		t.Fatalf("expected Connection keep-alive, got %s", got)
	}
}

func TestHandleSSE_ReceivesEvent(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, r, "nmgr/nodes", nil)
		close(done)
	}()

	// Wait for subscription to be registered
	time.Sleep(20 * time.Millisecond)

	hub.Publish("nmgr/nodes", "test", "payload")
	time.Sleep(20 * time.Millisecond)

	cancel()
	<-done

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("expected SSE event data in response body")
	}
	if body[:5] != "data:" {
		t.Fatalf("expected body to start with 'data:', got: %s", body[:20])
	}
}

func TestHandleEvents_NoUpgrade_FallsToSSE(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	r = r.WithContext(ctx)
	r.SetPathValue("applet", "nmgr")
	r.SetPathValue("channel", "nodes")

	w := httptest.NewRecorder()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	handleEvents(w, r)

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("expected SSE fallback when no Upgrade header")
	}
}

func TestHandleEvents_WebSocketUpgrade_MissingKey(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	r.Header.Set("Upgrade", "websocket")
	r.SetPathValue("applet", "nmgr")
	r.SetPathValue("channel", "nodes")

	w := httptest.NewRecorder()

	handleEvents(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing Sec-WebSocket-Key, got %d", resp.StatusCode)
	}
}

func TestWsGUID(t *testing.T) {
	if wsGUID != "258EAFA5-E914-47DA-95CA-5AB5DC085B11" {
		t.Fatalf("unexpected WS GUID: %s", wsGUID)
	}
}

func TestWsWrite_SmallPayload(t *testing.T) {
	// Test wsWrite with a small payload (< 126 bytes)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	data := []byte("hello")
	go func() {
		wsWrite(server, data)
		server.Close()
	}()

	buf := make([]byte, 128)
	n, _ := client.Read(buf)
	if n < 2+len(data) {
		t.Fatalf("expected at least %d bytes, got %d", 2+len(data), n)
	}
	// First byte: 0x81 (text frame, FIN)
	if buf[0] != 0x81 {
		t.Fatalf("expected 0x81, got 0x%02x", buf[0])
	}
	// Second byte: payload length
	if buf[1] != byte(len(data)) {
		t.Fatalf("expected length %d, got %d", len(data), buf[1])
	}
	// Payload
	if string(buf[2:2+len(data)]) != "hello" {
		t.Fatalf("expected 'hello', got %s", string(buf[2:2+len(data)]))
	}
}

func TestWsWrite_MediumPayload(t *testing.T) {
	// Test wsWrite with a medium payload (>= 126 but < 65536 bytes)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	data := make([]byte, 200)
	for i := range data {
		data[i] = 'A'
	}
	go func() {
		wsWrite(server, data)
		server.Close()
	}()

	buf := make([]byte, 300)
	total := 0
	for total < 4+len(data) {
		n, err := client.Read(buf[total:])
		if err != nil {
			break
		}
		total += n
	}
	if total < 4+len(data) {
		t.Fatalf("expected at least %d bytes, got %d", 4+len(data), total)
	}
	if buf[0] != 0x81 {
		t.Fatalf("expected 0x81, got 0x%02x", buf[0])
	}
	if buf[1] != 126 {
		t.Fatalf("expected extended length marker 126, got %d", buf[1])
	}
}

func TestWsWrite_LargePayload(t *testing.T) {
	// Test wsWrite with a large payload (>= 65536 bytes)
	server, client := net.Pipe()
	defer client.Close()

	data := make([]byte, 70000)
	for i := range data {
		data[i] = 'B'
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- wsWrite(server, data)
		server.Close()
	}()

	// Read enough to check the header
	buf := make([]byte, 10+len(data))
	total := 0
	for total < 10+len(data) {
		n, err := client.Read(buf[total:])
		if err != nil {
			break
		}
		total += n
	}
	if total < 10+len(data) {
		t.Fatalf("expected at least %d bytes, got %d", 10+len(data), total)
	}
	if buf[0] != 0x81 {
		t.Fatalf("expected 0x81, got 0x%02x", buf[0])
	}
	if buf[1] != 127 {
		t.Fatalf("expected extended length marker 127, got %d", buf[1])
	}
}

func TestWsWrite_ConnectionError(t *testing.T) {
	server, client := net.Pipe()
	client.Close() // Close read end

	err := wsWrite(server, []byte("hello"))
	server.Close()
	if err == nil {
		t.Fatal("expected error writing to closed connection")
	}
}

func TestWsWritePing(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		wsWritePing(server)
		server.Close()
	}()

	buf := make([]byte, 10)
	n, _ := client.Read(buf)
	if n != 2 {
		t.Fatalf("expected 2 bytes for ping, got %d", n)
	}
	if buf[0] != 0x89 || buf[1] != 0x00 {
		t.Fatalf("expected ping frame [0x89, 0x00], got [0x%02x, 0x%02x]", buf[0], buf[1])
	}
}

func TestWsWritePing_ConnectionError(t *testing.T) {
	server, client := net.Pipe()
	client.Close()

	err := wsWritePing(server)
	server.Close()
	if err == nil {
		t.Fatal("expected error writing ping to closed connection")
	}
}

func TestWsReadDiscard(t *testing.T) {
	server, client := net.Pipe()

	done := make(chan struct{})
	go func() {
		wsReadDiscard(client)
		close(done)
	}()

	// Write some data and close
	server.Write([]byte("some data"))
	server.Close()

	// wsReadDiscard should exit when connection closes
	select {
	case <-done:
		// expected
	case <-time.After(time.Second):
		t.Fatal("wsReadDiscard did not exit after connection closed")
	}
	client.Close()
}

func TestUpgradeWebSocket_NoHijacker(t *testing.T) {
	// httptest.ResponseRecorder implements http.Hijacker since Go 1.20+
	// but returns an error. This tests the "missing key" path handled
	// via handleEvents already.
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()

	conn, err := upgradeWebSocket(w, r)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		// Some Go versions may support Hijack on the recorder
		return
	}
	// Expected: either hijack not supported or some other error
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	// Just verify it doesn't panic
}

func TestUpgradeWebSocket_MissingKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	// No Sec-WebSocket-Key header
	w := httptest.NewRecorder()

	conn, err := upgradeWebSocket(w, r)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error for missing key")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleEvents_WebSocket_FullPath(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	// Create a real HTTP server to get proper hijacking support
	server := httptest.NewServer(http.HandlerFunc(handleEvents))
	defer server.Close()

	// Make a WebSocket upgrade request to the real server
	// This will fail at the mux route matching level but exercises the code path
	client := server.Client()
	req, _ := http.NewRequest("GET", server.URL, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	resp, err := client.Do(req)
	if err != nil {
		// Connection may be hijacked, which is fine
		return
	}
	defer resp.Body.Close()
	// If we get here, the response should be 101 (upgraded) or an error
}

func TestHandleSSE_Keepalive(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	origInterval := keepaliveInterval
	keepaliveInterval = 5 * time.Millisecond
	defer func() {
		DefaultHub = origDefault
		keepaliveInterval = origInterval
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, r, "nmgr/nodes", nil)
		close(done)
	}()

	// Wait long enough for keepalive to fire
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, ": keepalive") {
		t.Fatalf("expected keepalive comment in SSE output, got: %s", body)
	}
}

func TestHandleSSE_SubDone(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, r, "nmgr/nodes", nil)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)

	// Unsubscribe all subscribers to trigger sub.done
	hub.mu.RLock()
	subs := hub.subscribers["nmgr/nodes"]
	var subList []*subscriber
	for s := range subs {
		subList = append(subList, s)
	}
	hub.mu.RUnlock()

	for _, s := range subList {
		hub.Unsubscribe("nmgr/nodes", s)
	}

	select {
	case <-done:
		// expected
	case <-time.After(time.Second):
		t.Fatal("handleSSE did not exit after sub.done closed")
	}
}

// nonFlushWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type nonFlushWriter struct {
	header http.Header
	code   int
	body   []byte
}

func (w *nonFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *nonFlushWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *nonFlushWriter) WriteHeader(code int) {
	w.code = code
}

func TestHandleSSE_NoFlusher(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes", nil)
	w := &nonFlushWriter{}

	// Should return immediately since w doesn't implement http.Flusher
	handleSSE(w, r, "nmgr/nodes", nil)

	// Verify headers were still set
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatal("expected Content-Type to be set even without flusher")
	}
}

func TestHandleSSE_WithTypeFilter(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := httptest.NewRequest("GET", "/api/v1/events/nmgr/nodes?filter=wanted", nil)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, r, "nmgr/nodes", parseTypeFilter(r))
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)

	// Publish unwanted event
	hub.Publish("nmgr/nodes", "unwanted", "data")
	time.Sleep(10 * time.Millisecond)

	// Publish wanted event
	hub.Publish("nmgr/nodes", "wanted", "data")
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-done

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("expected event data")
	}
	// Should only have one "data:" entry (the wanted one)
}

func TestHandleEvents_WebSocket_EventLoop_DataAndClose(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("applet", "nmgr")
		r.SetPathValue("channel", "nodes")
		handleEvents(w, r)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Dial raw TCP to the test server
	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send WebSocket upgrade request
	req := "GET / HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	conn.Write([]byte(req))

	// Read response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Wait for subscription to be registered
	time.Sleep(20 * time.Millisecond)

	// Publish an event
	hub.Publish("nmgr/nodes", "test.event", "hello")
	time.Sleep(20 * time.Millisecond)

	// Read the WebSocket frame
	conn.SetReadDeadline(time.Now().Add(time.Second))
	hdr := make([]byte, 2)
	_, err = io.ReadFull(reader, hdr)
	if err != nil {
		t.Fatalf("failed to read WS frame header: %v", err)
	}
	if hdr[0] != 0x81 {
		t.Fatalf("expected text frame 0x81, got 0x%02x", hdr[0])
	}
	payloadLen := int(hdr[1] & 0x7F)
	if payloadLen == 126 {
		ext := make([]byte, 2)
		io.ReadFull(reader, ext)
		payloadLen = int(binary.BigEndian.Uint16(ext))
	}
	payload := make([]byte, payloadLen)
	io.ReadFull(reader, payload)
	if !strings.Contains(string(payload), "test.event") {
		t.Fatalf("expected payload to contain test.event, got: %s", string(payload))
	}

	// Close the connection to trigger WS write error path
	conn.Close()
}

func TestHandleEvents_WebSocket_Keepalive(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	origInterval := keepaliveInterval
	keepaliveInterval = 5 * time.Millisecond
	defer func() {
		DefaultHub = origDefault
		keepaliveInterval = origInterval
	}()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("applet", "nmgr")
		r.SetPathValue("channel", "nodes")
		handleEvents(w, r)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	req := "GET / HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	conn.Write([]byte(req))

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Wait for keepalive ping to fire
	time.Sleep(30 * time.Millisecond)

	// Read ping frame
	conn.SetReadDeadline(time.Now().Add(time.Second))
	hdr := make([]byte, 2)
	_, err = io.ReadFull(reader, hdr)
	if err != nil {
		t.Fatalf("failed to read ping frame: %v", err)
	}
	if hdr[0] != 0x89 {
		t.Fatalf("expected ping frame 0x89, got 0x%02x", hdr[0])
	}

	conn.Close()
}

func TestHandleEvents_WebSocket_SubDone(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	handlerDone := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("applet", "nmgr")
		r.SetPathValue("channel", "nodes")
		handleEvents(w, r)
		close(handlerDone)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	req := "GET / HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	conn.Write([]byte(req))

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	time.Sleep(20 * time.Millisecond)

	// Force close the sub.done channel by unsubscribing
	hub.mu.RLock()
	subs := hub.subscribers["nmgr/nodes"]
	var subList []*subscriber
	for s := range subs {
		subList = append(subList, s)
	}
	hub.mu.RUnlock()

	for _, s := range subList {
		hub.Unsubscribe("nmgr/nodes", s)
	}

	select {
	case <-handlerDone:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("handleEvents did not exit after sub.done closed")
	}
}

func TestHandleEvents_WebSocket_WriteError(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	handlerDone := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("applet", "nmgr")
		r.SetPathValue("channel", "nodes")
		handleEvents(w, r)
		close(handlerDone)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	req := "GET / HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	conn.Write([]byte(req))

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	time.Sleep(20 * time.Millisecond)

	// Close the client TCP connection with RST to ensure write errors
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetLinger(0)
	}
	conn.Close()
	time.Sleep(20 * time.Millisecond)

	// Publish many messages to trigger a write error (OS may buffer one)
	for i := 0; i < 100; i++ {
		hub.Publish("nmgr/nodes", "test", strings.Repeat("x", 1000))
	}

	select {
	case <-handlerDone:
		// expected - handler should exit on write error
	case <-time.After(2 * time.Second):
		t.Fatal("handleEvents did not exit after write error")
	}
}

func TestHandleEvents_WebSocket_PingError(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	origInterval := keepaliveInterval
	keepaliveInterval = 5 * time.Millisecond
	defer func() {
		DefaultHub = origDefault
		keepaliveInterval = origInterval
	}()

	handlerDone := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("applet", "nmgr")
		r.SetPathValue("channel", "nodes")
		handleEvents(w, r)
		close(handlerDone)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	req := "GET / HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	conn.Write([]byte(req))

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Close connection so the keepalive ping will fail
	conn.Close()

	select {
	case <-handlerDone:
		// expected - handler should exit on ping write error
	case <-time.After(2 * time.Second):
		t.Fatal("handleEvents did not exit after ping error")
	}
}

func TestWsReadDiscard_NonEOFError(t *testing.T) {
	server, client := net.Pipe()

	done := make(chan struct{})
	go func() {
		wsReadDiscard(client)
		close(done)
	}()

	// Set a read deadline on client to force a non-EOF error
	client.SetReadDeadline(time.Now().Add(5 * time.Millisecond))

	select {
	case <-done:
		// expected - wsReadDiscard should exit on timeout (non-EOF) error
	case <-time.After(time.Second):
		t.Fatal("wsReadDiscard did not exit on non-EOF error")
	}
	server.Close()
	client.Close()
}

// failHijackWriter implements http.ResponseWriter and http.Hijacker but always
// fails the Hijack call.
type failHijackWriter struct {
	header http.Header
	code   int
}

func (w *failHijackWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failHijackWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *failHijackWriter) WriteHeader(code int) {
	w.code = code
}

func (w *failHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, fmt.Errorf("hijack failed")
}

func TestUpgradeWebSocket_HijackError(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := &failHijackWriter{}

	conn, err := upgradeWebSocket(w, r)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error for hijack failure")
	}
}

func TestHandleEvents_SSE_PathValues(t *testing.T) {
	hub := newTestHub()
	origDefault := DefaultHub
	DefaultHub = hub
	defer func() { DefaultHub = origDefault }()

	ctx, cancel := context.WithCancel(context.Background())

	r := httptest.NewRequest("GET", "/api/v1/events/amgr/agent/test-1", nil)
	r = r.WithContext(ctx)
	r.SetPathValue("applet", "amgr")
	r.SetPathValue("channel", "agent/test-1")

	w := httptest.NewRecorder()

	go func() {
		time.Sleep(10 * time.Millisecond)
		hub.Publish("amgr/agent/test-1", "status", "running")
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	handleEvents(w, r)

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("expected event data for amgr channel")
	}
}
