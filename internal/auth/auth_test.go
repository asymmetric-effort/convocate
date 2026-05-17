package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/redis"
)

func testConfig(redisConn redis.Doer) *Config {
	return &Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		CallbackURL:  "https://localhost:8443/auth/callback",
		Org:          "asymmetric-effort",
		SessionTTL:   time.Hour,
		RedisConn:    redisConn,
	}
}

func TestLoginRedirect(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "https://github.com/login/oauth/authorize") {
		t.Fatalf("unexpected redirect URL: %s", loc)
	}
	if !strings.Contains(loc, "client_id=test-client-id") {
		t.Fatalf("missing client_id in URL: %s", loc)
	}
	if !strings.Contains(loc, "scope=read:org,read:user") {
		t.Fatalf("missing scope in URL: %s", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Fatalf("missing state in URL: %s", loc)
	}

	// Verify state cookie is set.
	var stateCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == stateCookieName {
			stateCookie = c
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("state cookie not set")
	}
	if stateCookie.Value == "" {
		t.Fatal("state cookie is empty")
	}
}

func TestCallbackSuccess(t *testing.T) {
	// Mock GitHub API.
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubTokenResponse{
				AccessToken: "gho_test_token",
				TokenType:   "bearer",
				Scope:       "read:org,read:user",
			})
		case r.URL.Path == "/user":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubUser{
				Login:     "testuser",
				AvatarURL: "https://github.com/testuser.png",
			})
		case strings.HasPrefix(r.URL.Path, "/user/memberships/orgs/asymmetric-effort"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockGH.Close()

	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)

	sessions := NewSessionStore(conn, cfg.SessionTTL)
	gh := NewGitHubClient(cfg.ClientID, cfg.ClientSecret)
	gh.tokenURL = mockGH.URL + "/login/oauth/access_token"
	gh.apiBaseURL = mockGH.URL

	// Build handler manually to inject mock GitHub client.
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(cfg, gh, sessions, logger))

	// Create request with state cookie.
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=valid-state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/" {
		t.Fatalf("expected redirect to /, got %s", resp.Header.Get("Location"))
	}

	// Verify session cookie is set.
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}
	if sessionCookie.Value == "" {
		t.Fatal("session cookie is empty")
	}

	// Verify session exists in Redis.
	session, err := sessions.Get(sessionCookie.Value)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session == nil {
		t.Fatal("session not found")
	}
	if session.GitHubUsername != "testuser" {
		t.Fatalf("expected username testuser, got %s", session.GitHubUsername)
	}
	if !session.OrgVerified {
		t.Fatal("expected org_verified=true")
	}
}

func TestCallbackNonOrgMember(t *testing.T) {
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubTokenResponse{
				AccessToken: "gho_test_token",
				TokenType:   "bearer",
			})
		case r.URL.Path == "/user":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubUser{
				Login:     "outsider",
				AvatarURL: "https://github.com/outsider.png",
			})
		case strings.HasPrefix(r.URL.Path, "/user/memberships/orgs/asymmetric-effort"):
			w.WriteHeader(http.StatusNotFound) // Not a member.
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=valid-state", http.NoBody)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSessionCreateAndGet(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(conn, time.Hour)

	session := &Session{
		GitHubUsername: "testuser",
		AvatarURL:      "https://github.com/testuser.png",
		OrgVerified:    true,
	}

	id, err := store.Create(session)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if id == "" {
		t.Fatal("session ID is empty")
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got == nil {
		t.Fatal("session not found")
	}
	if got.GitHubUsername != "testuser" {
		t.Fatalf("expected testuser, got %s", got.GitHubUsername)
	}

	// Non-existent session.
	got, err = store.Get("nonexistent")
	if err != nil {
		t.Fatalf("get non-existent: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent session")
	}
}

func TestMiddlewareAuthenticated(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(conn, time.Hour)

	session := &Session{
		GitHubUsername: "testuser",
		AvatarURL:      "https://github.com/testuser.png",
		OrgVerified:    true,
	}
	sessionID, err := store.Create(session)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	handler := Middleware(store)(inner)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/projects", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMiddlewareUnauthenticated(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(conn, time.Hour)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Middleware(store)(inner)

	// HTML request -> redirect.
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/auth/login" {
		t.Fatalf("expected redirect to /auth/login, got %s", w.Header().Get("Location"))
	}

	// API request -> 401.
	req = httptest.NewRequest(http.MethodGet, "/ui/api/projects", http.NoBody)
	req.Header.Set("Accept", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMiddlewareSkipPaths(t *testing.T) {
	conn := redis.NewMockConn()
	store := NewSessionStore(conn, time.Hour)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	handler := Middleware(store)(inner)

	paths := []string{"/auth/login", "/auth/callback", "/v1/jobs", "/health", "/favicon.svg", "/assets/app.js"}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s: expected 200, got %d", p, w.Code)
		}
	}
}

func TestLogoutClearsSession(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	store := NewSessionStore(conn, cfg.SessionTTL)
	session := &Session{
		GitHubUsername: "testuser",
		AvatarURL:      "https://github.com/testuser.png",
		OrgVerified:    true,
	}
	sessionID, err := store.Create(session)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/auth/login" {
		t.Fatalf("expected redirect to /auth/login, got %s", resp.Header.Get("Location"))
	}

	// Verify session is deleted.
	got, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got != nil {
		t.Fatal("expected session to be deleted")
	}

	// Verify cookie is cleared.
	var cleared bool
	for _, c := range resp.Cookies() {
		if c.Name == cookieName && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatal("session cookie not cleared")
	}
}

func TestMeEndpoint(t *testing.T) {
	conn := redis.NewMockConn()
	cfg := testConfig(conn)
	logger := log.New(&strings.Builder{}, "", 0)
	handler := Handler(cfg, logger)

	store := NewSessionStore(conn, cfg.SessionTTL)
	session := &Session{
		GitHubUsername: "testuser",
		AvatarURL:      "https://github.com/testuser.png",
		OrgVerified:    true,
	}
	sessionID, err := store.Create(session)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Authenticated.
	req := httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["username"] != "testuser" {
		t.Fatalf("expected testuser, got %s", body["username"])
	}

	// Unauthenticated.
	req = httptest.NewRequest(http.MethodGet, "/auth/me", http.NoBody)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
