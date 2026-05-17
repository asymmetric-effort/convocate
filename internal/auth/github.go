package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHubUser represents a GitHub user profile.
type GitHubUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubTokenResponse is the response from the GitHub OAuth token endpoint.
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// GitHubClient handles communication with the GitHub API for OAuth.
type GitHubClient struct {
	httpClient   *http.Client
	tokenURL     string
	apiBaseURL   string
	clientID     string
	clientSecret string
}

// NewGitHubClient creates a new GitHubClient.
func NewGitHubClient(clientID, clientSecret string) *GitHubClient {
	return &GitHubClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		tokenURL:     "https://github.com/login/oauth/access_token",
		apiBaseURL:   "https://api.github.com",
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// ExchangeCode exchanges an authorization code for an access token.
func (g *GitHubClient) ExchangeCode(code string) (*GitHubTokenResponse, error) {
	form := url.Values{
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"code":          {code},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("auth: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth: read token response: %w", err)
	}

	var tokenResp GitHubTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("auth: parse token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("auth: github oauth error: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &tokenResp, nil
}

// GetUser fetches the authenticated user's profile.
func (g *GitHubClient) GetUser(accessToken string) (*GitHubUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiBaseURL+"/user", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("auth: create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: github /user returned %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("auth: decode user: %w", err)
	}
	return &user, nil
}

// CheckOrgMembership verifies that the authenticated user is a member of the given org.
// Uses /user/memberships/orgs/{org} which works even with private membership.
func (g *GitHubClient) CheckOrgMembership(accessToken, org, username string) (bool, error) {
	// Use the user's own membership endpoint — works regardless of visibility setting.
	u := fmt.Sprintf("%s/user/memberships/orgs/%s", g.apiBaseURL, org)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("auth: create org check request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("auth: check org membership: %w", err)
	}
	defer resp.Body.Close()

	// 200 = member (active or pending), 403 = not a member, 404 = org not found
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}
