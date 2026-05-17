package router

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// Server is the Router API HTTP server.
type Server struct {
	store        *redis.RouterStore
	bao          *openbao.Client
	authHandler  http.Handler
	authMW       func(http.Handler) http.Handler
	dispatchSubs map[string]chan protocol.DispatchEvent
	logger       *log.Logger
	version      string
	mu           sync.RWMutex
}

// Config holds the Router API server configuration.
type Config struct {
	Store       *redis.RouterStore
	Bao         *openbao.Client
	Logger      *log.Logger
	AuthHandler http.Handler
	AuthMW      func(http.Handler) http.Handler
	Version     string
}

// NewServer creates a new Router API server.
func NewServer(config Config) *Server {
	logger := config.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		store:        config.Store,
		bao:          config.Bao,
		authHandler:  config.AuthHandler,
		authMW:       config.AuthMW,
		version:      config.Version,
		dispatchSubs: make(map[string]chan protocol.DispatchEvent),
		logger:       logger,
	}
}

// Handler returns the http.Handler for the Router API.
// Routes are path-multiplexed:
//   - /v1/jobs          — job submission (GitHub Action, bearer token)
//   - /v1/dispatch      — dispatch events (Dispatch Service, mTLS)
//   - /v1/status        — job status transitions (Dispatch Service, mTLS)
//   - /v1/heartbeat     — host health (Dispatch Service, mTLS)
//   - /v1/health        — health check
//   - /ui/api/...       — Web UI management endpoints
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public API (GitHub Actions, bearer token auth).
	mux.HandleFunc("/v1/jobs", s.handleJobs)

	// Dispatch Service API (mTLS).
	mux.HandleFunc("/v1/dispatch", s.handleDispatch)
	mux.HandleFunc("/v1/status", s.handleStatus)
	mux.HandleFunc("/v1/heartbeat", s.handleHeartbeat)

	// Health (canonical + convenience alias).
	mux.HandleFunc("/v1/health", s.handleHealth)
	mux.HandleFunc("/health", s.handleHealth)

	// Auth routes.
	if s.authHandler != nil {
		mux.Handle("/auth/", s.authHandler)
	}

	// Root handler — router-api does not serve Web UI (that's convocate-ui).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			writeJSON(w, http.StatusOK, map[string]string{"service": "convocate-router-api"})
			return
		}
		http.NotFound(w, r)
	})

	// Web UI management API.
	mux.HandleFunc("/ui/api/projects", s.handleProjects)
	mux.HandleFunc("/ui/api/projects/create", s.handleCreateProject)
	mux.HandleFunc("/ui/api/projects/delete", s.handleDeleteProject)
	mux.HandleFunc("/ui/api/projects/upgrade", s.handleUpgradeContainer)
	mux.HandleFunc("/ui/api/projects/upgrade-all-idle", s.handleUpgradeAllIdle)
	mux.HandleFunc("/ui/api/auth", s.handleClusterAuth)
	mux.HandleFunc("/ui/api/adhoc", s.handleAdHocSubmit)
	mux.HandleFunc("/ui/api/jobs", s.handleJobsList)
	mux.HandleFunc("/ui/api/hosts", s.handleHostsList)

	// Wrap with auth middleware if configured.
	var handler http.Handler = mux
	if s.authMW != nil {
		handler = s.authMW(mux)
	}
	return middleware.SecurityHeaders(handler)
}

// ListenAndServe starts the Router API on the given listeners.
// publicListener serves /v1/jobs (GitHub Actions, bearer token).
// internalListener serves everything else (mTLS + Web UI).
func (s *Server) ListenAndServe(publicListener, internalListener net.Listener, publicTLS, internalTLS *tls.Config) error {
	publicServer := &http.Server{
		Handler:           s.publicHandler(),
		TLSConfig:         publicTLS,
		ReadHeaderTimeout: 30 * time.Second,
	}
	internalServer := &http.Server{
		Handler:           s.Handler(),
		TLSConfig:         internalTLS,
		ReadHeaderTimeout: 30 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		errCh <- publicServer.ServeTLS(publicListener, "", "")
	}()
	go func() {
		errCh <- internalServer.ServeTLS(internalListener, "", "")
	}()

	return <-errCh
}

// publicHandler returns a handler that only serves /v1/jobs and /v1/health.
func (s *Server) publicHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/jobs", s.handleJobs)
	mux.HandleFunc("/v1/health", s.handleHealth)
	return middleware.SecurityHeaders(mux)
}

// --- Dispatch subscription management ---

// SubscribeDispatch registers a host for dispatch events. Returns a channel
// that receives events targeted at this host.
func (s *Server) SubscribeDispatch(hostID string) chan protocol.DispatchEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan protocol.DispatchEvent, 16)
	s.dispatchSubs[hostID] = ch
	return ch
}

// UnsubscribeDispatch removes a host's dispatch subscription.
func (s *Server) UnsubscribeDispatch(hostID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.dispatchSubs[hostID]; ok {
		close(ch)
		delete(s.dispatchSubs, hostID)
	}
}

// dispatchToHost sends a dispatch event to the subscribed host.
func (s *Server) dispatchToHost(hostID string, event *protocol.DispatchEvent) error {
	s.mu.RLock()
	ch, ok := s.dispatchSubs[hostID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("router: host %q not subscribed for dispatch", hostID)
	}
	select {
	case ch <- *event:
		return nil
	default:
		return fmt.Errorf("router: dispatch channel full for host %q", hostID)
	}
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// maxRequestBody is the maximum allowed request body size (1 MB).
const maxRequestBody = 1 << 20

func readJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBody)
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// extractBearerToken extracts a bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// generateAPIToken creates a random API token for project dispatch auth.
func generateAPIToken() string {
	return "cvt_" + uuid.New().String()
}
