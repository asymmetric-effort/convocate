// HTTPS server for the agent wrapper.
// Exposes health, metrics, stdin/stdout/stderr I/O, and control endpoints.

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// Server holds the HTTP server state and references to the process and metrics.
type Server struct {
	process        *Process
	metrics        *Metrics
	auth           *Auth
	wrapperVersion string
	claudeVersion  string
	podName        string
	nodeName       string
}

// NewServer creates a new Server instance.
func NewServer(proc *Process, metrics *Metrics, auth *Auth, wrapperVersion, claudeVersion, podName, nodeName string) *Server {
	return &Server{
		process:        proc,
		metrics:        metrics,
		auth:           auth,
		wrapperVersion: wrapperVersion,
		claudeVersion:  claudeVersion,
		podName:        podName,
		nodeName:       nodeName,
	}
}

// RegisterRoutes adds all handler routes to the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Health probes — no auth (K8s probes)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Metrics — requires agent-view
	mux.HandleFunc("GET /metrics", s.auth.RequireRole("agent-view", s.handleMetrics))

	// I/O — stdin requires agent-update, stdout/stderr require agent-view
	mux.HandleFunc("POST /stdin", s.auth.RequireRole("agent-update", s.handleStdin))
	mux.HandleFunc("GET /stdout", s.auth.RequireRole("agent-view", s.handleStdout))
	mux.HandleFunc("GET /stderr", s.auth.RequireRole("agent-view", s.handleStderr))

	// Control — requires agent-update
	mux.HandleFunc("POST /control/restart", s.auth.RequireRole("agent-update", s.handleRestart))
	mux.HandleFunc("POST /control/signal", s.auth.RequireRole("agent-update", s.handleSignal))
}

// handleHealthz returns 200 if the wrapper is running.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReadyz returns 200 if Claude CLI is running and ready for input.
func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.process.IsRunning() {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
	}
}

// handleMetrics returns usage and performance statistics.
func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	data, err := s.metrics.SnapshotJSON(
		s.wrapperVersion, s.claudeVersion,
		s.podName, s.nodeName,
		s.process.Uptime(),
	)
	if err != nil {
		http.Error(w, `{"error":"metrics serialization failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleStdin writes raw bytes to the Claude CLI's stdin.
func (s *Server) handleStdin(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}
	if err := s.process.WriteStdin(body); err != nil {
		http.Error(w, `{"error":"stdin write failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleStdout streams Claude CLI stdout via WebSocket.
func (s *Server) handleStdout(w http.ResponseWriter, r *http.Request) {
	s.streamOutput(w, r, true)
}

// handleStderr streams Claude CLI stderr via WebSocket.
func (s *Server) handleStderr(w http.ResponseWriter, r *http.Request) {
	s.streamOutput(w, r, false)
}

// streamOutput upgrades to WebSocket and streams process output.
func (s *Server) streamOutput(w http.ResponseWriter, r *http.Request, isStdout bool) {
	conn, err := upgradeWS(w, r)
	if err != nil {
		return
	}
	defer conn.Close()

	var ch chan []byte
	var unsub func()
	if isStdout {
		ch, unsub = s.process.SubscribeStdout()
	} else {
		ch, unsub = s.process.SubscribeStderr()
	}
	defer unsub()

	// Read and discard incoming client frames in background
	go wsReadDiscard(conn)

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			if err := wsWriteFrame(conn, data); err != nil {
				return
			}
		case <-time.After(30 * time.Second):
			if err := wsWritePing(conn); err != nil {
				return
			}
		}
	}
}

// handleRestart gracefully restarts the Claude CLI process.
func (s *Server) handleRestart(w http.ResponseWriter, _ *http.Request) {
	if err := s.process.Restart(s.process.flags); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSignal sends an OS signal to the Claude CLI process.
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Signal string `json:"signal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	sig, ok := parseSignal(req.Signal)
	if !ok {
		http.Error(w, `{"error":"unknown signal"}`, http.StatusBadRequest)
		return
	}

	if err := s.process.Signal(sig); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseSignal maps a signal name to a syscall.Signal.
func parseSignal(name string) (syscall.Signal, bool) {
	switch strings.ToUpper(name) {
	case "SIGTERM", "TERM":
		return syscall.SIGTERM, true
	case "SIGINT", "INT":
		return syscall.SIGINT, true
	case "SIGKILL", "KILL":
		return syscall.SIGKILL, true
	case "SIGUSR1", "USR1":
		return syscall.SIGUSR1, true
	case "SIGUSR2", "USR2":
		return syscall.SIGUSR2, true
	case "SIGHUP", "HUP":
		return syscall.SIGHUP, true
	default:
		return 0, false
	}
}

// ---------------------------------------------------------------------------
// WebSocket helpers — raw stdlib implementation (no third-party deps)
// Copied from the Convocate API events handler pattern.
// ---------------------------------------------------------------------------

const wsGUID = "258EAFA5-E914-47DA-95CA-5AB5DC085B11"

func upgradeWS(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "websocket upgrade required", http.StatusUpgradeRequired)
		return nil, fmt.Errorf("not a websocket request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return nil, fmt.Errorf("missing key")
	}

	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("no hijacker")
	}

	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	buf.WriteString(resp)
	buf.Flush()

	return conn, nil
}

func wsWriteFrame(conn net.Conn, data []byte) error {
	frame := make([]byte, 0, 2+8+len(data))
	frame = append(frame, 0x81) // text frame, FIN
	if len(data) < 126 {
		frame = append(frame, byte(len(data)))
	} else if len(data) < 65536 {
		frame = append(frame, 126)
		frame = append(frame, byte(len(data)>>8), byte(len(data)))
	} else {
		frame = append(frame, 127)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(len(data)))
		frame = append(frame, b...)
	}
	frame = append(frame, data...)
	_, err := conn.Write(frame)
	return err
}

func wsWritePing(conn net.Conn) error {
	_, err := conn.Write([]byte{0x89, 0x00})
	return err
}

func wsReadDiscard(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		_, err := reader.ReadByte()
		if err != nil {
			return
		}
	}
}
