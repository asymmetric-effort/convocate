package ac

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type User struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Groups []string `json:"groups"`
}

type Group struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Builtin   bool     `json:"builtin"`
	UserCount int      `json:"userCount"`
	Roles     []string `json:"roles"`
}

type Role struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Applet      string `json:"applet"`
}

type GlobalSettings struct {
	RequireMFA           bool `json:"requireMfa"`
	SessionTimeoutMin    int  `json:"sessionTimeoutMinutes"`
	PasswordMinLength    int  `json:"passwordMinLength"`
	PasswordRotationDays int  `json:"passwordRotationDays"`
}

// Store wraps an OpenBao client for access control persistence.
// Roles remain static since they are Convocate RBAC concepts, not OpenBao entities.
type Store struct {
	addr   string
	token  string
	client *http.Client
	roles  []Role
}

// NewStore creates a Store backed by OpenBao. It reads OPENBAO_ADDR
// (default http://openbao.security.svc:8200) and OPENBAO_TOKEN from the environment.
func NewStore() *Store {
	addr := os.Getenv("OPENBAO_ADDR")
	if addr == "" {
		addr = "http://openbao.security.svc:8200"
	}
	addr = strings.TrimRight(addr, "/")
	return &Store{
		addr:   addr,
		token:  os.Getenv("OPENBAO_TOKEN"),
		client: &http.Client{},
		roles: []Role{
			{ID: "admin", Description: "Full access to all features", Applet: "all"},
			{ID: "node-view", Description: "View nodes", Applet: "nmgr"},
			{ID: "node-create", Description: "Provision nodes", Applet: "nmgr"},
			{ID: "node-update", Description: "Start/stop/edit nodes", Applet: "nmgr"},
			{ID: "node-delete", Description: "Decommission nodes", Applet: "nmgr"},
			{ID: "agent-view", Description: "View agents", Applet: "amgr"},
			{ID: "agent-update", Description: "Create/start/stop/configure agents", Applet: "amgr"},
			{ID: "pb-view", Description: "View project boards", Applet: "pb"},
			{ID: "pb-update", Description: "Edit boards, cards, containers, edges", Applet: "pb"},
			{ID: "pb-execute", Description: "Implement/send cards to agents", Applet: "pb"},
			{ID: "ide-view", Description: "View projects and files", Applet: "ide"},
			{ID: "ide-update", Description: "Edit files and create projects", Applet: "ide"},
			{ID: "repo-view", Description: "View repositories and PRs", Applet: "repo"},
			{ID: "repo-update", Description: "Create repositories", Applet: "repo"},
			{ID: "repo-merge", Description: "Merge pull requests", Applet: "repo"},
			{ID: "access-view", Description: "View users, groups, settings", Applet: "ac"},
			{ID: "access-update", Description: "Manage users, groups, settings", Applet: "ac"},
			{ID: "support-view", Description: "View and create tickets", Applet: "sup"},
		},
	}
}

// ── OpenBao HTTP helpers ────────────────────────────────────────────────

// baoRequest sends an authenticated request to OpenBao and returns the
// decoded JSON body. method is GET/PUT/POST/DELETE, path is the API path
// (e.g. "/v1/auth/userpass/users"). body may be nil for requests without
// a payload.
func (s *Store) baoRequest(method, path string, body any) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, s.addr+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Vault-Token", s.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openbao request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 204 No Content is success with no body.
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	var result map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("openbao %s %s returned %d: %s", method, path, resp.StatusCode, string(raw))
	}
	return result, nil
}

// baoList performs a LIST request (OpenBao uses the LIST HTTP method for
// enumeration endpoints).
func (s *Store) baoList(path string) ([]string, error) {
	req, err := http.NewRequest("LIST", s.addr+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create list request: %w", err)
	}
	req.Header.Set("X-Vault-Token", s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openbao LIST %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 404 means no keys exist yet — return empty list.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openbao LIST %s returned %d: %s", path, resp.StatusCode, string(raw))
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, nil
	}
	keysRaw, ok := data["keys"].([]any)
	if !ok {
		return nil, nil
	}
	keys := make([]string, 0, len(keysRaw))
	for _, k := range keysRaw {
		if ks, ok := k.(string); ok {
			keys = append(keys, ks)
		}
	}
	return keys, nil
}

