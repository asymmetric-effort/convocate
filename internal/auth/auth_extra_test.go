package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/redis"
)

// errorConn is a redis.Doer that always returns an error.
type errorConn struct {
	err error
}

func (e *errorConn) Do(_ ...string) (interface{}, error) {
	return nil, e.err
}

func (e *errorConn) Close() error { return nil }

// badTypeConn returns a non-string value for GET, simulating an unexpected
// data type in Redis.
type badTypeConn struct {
	redis.Doer
}

func (b *badTypeConn) Do(args ...string) (interface{}, error) {
	if len(args) > 0 && strings.EqualFold(args[0], "GET") {
		return int64(42), nil // unexpected type
	}
	return b.Doer.Do(args...)
}

// badJSONConn stores invalid JSON for the session key so that Get returns a
// JSON parse error.
type badJSONConn struct {
	redis.Doer
}

func (b *badJSONConn) Do(args ...string) (interface{}, error) {
	if len(args) > 0 && strings.EqualFold(args[0], "GET") {
		return "not-valid-json{{{{", nil
	}
	return b.Doer.Do(args...)
}

// --- Handler with zero TTL ---

func TestHandlerDefaultTTL(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "csec",
		CallbackURL:  "https://localhost/cb",
		Org:          "myorg",
		SessionTTL:   0, // triggers the defaultTTL branch inside Handler
		RedisConn:    conn,
	}
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)
	if handler == nil {
		t.Fatal("Handler returned nil")
	}
	// Verify TTL was defaulted.
	if cfg.SessionTTL != defaultTTL {
		t.Fatalf("expected TTL=%v, got %v", defaultTTL, cfg.SessionTTL)
	}
}

// --- Sessions() ---

func TestSessions(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "csec",
		CallbackURL:  "https://localhost/cb",
		Org:          "myorg",
		SessionTTL:   0, // should default to defaultTTL
		RedisConn:    conn,
	}
	store := Sessions(cfg)
	if store == nil {
		t.Fatal("Sessions returned nil")
	}
	// Verify we can round-trip a session through the returned store.
	id, err := store.Create(&Session{GitHubUsername: "u"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.GitHubUsername != "u" {
		t.Fatalf("unexpected session: %+v", got)
	}
}

// --- loginHandler ---

func TestLoginHandlerMethodNotAllowed(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// --- callbackHandler ---

func TestCallbackHandlerMethodNotAllowed(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/auth/callback", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCallbackHandlerMissingStateCookie(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	// No state cookie.
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=xyz&state=abc", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallbackHandlerInvalidState(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	// State cookie does not match query param.
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=xyz&state=wrong", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "correct-state"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallbackHandlerMissingCode(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	// State matches but no code.
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=mystate", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "mystate"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallbackHandlerCodeExchangeFailure(t *testing.T) {
	// Use a server that immediately closes so the HTTP client gets a network error.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // close immediately — any connection attempt will fail

	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)

	sessions := NewSessionStore(conn, cfg.SessionTTL)
	gh := NewGitHubClient(cfg.ClientID, cfg.ClientSecret)
	gh.tokenURL = "http://" + addr + "/login/oauth/access_token"

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(cfg, gh, sessions, logger))

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=xyz&state=s", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "s"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCallbackHandlerUserFetchFailure(t *testing.T) {
	// Token endpoint succeeds; /user endpoint fails with 500.
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/oauth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubTokenResponse{
				AccessToken: "tok",
				TokenType:   "bearer",
			})
			return
		}
		if r.URL.Path == "/user" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockGH.Close()

	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)

	sessions := NewSessionStore(conn, cfg.SessionTTL)
	gh := NewGitHubClient(cfg.ClientID, cfg.ClientSecret)
	gh.tokenURL = mockGH.URL + "/login/oauth/access_token"
	gh.apiBaseURL = mockGH.URL

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(cfg, gh, sessions, logger))

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=c&state=s", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "s"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from user fetch failure, got %d", w.Code)
	}
}

