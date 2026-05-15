package openbao

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal OpenBao HTTP client for the KV v2 secrets engine
// and policy management. It speaks the OpenBao/Vault REST API over HTTPS.
type Client struct {
	httpClient *http.Client
	address    string
	token      string
}

// Config holds connection parameters for an OpenBao client.
type Config struct {
	TLSConfig *tls.Config
	Address   string
	Token     string
	Timeout   time.Duration
}

// NewClient creates a new OpenBao client.
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	transport := &http.Transport{}
	if config.TLSConfig != nil {
		transport.TLSClientConfig = config.TLSConfig
	}
	return &Client{
		address: strings.TrimRight(config.Address, "/"),
		token:   config.Token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		},
	}
}

// SetToken updates the client's authentication token.
func (c *Client) SetToken(token string) {
	c.token = token
}

// --- KV v2 Operations ---

// KVWrite writes a secret to the KV v2 secrets engine at the given path.
// The mount defaults to "secret/".
func (c *Client) KVWrite(mount, path string, data map[string]interface{}) error {
	endpoint := fmt.Sprintf("/v1/%s/data/%s", mount, path)
	body := map[string]interface{}{
		"data": data,
	}
	_, err := c.doRequest("POST", endpoint, body)
	return err
}

// KVRead reads a secret from the KV v2 secrets engine at the given path.
// Returns the data map or nil if the secret does not exist.
func (c *Client) KVRead(mount, path string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/v1/%s/data/%s", mount, path)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	dataField, ok := resp["data"]
	if !ok {
		return nil, nil
	}
	dataMap, ok := dataField.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("openbao: unexpected data type: %T", dataField)
	}
	innerData, ok := dataMap["data"]
	if !ok {
		return nil, nil
	}
	innerMap, ok := innerData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("openbao: unexpected inner data type: %T", innerData)
	}
	return innerMap, nil
}

// KVDelete deletes a secret from the KV v2 secrets engine at the given path.
func (c *Client) KVDelete(mount, path string) error {
	endpoint := fmt.Sprintf("/v1/%s/metadata/%s", mount, path)
	_, err := c.doRequest("DELETE", endpoint, nil)
	return err
}

// --- Policy Management ---

// PolicyWrite creates or updates a policy with the given name and HCL rules.
func (c *Client) PolicyWrite(name, rules string) error {
	endpoint := fmt.Sprintf("/v1/sys/policies/acl/%s", name)
	body := map[string]interface{}{
		"policy": rules,
	}
	_, err := c.doRequest("PUT", endpoint, body)
	return err
}

// PolicyRead reads a policy by name. Returns the rules string.
func (c *Client) PolicyRead(name string) (string, error) {
	endpoint := fmt.Sprintf("/v1/sys/policies/acl/%s", name)
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	dataField, ok := resp["data"]
	if !ok {
		return "", nil
	}
	dataMap, ok := dataField.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("openbao: unexpected policy data type: %T", dataField)
	}
	rules, ok := dataMap["rules"]
	if !ok {
		return "", nil
	}
	rulesStr, ok := rules.(string)
	if !ok {
		return "", fmt.Errorf("openbao: unexpected rules type: %T", rules)
	}
	return rulesStr, nil
}

// PolicyDelete deletes a policy by name.
func (c *Client) PolicyDelete(name string) error {
	endpoint := fmt.Sprintf("/v1/sys/policies/acl/%s", name)
	_, err := c.doRequest("DELETE", endpoint, nil)
	return err
}

// --- AppRole Auth ---

// AppRoleLogin authenticates with the AppRole auth method and returns
// the client token.
func (c *Client) AppRoleLogin(mountPath, roleID, secretID string) (string, error) {
	endpoint := fmt.Sprintf("/v1/auth/%s/login", mountPath)
	body := map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	}
	resp, err := c.doRequest("POST", endpoint, body)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("openbao: empty AppRole login response")
	}
	authField, ok := resp["auth"]
	if !ok {
		return "", fmt.Errorf("openbao: missing auth field in login response")
	}
	authMap, ok := authField.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("openbao: unexpected auth type: %T", authField)
	}
	token, ok := authMap["client_token"]
	if !ok {
		return "", fmt.Errorf("openbao: missing client_token in auth")
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", fmt.Errorf("openbao: unexpected token type: %T", token)
	}
	return tokenStr, nil
}

// --- Health ---

// Health checks the OpenBao server health. Returns nil if healthy.
func (c *Client) Health() error {
	endpoint := "/v1/sys/health"
	req, err := http.NewRequestWithContext(context.Background(), "GET", c.address+endpoint, http.NoBody)
	if err != nil {
		return fmt.Errorf("openbao: create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openbao: health check: %w", err)
	}
	defer resp.Body.Close()
	// 200 = initialized+unsealed+active, 429 = standby, 472 = DR secondary,
	// 473 = performance standby. All are "healthy enough" for our purposes.
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("openbao: unhealthy status %d: %s", resp.StatusCode, string(body))
}

// --- Initialization ---

// InitStatus checks whether OpenBao has been initialized.
func (c *Client) InitStatus() (bool, error) {
	endpoint := "/v1/sys/init"
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, fmt.Errorf("openbao: empty init status response")
	}
	initialized, ok := resp["initialized"]
	if !ok {
		return false, fmt.Errorf("openbao: missing initialized field")
	}
	boolVal, ok := initialized.(bool)
	if !ok {
		return false, fmt.Errorf("openbao: unexpected initialized type: %T", initialized)
	}
	return boolVal, nil
}

// --- Internal HTTP ---

// Error represents an error response from the OpenBao API.
type Error struct {
	Errors     []string
	StatusCode int
}

func (e *Error) Error() string {
	return fmt.Sprintf("openbao: HTTP %d: %s", e.StatusCode, strings.Join(e.Errors, "; "))
}

func (c *Client) doRequest(method, endpoint string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("openbao: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, c.address+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("openbao: create request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("X-Vault-Token", c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openbao: %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openbao: read response: %w", err)
	}

	// 204 No Content is a valid success response (e.g. DELETE).
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	// 404 means not found — return nil data, no error.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Errors []string `json:"errors"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Errors:     errResp.Errors,
		}
	}

	if len(respBody) == 0 {
		return nil, nil
	}

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return nil, fmt.Errorf("openbao: unmarshal response: %w", err)
	}
	return result, nil
}
