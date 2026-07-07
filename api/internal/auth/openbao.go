package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// openbaoAddr returns the OpenBao server address from env or default.
func openbaoAddr() string {
	if addr := os.Getenv("OPENBAO_ADDR"); addr != "" {
		return strings.TrimRight(addr, "/")
	}
	return "http://openbao.security.svc:8200"
}

// mfaRequirement represents the MFA challenge returned by OpenBao when MFA is enforced.
type mfaRequirement struct {
	MFARequestID string `json:"mfa_request_id"`
}

// openbaoLoginResponse represents the auth block from a userpass login.
type openbaoLoginResponse struct {
	Auth struct {
		ClientToken    string            `json:"client_token"`
		EntityID       string            `json:"entity_id"`
		Policies       []string          `json:"policies"`
		Metadata       map[string]string `json:"metadata"`
		LeaseDuration  int               `json:"lease_duration"`
		MFARequirement *mfaRequirement   `json:"mfa_requirement"`
	} `json:"auth"`
	Warnings []string `json:"warnings"`
}

// openbaoEntityResponse represents an identity entity lookup.
type openbaoEntityResponse struct {
	Data struct {
		ID       string            `json:"id"`
		Name     string            `json:"name"`
		Metadata map[string]string `json:"metadata"`
		Policies []string          `json:"policies"`
		GroupIDs []string          `json:"group_ids"`
	} `json:"data"`
}

// openbaoTokenLookupResponse represents a token lookup-self response.
type openbaoTokenLookupResponse struct {
	Data struct {
		EntityID string   `json:"entity_id"`
		Policies []string `json:"policies"`
		Metadata map[string]string `json:"meta"`
	} `json:"data"`
}

// openbaoLogin authenticates a user via the userpass auth method.
func openbaoLogin(username, password string) (*openbaoLoginResponse, error) {
	url := fmt.Sprintf("%s/v1/auth/userpass/login/%s", openbaoAddr(), username)

	body, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		return nil, fmt.Errorf("marshal login body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result openbaoLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}
	return &result, nil
}

// openbaoMFAMethodID returns the MFA method ID from env.
func openbaoMFAMethodID() string {
	return os.Getenv("OPENBAO_MFA_METHOD_ID")
}

// openbaoMFAValidate completes the MFA two-step login by validating a TOTP code.
func openbaoMFAValidate(mfaRequestID, methodID, totpCode string) (*openbaoLoginResponse, error) {
	url := fmt.Sprintf("%s/v1/sys/mfa/validate", openbaoAddr())

	payload := map[string]any{
		"mfa_request_id": mfaRequestID,
		"mfa_payload": map[string][]string{
			methodID: {totpCode},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mfa validate body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create mfa validate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mfa validate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("mfa validation failed")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mfa validate unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result openbaoLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode mfa validate response: %w", err)
	}
	return &result, nil
}

// openbaoLookupEntity fetches entity metadata by entity ID using the given token.
func openbaoLookupEntity(token, entityID string) (*openbaoEntityResponse, error) {
	url := fmt.Sprintf("%s/v1/identity/entity/id/%s", openbaoAddr(), entityID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create entity request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity lookup failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("entity lookup status %d: %s", resp.StatusCode, string(respBody))
	}

	var result openbaoEntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode entity response: %w", err)
	}
	return &result, nil
}

// openbaoTokenLookupSelf validates a token and returns its metadata.
func openbaoTokenLookupSelf(token string) (*openbaoTokenLookupResponse, error) {
	url := fmt.Sprintf("%s/v1/auth/token/lookup-self", openbaoAddr())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create token lookup request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token lookup failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token lookup failed with status %d", resp.StatusCode)
	}

	var result openbaoTokenLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode token lookup response: %w", err)
	}
	return &result, nil
}

// openbaoRevokeSelf revokes the given token.
func openbaoRevokeSelf(token string) error {
	url := fmt.Sprintf("%s/v1/auth/token/revoke-self", openbaoAddr())

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create revoke request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revoke failed with status %d", resp.StatusCode)
	}
	return nil
}

// allApplets is the full list of applet shortnames.
var allApplets = []string{"nmgr", "amgr", "pb", "ide", "repo", "ac", "sup"}

// rolesToApplets maps a list of OpenBao policies/roles to authorized applets.
func rolesToApplets(policies []string) []string {
	appletSet := make(map[string]bool)

	for _, policy := range policies {
		p := strings.ToLower(policy)
		if p == "admin-policy" || p == "admin" {
			// Admin gets all applets
			return allApplets
		}
		switch {
		case strings.Contains(p, "node-"):
			appletSet["nmgr"] = true
		case strings.Contains(p, "agent-"):
			appletSet["amgr"] = true
		case strings.Contains(p, "pb-"):
			appletSet["pb"] = true
		case strings.Contains(p, "ide-"):
			appletSet["ide"] = true
		case strings.Contains(p, "access-"):
			appletSet["ac"] = true
		case strings.Contains(p, "repo-"):
			appletSet["repo"] = true
		case strings.Contains(p, "support-"):
			appletSet["sup"] = true
		}
	}

	applets := make([]string, 0, len(appletSet))
	for a := range appletSet {
		applets = append(applets, a)
	}
	return applets
}

// buildPrincipalFromEntity constructs a Principal from an OpenBao entity and policies.
func buildPrincipalFromEntity(entity *openbaoEntityResponse, policies []string) *httputil.Principal {
	meta := entity.Data.Metadata

	// Combine entity policies with auth policies
	allPolicies := make([]string, 0, len(entity.Data.Policies)+len(policies))
	allPolicies = append(allPolicies, entity.Data.Policies...)
	allPolicies = append(allPolicies, policies...)

	// Deduplicate policies
	seen := make(map[string]bool)
	roles := make([]string, 0, len(allPolicies))
	for _, p := range allPolicies {
		if p != "" && !seen[p] {
			seen[p] = true
			roles = append(roles, p)
		}
	}

	name := meta["name"]
	if name == "" {
		name = entity.Data.Name
	}

	return &httputil.Principal{
		ID:                entity.Data.ID,
		Username:          entity.Data.Name,
		Name:              name,
		Email:             meta["email"],
		Groups:            entity.Data.GroupIDs,
		Roles:             roles,
		IDP:               "openbao",
		AuthorizedApplets: rolesToApplets(roles),
	}
}