func TestCallbackHandlerOrgCheckFailure(t *testing.T) {
	// org membership endpoint returns a network-error by closing immediately.
	orgListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// Keep this open; the real mock server below will be apiBaseURL for /user,
	// and we'll point the gh.apiBaseURL at a broken address only for /orgs/.
	//
	// Simpler approach: use a single mock that returns an error status for /orgs/
	// that causes the http client to fail. We can do this by returning 500 from
	// /orgs/ endpoint and rely on our CheckOrgMembership returning non-204.
	// But CheckOrgMembership doesn't error on non-204 — it returns false.
	// To get an actual error we need a network failure. Use a closed listener.
	orgListener.Close()
	brokenAddr := orgListener.Addr().String()

	// A mock that handles /user fine but is not used for /orgs/ (that hits brokenAddr).
	userServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/oauth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubTokenResponse{
				AccessToken: "tok",
				TokenType:   "bearer",
			})
			return
		}
		if r.URL.Path == "/user" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubUser{Login: "u", AvatarURL: "a"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer userServer.Close()

	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)

	sessions := NewSessionStore(conn, cfg.SessionTTL)
	gh := NewGitHubClient(cfg.ClientID, cfg.ClientSecret)
	gh.tokenURL = userServer.URL + "/login/oauth/access_token"
	// Point apiBaseURL at the broken address so /orgs/ calls fail.
	gh.apiBaseURL = "http://" + brokenAddr

	// But /user is at apiBaseURL too, so we need a split. Use a custom
	// httpClient with a transport that routes /user to userServer and /orgs/ to broken.
	gh.apiBaseURL = userServer.URL
	// Override only the org check by using a custom transport.
	gh.httpClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: &splitTransport{
			userServer: userServer.URL,
			broken:     "http://" + brokenAddr,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(cfg, gh, sessions, logger))

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=c&state=s", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "s"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from org check failure, got %d", w.Code)
	}
}

// splitTransport routes /orgs/ requests to a broken address and everything
// else to the real test server.
type splitTransport struct {
	userServer string
	broken     string
}

func (st *splitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/memberships/orgs/") {
		broken := *req.URL
		broken.Host = strings.TrimPrefix(st.broken, "http://")
		broken.Scheme = "http"
		req2 := req.Clone(req.Context())
		req2.URL = &broken
		return http.DefaultTransport.RoundTrip(req2)
	}
	// Everything else goes to the real server.
	dest := *req.URL
	dest.Host = strings.TrimPrefix(st.userServer, "http://")
	dest.Scheme = "http"
	req2 := req.Clone(req.Context())
	req2.URL = &dest
	return http.DefaultTransport.RoundTrip(req2)
}

func TestCallbackHandlerSessionCreateFailure(t *testing.T) {
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubTokenResponse{
				AccessToken: "tok",
				TokenType:   "bearer",
			})
		case r.URL.Path == "/user":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubUser{Login: "u", AvatarURL: "a"})
		case strings.HasPrefix(r.URL.Path, "/user/memberships/orgs/"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockGH.Close()

	// Use an error conn so session.Create fails.
	errConn := &errorConn{err: fmt.Errorf("redis: unavailable")}
	cfg := testConfig(errConn)
	logger := log.New(&strings.Builder{}, "", 0)

	sessions := NewSessionStore(errConn, cfg.SessionTTL)
	gh := NewGitHubClient(cfg.ClientID, cfg.ClientSecret)
	gh.tokenURL = mockGH.URL + "/login/oauth/access_token"
	gh.apiBaseURL = mockGH.URL

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(cfg, gh, sessions, logger))

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=c&state=s", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "s"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from session create failure, got %d", w.Code)
	}
}

// --- logoutHandler ---

func TestLogoutHandlerMethodNotAllowed(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	req := httptest.NewRequest(http.MethodGet, "/auth/logout", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestLogoutHandlerNoCookie(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	// No session cookie — should still redirect to /auth/login.
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/auth/login" {
		t.Fatalf("expected /auth/login, got %s", resp.Header.Get("Location"))
	}
}

// --- meHandler ---

func TestMeHandlerMethodNotAllowed(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/auth/me", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestMeHandlerInvalidSession(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	// Cookie present but session does not exist in Redis.
	req := httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "nonexistent-session-id"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- ExchangeCode ---

func TestExchangeCodeInvalidURL(t *testing.T) {
	gh := NewGitHubClient("cid", "csec")
	// A URL with a control character is invalid and causes NewRequestWithContext to fail.
	gh.tokenURL = "http://\x00invalid"

	_, err := gh.ExchangeCode("code")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestExchangeCodeNetworkError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.tokenURL = "http://" + addr + "/token"

	_, err = gh.ExchangeCode("mycode")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestExchangeCodeBodyReadError(t *testing.T) {
	// Server declares a large Content-Length but closes immediately so io.ReadAll fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "9999")
		w.WriteHeader(http.StatusOK)
		// hijack and close to cause a read error
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.tokenURL = srv.URL + "/token"

	_, err := gh.ExchangeCode("mycode")
	if err == nil {
		t.Fatal("expected body read error, got nil")
	}
}

func TestExchangeCodeBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.tokenURL = srv.URL + "/token"

	_, err := gh.ExchangeCode("mycode")
	if err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
}

func TestExchangeCodeOAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GitHubTokenResponse{
			Error:     "bad_verification_code",
			ErrorDesc: "The code passed is incorrect or expired.",
		})
	}))
	defer srv.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.tokenURL = srv.URL + "/token"

	_, err := gh.ExchangeCode("badcode")
	if err == nil {
		t.Fatal("expected OAuth error, got nil")
	}
	if !strings.Contains(err.Error(), "bad_verification_code") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GetUser ---

