package openbao

import (
	"fmt"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// Convocate-specific secret paths and policy templates.

const (
	kvMount = "secret"

	// Per-project secrets live under secret/data/projects/<projectID>/
	projectPrefix = "projects"

	// Shared service credentials live under secret/data/shared/
	sharedPrefix = "shared"
)

// ProjectSecrets holds the credentials for a single project.
type ProjectSecrets struct {
	SSHPrivateKey string            `json:"ssh_private_key"`
	GitHubPAT     string            `json:"github_pat"`
	CustomSecrets map[string]string `json:"custom_secrets,omitempty"`
}

// StoreProjectSecrets writes a project's secrets to OpenBao.
func (c *Client) StoreProjectSecrets(projectID uuid.UUID, secrets ProjectSecrets) error {
	path := fmt.Sprintf("%s/%s", projectPrefix, projectID.String())
	data := map[string]interface{}{
		"ssh_private_key": secrets.SSHPrivateKey,
		"github_pat":      secrets.GitHubPAT,
	}
	for key, val := range secrets.CustomSecrets {
		data["custom_"+key] = val
	}
	return c.KVWrite(kvMount, path, data)
}

// ReadProjectSecrets reads a project's secrets from OpenBao.
func (c *Client) ReadProjectSecrets(projectID uuid.UUID) (*ProjectSecrets, error) {
	path := fmt.Sprintf("%s/%s", projectPrefix, projectID.String())
	data, err := c.KVRead(kvMount, path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	secrets := &ProjectSecrets{
		CustomSecrets: make(map[string]string),
	}
	if v, ok := data["ssh_private_key"].(string); ok {
		secrets.SSHPrivateKey = v
	}
	if v, ok := data["github_pat"].(string); ok {
		secrets.GitHubPAT = v
	}
	for key, val := range data {
		if len(key) > 7 && key[:7] == "custom_" {
			if s, ok := val.(string); ok {
				secrets.CustomSecrets[key[7:]] = s
			}
		}
	}
	return secrets, nil
}

// DeleteProjectSecrets removes a project's secrets from OpenBao.
func (c *Client) DeleteProjectSecrets(projectID uuid.UUID) error {
	path := fmt.Sprintf("%s/%s", projectPrefix, projectID.String())
	return c.KVDelete(kvMount, path)
}

// StoreSharedCredential writes a shared service credential (e.g.
// Anthropic API key or Claude.ai session token).
func (c *Client) StoreSharedCredential(name, value string) error {
	path := fmt.Sprintf("%s/%s", sharedPrefix, name)
	return c.KVWrite(kvMount, path, map[string]interface{}{
		"value": value,
	})
}

// ReadSharedCredential reads a shared service credential.
func (c *Client) ReadSharedCredential(name string) (string, error) {
	path := fmt.Sprintf("%s/%s", sharedPrefix, name)
	data, err := c.KVRead(kvMount, path)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", nil
	}
	val, ok := data["value"].(string)
	if !ok {
		return "", nil
	}
	return val, nil
}

// DeleteSharedCredential removes a shared service credential.
func (c *Client) DeleteSharedCredential(name string) error {
	path := fmt.Sprintf("%s/%s", sharedPrefix, name)
	return c.KVDelete(kvMount, path)
}

// ProjectPolicyName returns the policy name for a given project's host binding.
func ProjectPolicyName(projectID uuid.UUID) string {
	return fmt.Sprintf("convocate-project-%s", projectID.String())
}

// WriteProjectPolicy creates an OpenBao policy that authorizes reading
// the secrets for the given project. This policy is bound to the host's
// AppRole when the Router API binds a project to a container on that host.
func (c *Client) WriteProjectPolicy(projectID uuid.UUID) error {
	name := ProjectPolicyName(projectID)
	rules := fmt.Sprintf(`
path "%s/data/%s/%s" {
  capabilities = ["read"]
}
`, kvMount, projectPrefix, projectID.String())
	return c.PolicyWrite(name, rules)
}

// DeleteProjectPolicy removes the policy for a project.
func (c *Client) DeleteProjectPolicy(projectID uuid.UUID) error {
	return c.PolicyDelete(ProjectPolicyName(projectID))
}