// mapStr safely extracts a string from a nested map.
func mapStr(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			if s, ok := v.(string); ok {
				return s
			}
			return ""
		}
		if next, ok := v.(map[string]any); ok {
			cur = next
		} else {
			return ""
		}
	}
	return ""
}

// mapStrSlice extracts a string slice from a nested map.
func mapStrSlice(m map[string]any, keys ...string) []string {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return nil
		}
		if i == len(keys)-1 {
			if arr, ok := v.([]any); ok {
				out := make([]string, 0, len(arr))
				for _, item := range arr {
					if s, ok := item.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
			return nil
		}
		if next, ok := v.(map[string]any); ok {
			cur = next
		} else {
			return nil
		}
	}
	return nil
}

// ── Users (userpass auth + identity entities) ───────────────────────────

// ListUsers enumerates all userpass users and enriches each with identity
// entity metadata (email, display name, groups).
func (s *Store) ListUsers() ([]User, error) {
	names, err := s.baoList("/v1/auth/userpass/users")
	if err != nil {
		return nil, fmt.Errorf("list userpass users: %w", err)
	}

	users := make([]User, 0, len(names))
	for _, name := range names {
		u, err := s.getUser(name)
		if err != nil {
			return nil, fmt.Errorf("get user %s: %w", name, err)
		}
		users = append(users, u)
	}
	return users, nil
}

// getUser reads a single user from userpass + identity entity.
func (s *Store) getUser(username string) (User, error) {
	entity, err := s.baoRequest("GET", "/v1/identity/entity/name/"+username, nil)
	if err != nil {
		// Entity may not exist yet; return minimal info from username.
		return User{
			ID:     username,
			Email:  "",
			Name:   username,
			Status: "active",
			Groups: []string{},
		}, nil
	}

	data, _ := entity["data"].(map[string]any)
	if data == nil {
		return User{ID: username, Name: username, Status: "active", Groups: []string{}}, nil
	}

	id := mapStr(data, "id")
	if id == "" {
		id = username
	}
	email := mapStr(data, "metadata", "email")
	displayName := mapStr(data, "metadata", "name")
	if displayName == "" {
		displayName = username
	}

	status := "active"
	if d, ok := data["disabled"].(bool); ok && d {
		status = "disabled"
	}

	// Extract group IDs from the entity's group_ids field.
	groups := mapStrSlice(data, "group_ids")
	if groups == nil {
		groups = []string{}
	}

	return User{
		ID:     id,
		Email:  email,
		Name:   displayName,
		Status: status,
		Groups: groups,
	}, nil
}

// CreateUser creates a userpass login and a corresponding identity entity.
func (s *Store) CreateUser(u User) (User, error) {
	username := u.Email
	if username == "" {
		username = u.Name
	}

	// Create userpass login.
	_, err := s.baoRequest("PUT", "/v1/auth/userpass/users/"+username, map[string]any{
		"password": "changeme", // Default password; caller should set via separate password-reset flow.
	})
	if err != nil {
		return User{}, fmt.Errorf("create userpass user: %w", err)
	}

	// Create identity entity with metadata.
	entityResp, err := s.baoRequest("POST", "/v1/identity/entity", map[string]any{
		"name": username,
		"metadata": map[string]string{
			"email": u.Email,
			"name":  u.Name,
		},
	})
	if err != nil {
		return User{}, fmt.Errorf("create identity entity: %w", err)
	}

	id := username
	if entityResp != nil {
		if eid := mapStr(entityResp, "data", "id"); eid != "" {
			id = eid
		}
	}

	return User{
		ID:     id,
		Email:  u.Email,
		Name:   u.Name,
		Status: "active",
		Groups: []string{},
	}, nil
}