func TestGetUserInvalidURL(t *testing.T) {
	gh := NewGitHubClient("cid", "csec")
	gh.apiBaseURL = "http://\x00invalid"

	_, err := gh.GetUser("tok")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestGetUserNetworkError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.apiBaseURL = "http://" + addr

	_, err = gh.GetUser("tok")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestGetUserBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json{{"))
	}))
	defer srv.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.apiBaseURL = srv.URL

	_, err := gh.GetUser("tok")
	if err == nil {
		t.Fatal("expected JSON decode error, got nil")
	}
}

// --- CheckOrgMembership ---

func TestCheckOrgMembershipInvalidURL(t *testing.T) {
	gh := NewGitHubClient("cid", "csec")
	gh.apiBaseURL = "http://\x00invalid"

	_, err := gh.CheckOrgMembership("tok", "org", "user")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestCheckOrgMembershipNetworkError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	gh := NewGitHubClient("cid", "csec")
	gh.apiBaseURL = "http://" + addr

	_, err = gh.CheckOrgMembership("tok", "myorg", "user")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

// --- session.Create / Get / Delete with Redis errors ---

func TestSessionCreateRedisError(t *testing.T) {
	errConn := &errorConn{err: fmt.Errorf("redis: timeout")}
	store := NewSessionStore(errConn, time.Hour)

	_, err := store.Create(&Session{GitHubUsername: "u"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSessionGetRedisError(t *testing.T) {
	errConn := &errorConn{err: fmt.Errorf("redis: timeout")}
	store := NewSessionStore(errConn, time.Hour)

	_, err := store.Get("some-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSessionGetUnexpectedType(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(&badTypeConn{Doer: conn}, time.Hour)

	// First create a real session via the underlying conn to give the key a value.
	// (The badTypeConn intercepts GET and returns int64, so Get should fail.)
	_, err := store.Get("any-id")
	if err == nil {
		t.Fatal("expected type error, got nil")
	}
}

func TestSessionGetBadJSON(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(&badJSONConn{Doer: conn}, time.Hour)

	// Pre-seed valid session in the underlying conn so the key exists.
	_, _ = conn.Do("SET", sessionKeyPrefix+"someid", `{"github_username":"u"}`, "EX", "3600")

	// badJSONConn overrides GET to return invalid JSON.
	_, err := store.Get("someid")
	if err == nil {
		t.Fatal("expected JSON unmarshal error, got nil")
	}
}

func TestSessionDeleteRedisError(t *testing.T) {
	errConn := &errorConn{err: fmt.Errorf("redis: timeout")}
	store := NewSessionStore(errConn, time.Hour)

	err := store.Delete("some-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Middleware — paths that bypass auth ---

func TestMiddlewareBypassPaths(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(conn, time.Hour)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(store)(inner)

	bypassPaths := []string{
		"/auth/login",
		"/auth/callback",
		"/auth/logout",
		"/v1/jobs",
		"/v1/anything",
		"/health",
		"/favicon.svg",
		"/assets/app.js",
		"/assets/style.css",
	}

	for _, p := range bypassPaths {
		req := httptest.NewRequest(http.MethodGet, p, http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s: expected 200 (bypass), got %d", p, w.Code)
		}
	}
}

func TestMiddlewareRedisError(t *testing.T) {
	// When Redis is down, session.Get returns an error — user should be
	// redirected to login (treated as unauthenticated).
	errConn := &errorConn{err: fmt.Errorf("redis: unavailable")}
	store := NewSessionStore(errConn, time.Hour)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(store)(inner)

	req := httptest.NewRequest(http.MethodGet, "/ui/dashboard", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "some-session-id"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on Redis error, got %d", w.Code)
	}
}
