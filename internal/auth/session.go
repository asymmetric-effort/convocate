package auth

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

const sessionKeyPrefix = "auth:session:"

// Session holds the authenticated user's session data.
type Session struct {
	GitHubUsername string `json:"github_username"`
	AvatarURL      string `json:"avatar_url"`
	OrgVerified    bool   `json:"org_verified"`
	CreatedAt      int64  `json:"created_at"`
}

// SessionStore manages sessions in Redis.
type SessionStore struct {
	conn redis.Doer
	ttl  time.Duration
}

// NewSessionStore creates a new session store backed by the given Redis connection.
func NewSessionStore(conn redis.Doer, ttl time.Duration) *SessionStore {
	return &SessionStore{conn: conn, ttl: ttl}
}

// Create generates a new session ID and persists the session in Redis.
func (s *SessionStore) Create(session *Session) (string, error) {
	id := uuid.New().String()
	session.CreatedAt = time.Now().Unix()

	data, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("auth: marshal session: %w", err)
	}

	key := sessionKeyPrefix + id
	ttlSec := strconv.Itoa(int(s.ttl.Seconds()))
	_, err = s.conn.Do("SET", key, string(data), "EX", ttlSec)
	if err != nil {
		return "", fmt.Errorf("auth: store session: %w", err)
	}

	return id, nil
}

// Get retrieves a session by ID. Returns nil if not found.
func (s *SessionStore) Get(id string) (*Session, error) {
	key := sessionKeyPrefix + id
	result, err := s.conn.Do("GET", key)
	if err != nil {
		return nil, fmt.Errorf("auth: get session: %w", err)
	}
	if result == nil {
		return nil, nil
	}

	data, ok := result.(string)
	if !ok {
		return nil, fmt.Errorf("auth: unexpected session data type")
	}

	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("auth: unmarshal session: %w", err)
	}

	return &session, nil
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(id string) error {
	key := sessionKeyPrefix + id
	_, err := s.conn.Do("DEL", key)
	if err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}
