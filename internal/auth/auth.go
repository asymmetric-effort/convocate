package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/redis"
)

const (
	cookieName      = "convocate_session"
	stateCookieName = "convocate_oauth_state"
	defaultTTL      = 24 * time.Hour
)

// Config holds all configuration for the OAuth auth system.
type Config struct {
	RedisConn    redis.Doer
	ClientID     string
	ClientSecret string
	CallbackURL  string
	Org          string
	SessionTTL   time.Duration
}

// Handler returns an http.Handler that serves all /auth/* routes.
func Handler(cfg *Config, logger *log.Logger) http.Handler {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = defaultTTL
	}

	sessions := NewSessionStore(cfg.RedisConn, cfg.SessionTTL)
	gh := NewGitHubClient(cfg.ClientID, cfg.ClientSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", loginHandler(cfg))
	mux.HandleFunc("/auth/callback", callbackHandler(cfg, gh, sessions, logger))
	mux.HandleFunc("/auth/logout", logoutHandler(sessions))
	mux.HandleFunc("/auth/me", meHandler(sessions))
	return mux
}

// Sessions returns a SessionStore for the given config (used by middleware).
func Sessions(cfg *Config) *SessionStore {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = defaultTTL
	}
	return NewSessionStore(cfg.RedisConn, cfg.SessionTTL)
}

func loginHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		state := generateState()
		http.SetCookie(w, &http.Cookie{
			Name:     stateCookieName,
			Value:    state,
			Path:     "/auth/callback",
			MaxAge:   600, // 10 minutes
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		authURL := "https://github.com/login/oauth/authorize" +
			"?client_id=" + cfg.ClientID +
			"&redirect_uri=" + cfg.CallbackURL +
			"&scope=read:org,read:user" +
			"&state=" + state

		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func callbackHandler(cfg *Config, gh *GitHubClient, sessions *SessionStore, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Verify state.
		stateCookie, err := r.Cookie(stateCookieName)
		if err != nil || stateCookie.Value == "" {
			http.Error(w, "missing state cookie", http.StatusBadRequest)
			return
		}
		queryState := r.URL.Query().Get("state")
		if queryState == "" || queryState != stateCookie.Value {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			return
		}

		// Clear state cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     stateCookieName,
			Value:    "",
			Path:     "/auth/callback",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// Exchange code for token.
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			return
		}

		tokenResp, err := gh.ExchangeCode(code)
		if err != nil {
			logger.Printf("auth: exchange code failed: %v", err)
			http.Error(w, "authentication failed", http.StatusInternalServerError)
			return
		}

		// Get user info.
		user, err := gh.GetUser(tokenResp.AccessToken)
		if err != nil {
			logger.Printf("auth: get user failed: %v", err)
			http.Error(w, "failed to get user info", http.StatusInternalServerError)
			return
		}

		// Check org membership.
		isMember, err := gh.CheckOrgMembership(tokenResp.AccessToken, cfg.Org, user.Login)
		if err != nil {
			logger.Printf("auth: org check failed: %v", err)
			http.Error(w, "failed to verify org membership", http.StatusInternalServerError)
			return
		}
		if !isMember {
			http.Error(w, "access denied: not a member of "+cfg.Org, http.StatusForbidden)
			return
		}

		// Create session.
		session := &Session{
			GitHubUsername: user.Login,
			AvatarURL:      user.AvatarURL,
			OrgVerified:    true,
		}
		sessionID, err := sessions.Create(session)
		if err != nil {
			logger.Printf("auth: create session failed: %v", err)
			http.Error(w, "failed to create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   int(cfg.SessionTTL.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func logoutHandler(sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err == nil && cookie.Value != "" {
			_ = sessions.Delete(cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		http.Redirect(w, r, "/auth/login", http.StatusFound)
	}
}

func meHandler(sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		session, err := sessions.Get(cookie.Value)
		if err != nil || session == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"username": session.GitHubUsername,
			"avatar":   session.AvatarURL,
		})
	}
}

func generateState() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic("auth: read crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