// UpdateUser modifies a user. For status changes: disabling deletes the
// userpass entry; enabling recreates it.
func (s *Store) UpdateUser(id string, u User) (User, bool, error) {
	// Look up the entity by ID to get the username.
	entity, err := s.baoRequest("GET", "/v1/identity/entity/id/"+id, nil)
	if err != nil {
		return User{}, false, nil
	}
	data, _ := entity["data"].(map[string]any)
	if data == nil {
		return User{}, false, nil
	}
	username := mapStr(data, "name")
	if username == "" {
		return User{}, false, nil
	}

	// Update metadata fields if provided.
	metadata := map[string]string{}
	existingEmail := mapStr(data, "metadata", "email")
	existingName := mapStr(data, "metadata", "name")

	if u.Email != "" {
		metadata["email"] = u.Email
	} else {
		metadata["email"] = existingEmail
	}
	if u.Name != "" {
		metadata["name"] = u.Name
	} else {
		metadata["name"] = existingName
	}

	_, err = s.baoRequest("POST", "/v1/identity/entity/id/"+id, map[string]any{
		"metadata": metadata,
	})
	if err != nil {
		return User{}, false, fmt.Errorf("update entity metadata: %w", err)
	}

	// Handle status changes.
	if u.Status == "disabled" {
		// Delete the userpass entry to prevent login.
		_, _ = s.baoRequest("DELETE", "/v1/auth/userpass/users/"+username, nil)
	} else if u.Status == "active" {
		// Recreate userpass entry (with a temporary password).
		_, _ = s.baoRequest("PUT", "/v1/auth/userpass/users/"+username, map[string]any{
			"password": "changeme",
		})
	}

	result, err := s.getUser(username)
	if err != nil {
		return User{}, false, fmt.Errorf("read back user: %w", err)
	}
	return result, true, nil
}

// DeleteUser removes both the userpass entry and the identity entity.
func (s *Store) DeleteUser(id string) (bool, error) {
	// Look up entity to get username.
	entity, err := s.baoRequest("GET", "/v1/identity/entity/id/"+id, nil)
	if err != nil {
		return false, nil
	}
	data, _ := entity["data"].(map[string]any)
	if data == nil {
		return false, nil
	}
	username := mapStr(data, "name")

	if username != "" {
		_, _ = s.baoRequest("DELETE", "/v1/auth/userpass/users/"+username, nil)
	}
	_, err = s.baoRequest("DELETE", "/v1/identity/entity/id/"+id, nil)
	if err != nil {
		return false, fmt.Errorf("delete entity: %w", err)
	}
	return true, nil
}

// ── Groups (identity groups) ────────────────────────────────────────────

// ListGroups enumerates all identity groups.
func (s *Store) ListGroups() ([]Group, error) {
	names, err := s.baoList("/v1/identity/group/name")
	if err != nil {
		return nil, fmt.Errorf("list identity groups: %w", err)
	}

	groups := make([]Group, 0, len(names))
	for _, name := range names {
		resp, err := s.baoRequest("GET", "/v1/identity/group/name/"+name, nil)
		if err != nil {
			continue
		}
		data, _ := resp["data"].(map[string]any)
		if data == nil {
			continue
		}

		gid := mapStr(data, "id")
		builtin := false
		if m, ok := data["metadata"].(map[string]any); ok {
			if b, ok := m["builtin"].(string); ok && b == "true" {
				builtin = true
			}
		}

		memberIDs := mapStrSlice(data, "member_entity_ids")
		policies := mapStrSlice(data, "policies")
		if policies == nil {
			policies = []string{}
		}

		groups = append(groups, Group{
			ID:        gid,
			Name:      name,
			Builtin:   builtin,
			UserCount: len(memberIDs),
			Roles:     policies,
		})
	}
	return groups, nil
}

// CreateGroup creates a new identity group.
func (s *Store) CreateGroup(name string) (Group, error) {
	resp, err := s.baoRequest("POST", "/v1/identity/group", map[string]any{
		"name":     name,
		"policies": []string{},
		"type":     "internal",
	})
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}

	gid := ""
	if resp != nil {
		gid = mapStr(resp, "data", "id")
	}

	return Group{
		ID:        gid,
		Name:      name,
		Builtin:   false,
		UserCount: 0,
		Roles:     []string{},
	}, nil
}

// DeleteGroup deletes an identity group by ID. Builtin groups cannot be deleted.
func (s *Store) DeleteGroup(id string) (bool, error) {
	// Read group first to check builtin flag.
	resp, err := s.baoRequest("GET", "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return false, nil
	}
	data, _ := resp["data"].(map[string]any)
	if data != nil {
		if m, ok := data["metadata"].(map[string]any); ok {
			if b, ok := m["builtin"].(string); ok && b == "true" {
				return false, nil
			}
		}
	}

	_, err = s.baoRequest("DELETE", "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return false, fmt.Errorf("delete group: %w", err)
	}
	return true, nil
}

