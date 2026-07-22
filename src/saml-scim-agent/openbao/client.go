package openbao

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an HTTP client for the OpenBao API.
type Client struct {
	Addr  string
	Token string
	HTTP  *http.Client
}

// NewClient creates a new OpenBao client.
func NewClient(addr, token string, skipTLSVerify bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify,
		},
	}
	return &Client{
		Addr:  strings.TrimRight(addr, "/"),
		Token: token,
		HTTP: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// request performs an authenticated HTTP request to OpenBao.
func (c *Client) request(method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.Addr + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// requestWithToken performs an authenticated HTTP request using a provided token.
func (c *Client) requestWithToken(method, path, token string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.Addr + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// CheckHealth checks if OpenBao is reachable and unsealed.
func (c *Client) CheckHealth() error {
	_, status, err := c.request(http.MethodGet, "/v1/sys/health", nil)
	if err != nil {
		return fmt.Errorf("openbao health check failed: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("openbao unhealthy: status %d", status)
	}
	return nil
}

// Authenticate validates username/password against OpenBao userpass and returns a client token.
func (c *Client) Authenticate(username, password string) (string, error) {
	body := map[string]string{"password": password}
	respBody, status, err := c.request(http.MethodPost, "/v1/auth/userpass/login/"+username, body)
	if err != nil {
		return "", fmt.Errorf("authentication request failed: %w", err)
	}
	if status != 200 {
		return "", fmt.Errorf("authentication failed: status %d", status)
	}

	var result struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse auth response: %w", err)
	}
	return result.Auth.ClientToken, nil
}

// KVRead reads a secret from the KV v2 engine.
func (c *Client) KVRead(path string) (map[string]interface{}, error) {
	respBody, status, err := c.request(http.MethodGet, "/v1/"+path, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("kv read %s: status %d", path, status)
	}

	var result struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse kv response: %w", err)
	}
	return result.Data.Data, nil
}

// KVWrite writes a secret to the KV v2 engine.
func (c *Client) KVWrite(path string, data map[string]interface{}) error {
	body := map[string]interface{}{"data": data}
	_, status, err := c.request(http.MethodPost, "/v1/"+path, body)
	if err != nil {
		return err
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("kv write %s: status %d", path, status)
	}
	return nil
}

// Entity represents an OpenBao identity entity.
type Entity struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata"`
	Disabled bool              `json:"disabled"`
	GroupIDs []string          `json:"group_ids"`
}

// GetEntityByName retrieves an entity by name.
func (c *Client) GetEntityByName(name string) (*Entity, error) {
	respBody, status, err := c.request(http.MethodGet, "/v1/identity/entity/name/"+name, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("get entity %s: status %d", name, status)
	}

	var result struct {
		Data Entity `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse entity response: %w", err)
	}
	return &result.Data, nil
}

// GetEntityByID retrieves an entity by ID.
func (c *Client) GetEntityByID(id string) (*Entity, error) {
	respBody, status, err := c.request(http.MethodGet, "/v1/identity/entity/id/"+id, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("get entity id %s: status %d", id, status)
	}

	var result struct {
		Data Entity `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse entity response: %w", err)
	}
	return &result.Data, nil
}

// CreateEntity creates a new identity entity.
func (c *Client) CreateEntity(name string, metadata map[string]string) (*Entity, error) {
	body := map[string]interface{}{
		"name":     name,
		"metadata": metadata,
	}
	respBody, status, err := c.request(http.MethodPost, "/v1/identity/entity", body)
	if err != nil {
		return nil, err
	}
	if status != 200 && status != 204 {
		return nil, fmt.Errorf("create entity %s: status %d", name, status)
	}

	var result struct {
		Data Entity `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse create entity response: %w", err)
	}
	return &result.Data, nil
}

// UpdateEntity updates an existing entity by name.
func (c *Client) UpdateEntity(name string, metadata map[string]string, disabled bool) error {
	body := map[string]interface{}{
		"metadata": metadata,
		"disabled": disabled,
	}
	_, status, err := c.request(http.MethodPut, "/v1/identity/entity/name/"+name, body)
	if err != nil {
		return err
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("update entity %s: status %d", name, status)
	}
	return nil
}

// DeleteEntity deletes an entity by name.
func (c *Client) DeleteEntity(name string) error {
	_, status, err := c.request(http.MethodDelete, "/v1/identity/entity/name/"+name, nil)
	if err != nil {
		return err
	}
	if status != 204 && status != 200 {
		return fmt.Errorf("delete entity %s: status %d", name, status)
	}
	return nil
}

// CreateUserpass creates a userpass user in OpenBao.
func (c *Client) CreateUserpass(username, password string) error {
	body := map[string]string{"password": password}
	_, status, err := c.request(http.MethodPost, "/v1/auth/userpass/users/"+username, body)
	if err != nil {
		return err
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("create userpass %s: status %d", username, status)
	}
	return nil
}

// DeleteUserpass deletes a userpass user.
func (c *Client) DeleteUserpass(username string) error {
	_, status, err := c.request(http.MethodDelete, "/v1/auth/userpass/users/"+username, nil)
	if err != nil {
		return err
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("delete userpass %s: status %d", username, status)
	}
	return nil
}

// Group represents an OpenBao identity group.
type Group struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Metadata        map[string]string `json:"metadata"`
	MemberEntityIDs []string          `json:"member_entity_ids"`
	Type            string            `json:"type"`
}

// GetGroupByName retrieves a group by name.
func (c *Client) GetGroupByName(name string) (*Group, error) {
	respBody, status, err := c.request(http.MethodGet, "/v1/identity/group/name/"+name, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("get group %s: status %d", name, status)
	}

	var result struct {
		Data Group `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse group response: %w", err)
	}
	return &result.Data, nil
}

// GetGroupByID retrieves a group by ID.
func (c *Client) GetGroupByID(id string) (*Group, error) {
	respBody, status, err := c.request(http.MethodGet, "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("get group id %s: status %d", id, status)
	}

	var result struct {
		Data Group `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse group response: %w", err)
	}
	return &result.Data, nil
}

// CreateGroup creates a new identity group.
func (c *Client) CreateGroup(name string, memberEntityIDs []string, metadata map[string]string) (*Group, error) {
	body := map[string]interface{}{
		"name":              name,
		"type":              "internal",
		"member_entity_ids": memberEntityIDs,
		"metadata":          metadata,
	}
	respBody, status, err := c.request(http.MethodPost, "/v1/identity/group", body)
	if err != nil {
		return nil, err
	}
	if status != 200 && status != 204 {
		return nil, fmt.Errorf("create group %s: status %d", name, status)
	}

	var result struct {
		Data Group `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse create group response: %w", err)
	}
	return &result.Data, nil
}

// UpdateGroup updates a group.
func (c *Client) UpdateGroup(name string, memberEntityIDs []string, metadata map[string]string) error {
	body := map[string]interface{}{
		"name":              name,
		"type":              "internal",
		"member_entity_ids": memberEntityIDs,
		"metadata":          metadata,
	}
	_, status, err := c.request(http.MethodPost, "/v1/identity/group", body)
	if err != nil {
		return err
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("update group %s: status %d", name, status)
	}
	return nil
}

// DeleteGroup deletes a group by ID.
func (c *Client) DeleteGroup(id string) error {
	_, status, err := c.request(http.MethodDelete, "/v1/identity/group/id/"+id, nil)
	if err != nil {
		return err
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("delete group %s: status %d", id, status)
	}
	return nil
}

// ListEntities lists all identity entity names.
func (c *Client) ListEntities() ([]string, error) {
	respBody, status, err := c.request("LIST", "/v1/identity/entity/name", nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("list entities: status %d", status)
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse list entities response: %w", err)
	}
	return result.Data.Keys, nil
}

// ListGroups lists all identity group names.
func (c *Client) ListGroups() ([]string, error) {
	respBody, status, err := c.request("LIST", "/v1/identity/group/name", nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("list groups: status %d", status)
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse list groups response: %w", err)
	}
	return result.Data.Keys, nil
}

// GetEntityByNameWithToken retrieves an entity using a specific token.
func (c *Client) GetEntityByNameWithToken(name, token string) (*Entity, error) {
	respBody, status, err := c.requestWithToken(http.MethodGet, "/v1/identity/entity/name/"+name, token, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("get entity %s: status %d", name, status)
	}

	var result struct {
		Data Entity `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse entity response: %w", err)
	}
	return &result.Data, nil
}
