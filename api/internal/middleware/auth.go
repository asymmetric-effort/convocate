package middleware

import (
	"encoding/json"
	"fmt"
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

// tokenLookupResponse represents the response from token/lookup-self.
type tokenLookupResponse struct {
	Data struct {
		EntityID string   `json:"entity_id"`
		Policies []string `json:"policies"`
	} `json:"data"`
}

// entityResponse represents the response from identity/entity/id/{id}.
type entityResponse struct {
	Data struct {
		ID       string            `json:"id"`
		Name     string            `json:"name"`
		Metadata map[string]string `json:"metadata"`
		Policies []string          `json:"policies"`
		GroupIDs []string          `json:"group_ids"`
	} `json:"data"`
}

// allApplets is the full list of applet shortnames.
var allApplets = []string{"nmgr", "amgr", "pb", "ide", "repo", "ac", "sup"}

// rolesToApplets maps OpenBao policies to authorized applets.
func rolesToApplets(policies []string) []string {
	appletSet := make(map[string]bool)
	for _, policy := range policies {
		p := strings.ToLower(policy)
		if p == "admin-policy" || p == "admin" {
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

// lookupTokenSelf validates a token via OpenBao token/lookup-self.
func lookupTokenSelf(token string) (*tokenLookupResponse, error) {
	url := fmt.Sprintf("%s/v1/auth/token/lookup-self", openbaoAddr())
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token lookup failed: status %d", resp.StatusCode)
	}

	var result tokenLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// lookupEntity fetches entity metadata from OpenBao.
func lookupEntity(token, entityID string) (*entityResponse, error) {
	url := fmt.Sprintf("%s/v1/identity/entity/id/%s", openbaoAddr(), entityID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("entity lookup failed: status %d", resp.StatusCode)
	}

	var result entityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// buildPrincipal constructs a Principal from entity data and policies.
func buildPrincipal(entity *entityResponse, policies []string) *httputil.Principal {
	meta := entity.Data.Metadata

	allPolicies := make([]string, 0, len(entity.Data.Policies)+len(policies))
	allPolicies = append(allPolicies, entity.Data.Policies...)
	allPolicies = append(allPolicies, policies...)

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

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// Fallback: accept ?token= query param for EventSource/WebSocket
		// (browser APIs cannot send custom headers on these connections).
		if (!strings.HasPrefix(auth, "Bearer ") || len(auth) <= 7) && r.URL.Query().Get("token") != "" {
			auth = "Bearer " + r.URL.Query().Get("token")
		}
		if !strings.HasPrefix(auth, "Bearer ") || len(auth) <= 7 {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
			return
		}

		token := auth[7:] // strip "Bearer "

		// Validate token via OpenBao
		lookupResp, err := lookupTokenSelf(token)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		// Look up entity for full principal data
		if lookupResp.Data.EntityID == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "token has no associated entity")
			return
		}

		entity, err := lookupEntity(token, lookupResp.Data.EntityID)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "failed to resolve identity")
			return
		}

		principal := buildPrincipal(entity, lookupResp.Data.Policies)
		ctx := httputil.ContextWithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