// SetGroupUsers sets the member entity IDs for a group.
func (s *Store) SetGroupUsers(id string, entityIDs []string) (Group, bool, error) {
	_, err := s.baoRequest("POST", "/v1/identity/group/id/"+id, map[string]any{
		"member_entity_ids": entityIDs,
	})
	if err != nil {
		return Group{}, false, fmt.Errorf("set group members: %w", err)
	}

	// Read back group to return current state.
	resp, err := s.baoRequest("GET", "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return Group{}, false, fmt.Errorf("read back group: %w", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return Group{}, false, nil
	}

	name := mapStr(data, "name")
	members := mapStrSlice(data, "member_entity_ids")
	policies := mapStrSlice(data, "policies")
	if policies == nil {
		policies = []string{}
	}
	builtin := false
	if m, ok := data["metadata"].(map[string]any); ok {
		if b, ok := m["builtin"].(string); ok && b == "true" {
			builtin = true
		}
	}

	return Group{
		ID:        id,
		Name:      name,
		Builtin:   builtin,
		UserCount: len(members),
		Roles:     policies,
	}, true, nil
}

// SetGroupRoles sets the policies (mapped from role names) for a group.
func (s *Store) SetGroupRoles(id string, roles []string) (Group, bool, error) {
	_, err := s.baoRequest("POST", "/v1/identity/group/id/"+id, map[string]any{
		"policies": roles,
	})
	if err != nil {
		return Group{}, false, fmt.Errorf("set group roles: %w", err)
	}

	resp, err := s.baoRequest("GET", "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return Group{}, false, fmt.Errorf("read back group: %w", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return Group{}, false, nil
	}

	name := mapStr(data, "name")
	members := mapStrSlice(data, "member_entity_ids")
	policies := mapStrSlice(data, "policies")
	if policies == nil {
		policies = []string{}
	}
	builtin := false
	if m, ok := data["metadata"].(map[string]any); ok {
		if b, ok := m["builtin"].(string); ok && b == "true" {
			builtin = true
		}
	}

	return Group{
		ID:        id,
		Name:      name,
		Builtin:   builtin,
		UserCount: len(members),
		Roles:     policies,
	}, true, nil
}

// ── Roles (static) ─────────────────────────────────────────────────────

func (s *Store) ListRoles() []Role {
	out := make([]Role, len(s.roles))
	copy(out, s.roles)
	return out
}

// ── Settings (OpenBao KV v2) ────────────────────────────────────────────

func (s *Store) GetSettings() (GlobalSettings, error) {
	resp, err := s.baoRequest("GET", "/v1/convocate/data/settings", nil)
	if err != nil {
		// Return defaults if KV path does not exist yet.
		return GlobalSettings{
			RequireMFA:           false,
			SessionTimeoutMin:    30,
			PasswordMinLength:    12,
			PasswordRotationDays: 90,
		}, nil
	}

	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return GlobalSettings{RequireMFA: false, SessionTimeoutMin: 30, PasswordMinLength: 12, PasswordRotationDays: 90}, nil
	}

	// KV v2 nests actual data under data.data.
	inner, _ := data["data"].(map[string]any)
	if inner == nil {
		return GlobalSettings{RequireMFA: false, SessionTimeoutMin: 30, PasswordMinLength: 12, PasswordRotationDays: 90}, nil
	}

	gs := GlobalSettings{
		RequireMFA:           false,
		SessionTimeoutMin:    30,
		PasswordMinLength:    12,
		PasswordRotationDays: 90,
	}
	if v, ok := inner["requireMfa"].(bool); ok {
		gs.RequireMFA = v
	}
	if v, ok := inner["sessionTimeoutMinutes"].(float64); ok {
		gs.SessionTimeoutMin = int(v)
	}
	if v, ok := inner["passwordMinLength"].(float64); ok {
		gs.PasswordMinLength = int(v)
	}
	if v, ok := inner["passwordRotationDays"].(float64); ok {
		gs.PasswordRotationDays = int(v)
	}

	return gs, nil
}

func (s *Store) SetSettings(gs GlobalSettings) (GlobalSettings, error) {
	_, err := s.baoRequest("PUT", "/v1/convocate/data/settings", map[string]any{
		"data": map[string]any{
			"requireMfa":           gs.RequireMFA,
			"sessionTimeoutMinutes": gs.SessionTimeoutMin,
			"passwordMinLength":    gs.PasswordMinLength,
			"passwordRotationDays": gs.PasswordRotationDays,
		},
	})
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("write settings to openbao: %w", err)
	}
	return gs, nil
}
