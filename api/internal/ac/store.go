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
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Password string   `json:"password,omitempty"`
	Status   string   `json:"status"`
	Groups   []string `json:"groups"`
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
	RequireMFA        bool `json:"requireMfa"`
	SessionTimeoutMin int  `json:"sessionTimeoutMinutes"`
	PasswordMinLength int  `json:"passwordMinLength"`
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

// CreateUser creates a userpass login and a corresponding identity entity
// with an entity alias linking the two.
func (s *Store) CreateUser(u User) (User, error) {
	username := u.Name
	if username == "" {
		username = u.Email
	}

	password := u.Password
	if password == "" {
		password = "changeme"
	}

	// Create userpass login with the caller-supplied password.
	_, err := s.baoRequest("PUT", "/v1/auth/userpass/users/"+username, map[string]any{
		"password": password,
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

	// Create entity alias linking the userpass user to the identity entity.
	// Look up the userpass auth mount accessor first.
	mountAccessor, err := s.getUserpassAccessor()
	if err == nil && mountAccessor != "" {
		_, _ = s.baoRequest("POST", "/v1/identity/entity-alias", map[string]any{
			"name":           username,
			"canonical_id":   id,
			"mount_accessor": mountAccessor,
		})
	}

	return User{
		ID:     id,
		Email:  u.Email,
		Name:   u.Name,
		Status: "active",
		Groups: []string{},
	}, nil
}

// getUserpassAccessor returns the mount accessor for the userpass auth method.
func (s *Store) getUserpassAccessor() (string, error) {
	resp, err := s.baoRequest("GET", "/v1/sys/auth", nil)
	if err != nil {
		return "", fmt.Errorf("list auth mounts: %w", err)
	}

	// The response has mount paths as keys (e.g. "userpass/").
	for key, val := range resp {
		if !strings.HasPrefix(key, "userpass") {
			continue
		}
		if m, ok := val.(map[string]any); ok {
			if accessor, ok := m["accessor"].(string); ok {
				return accessor, nil
			}
		}
	}
	return "", fmt.Errorf("userpass auth mount not found")
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

	// Handle group membership changes if groups were provided.
	if u.Groups != nil {
		desiredGroups := make(map[string]bool, len(u.Groups))
		for _, gid := range u.Groups {
			desiredGroups[gid] = true
		}

		// List all groups to update membership.
		allGroups, err := s.ListGroups()
		if err != nil {
			return User{}, false, fmt.Errorf("list groups for membership update: %w", err)
		}

		for _, g := range allGroups {
			// Read the group to get current member_entity_ids.
			resp, err := s.baoRequest("GET", "/v1/identity/group/id/"+g.ID, nil)
			if err != nil {
				continue
			}
			gData, _ := resp["data"].(map[string]any)
			if gData == nil {
				continue
			}
			currentMembers := mapStrSlice(gData, "member_entity_ids")
			if currentMembers == nil {
				currentMembers = []string{}
			}

			hasMember := false
			for _, m := range currentMembers {
				if m == id {
					hasMember = true
					break
				}
			}

			if desiredGroups[g.ID] && !hasMember {
				// Add entity to this group.
				newMembers := append(currentMembers, id)
				_, _ = s.baoRequest("POST", "/v1/identity/group/id/"+g.ID, map[string]any{
					"member_entity_ids": newMembers,
				})
			} else if !desiredGroups[g.ID] && hasMember {
				// Remove entity from this group.
				newMembers := make([]string, 0, len(currentMembers))
				for _, m := range currentMembers {
					if m != id {
						newMembers = append(newMembers, m)
					}
				}
				_, _ = s.baoRequest("POST", "/v1/identity/group/id/"+g.ID, map[string]any{
					"member_entity_ids": newMembers,
				})
			}
		}
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

// CreateGroup creates a new identity group with optional role assignments.
func (s *Store) CreateGroup(name string, roles []string) (Group, error) {
	if roles == nil {
		roles = []string{}
	}
	resp, err := s.baoRequest("POST", "/v1/identity/group", map[string]any{
		"name":     name,
		"policies": roles,
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
		Roles:     roles,
	}, nil
}

// UpdateGroup updates the name of an identity group by ID. Builtin groups cannot be renamed.
func (s *Store) UpdateGroup(id string, name string) (Group, bool, error) {
	// Read group first to check builtin flag.
	resp, err := s.baoRequest("GET", "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return Group{}, false, nil
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return Group{}, false, nil
	}
	if m, ok := data["metadata"].(map[string]any); ok {
		if b, ok := m["builtin"].(string); ok && b == "true" {
			return Group{}, false, fmt.Errorf("cannot rename builtin group")
		}
	}

	_, err = s.baoRequest("POST", "/v1/identity/group/id/"+id, map[string]any{
		"name": name,
	})
	if err != nil {
		return Group{}, false, fmt.Errorf("update group name: %w", err)
	}

	// Read back group to return current state.
	resp, err = s.baoRequest("GET", "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return Group{}, false, fmt.Errorf("read back group: %w", err)
	}
	data, _ = resp["data"].(map[string]any)
	if data == nil {
		return Group{}, false, nil
	}

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

// ── MFA (TOTP via OpenBao) ──────────────────────────────────────────────

// MFAEnrollResult holds the TOTP enrollment response from OpenBao.
type MFAEnrollResult struct {
	URL     string `json:"url"`
	Barcode string `json:"barcode"`
}

// mfaMethodID returns the TOTP method ID from the environment.
func (s *Store) mfaMethodID() string {
	return os.Getenv("OPENBAO_MFA_METHOD_ID")
}

// EnrollMFA generates a TOTP secret for an entity via the admin-generate endpoint.
func (s *Store) EnrollMFA(entityID string) (MFAEnrollResult, error) {
	methodID := s.mfaMethodID()
	if methodID == "" {
		return MFAEnrollResult{}, fmt.Errorf("OPENBAO_MFA_METHOD_ID not configured")
	}

	resp, err := s.baoRequest("POST", "/v1/identity/mfa/method/totp/admin-generate", map[string]any{
		"method_id": methodID,
		"entity_id": entityID,
	})
	if err != nil {
		return MFAEnrollResult{}, fmt.Errorf("enroll MFA: %w", err)
	}

	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return MFAEnrollResult{}, fmt.Errorf("enroll MFA: empty response data")
	}

	return MFAEnrollResult{
		URL:     mapStr(data, "url"),
		Barcode: mapStr(data, "barcode"),
	}, nil
}

// DestroyMFA removes TOTP MFA for an entity via the admin-destroy endpoint.
func (s *Store) DestroyMFA(entityID string) error {
	methodID := s.mfaMethodID()
	if methodID == "" {
		return fmt.Errorf("OPENBAO_MFA_METHOD_ID not configured")
	}

	_, err := s.baoRequest("POST", "/v1/identity/mfa/method/totp/admin-destroy", map[string]any{
		"method_id": methodID,
		"entity_id": entityID,
	})
	if err != nil {
		return fmt.Errorf("destroy MFA: %w", err)
	}
	return nil
}

// GetMFAStatus checks whether an entity has TOTP MFA configured by reading the
// entity and inspecting its mfa_secrets field.
func (s *Store) GetMFAStatus(entityID string) (bool, error) {
	resp, err := s.baoRequest("GET", "/v1/identity/entity/id/"+entityID, nil)
	if err != nil {
		return false, fmt.Errorf("get entity for MFA status: %w", err)
	}

	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return false, nil
	}

	// OpenBao stores TOTP secrets in mfa_secrets keyed by method ID.
	mfaSecrets, ok := data["mfa_secrets"].(map[string]any)
	if !ok || len(mfaSecrets) == 0 {
		return false, nil
	}

	methodID := s.mfaMethodID()
	if methodID == "" {
		return false, nil
	}

	_, enrolled := mfaSecrets[methodID]
	return enrolled, nil
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
		}, nil
	}

	defaults := GlobalSettings{
		RequireMFA:        false,
		SessionTimeoutMin: 30,
		PasswordMinLength: 12,
	}

	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return defaults, nil
	}

	// KV v2 nests actual data under data.data.
	inner, _ := data["data"].(map[string]any)
	if inner == nil {
		return defaults, nil
	}

	gs := GlobalSettings{
		RequireMFA:           false,
		SessionTimeoutMin:    30,
		PasswordMinLength:    12,
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

	return gs, nil
}

func (s *Store) SetSettings(gs GlobalSettings) (GlobalSettings, error) {
	_, err := s.baoRequest("PUT", "/v1/convocate/data/settings", map[string]any{
		"data": map[string]any{
			"requireMfa":           gs.RequireMFA,
			"sessionTimeoutMinutes": gs.SessionTimeoutMin,
			"passwordMinLength":    gs.PasswordMinLength,
		},
	})
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("write settings to openbao: %w", err)
	}
	return gs, nil
}
