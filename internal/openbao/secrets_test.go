package openbao

import (
	"testing"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func TestStoreReadProjectSecrets(t *testing.T) {
	client, _ := testClient(t)
	projectID := uuid.MustNew()

	secrets := ProjectSecrets{
		SSHPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----",
		GitHubPAT:     "ghp_abc123",
		CustomSecrets: map[string]string{
			"NPM_TOKEN":    "tok_npm",
			"EXTRA_SECRET": "extra_val",
		},
	}

	err := client.StoreProjectSecrets(projectID, secrets)
	if err != nil {
		t.Fatalf("StoreProjectSecrets error: %v", err)
	}

	got, err := client.ReadProjectSecrets(projectID)
	if err != nil {
		t.Fatalf("ReadProjectSecrets error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadProjectSecrets returned nil")
	}

	if got.SSHPrivateKey != secrets.SSHPrivateKey {
		t.Errorf("SSHPrivateKey mismatch")
	}
	if got.GitHubPAT != secrets.GitHubPAT {
		t.Errorf("GitHubPAT: got %q, want %q", got.GitHubPAT, secrets.GitHubPAT)
	}
	if got.CustomSecrets["NPM_TOKEN"] != "tok_npm" {
		t.Errorf("NPM_TOKEN: got %q, want %q", got.CustomSecrets["NPM_TOKEN"], "tok_npm")
	}
	if got.CustomSecrets["EXTRA_SECRET"] != "extra_val" {
		t.Errorf("EXTRA_SECRET: got %q, want %q", got.CustomSecrets["EXTRA_SECRET"], "extra_val")
	}
}

func TestReadProjectSecretsNotFound(t *testing.T) {
	client, _ := testClient(t)
	got, err := client.ReadProjectSecrets(uuid.MustNew())
	if err != nil {
		t.Fatalf("ReadProjectSecrets error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestDeleteProjectSecrets(t *testing.T) {
	client, _ := testClient(t)
	projectID := uuid.MustNew()

	err := client.StoreProjectSecrets(projectID, ProjectSecrets{
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	})
	if err != nil {
		t.Fatalf("StoreProjectSecrets error: %v", err)
	}

	err = client.DeleteProjectSecrets(projectID)
	if err != nil {
		t.Fatalf("DeleteProjectSecrets error: %v", err)
	}

	got, err := client.ReadProjectSecrets(projectID)
	if err != nil {
		t.Fatalf("ReadProjectSecrets error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestStoreReadSharedCredential(t *testing.T) {
	client, _ := testClient(t)

	err := client.StoreSharedCredential("anthropic_api_key", "sk-ant-123")
	if err != nil {
		t.Fatalf("StoreSharedCredential error: %v", err)
	}

	got, err := client.ReadSharedCredential("anthropic_api_key")
	if err != nil {
		t.Fatalf("ReadSharedCredential error: %v", err)
	}
	if got != "sk-ant-123" {
		t.Errorf("got %q, want %q", got, "sk-ant-123")
	}
}

func TestReadSharedCredentialNotFound(t *testing.T) {
	client, _ := testClient(t)
	got, err := client.ReadSharedCredential("nonexistent")
	if err != nil {
		t.Fatalf("ReadSharedCredential error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDeleteSharedCredential(t *testing.T) {
	client, _ := testClient(t)

	err := client.StoreSharedCredential("to-delete", "val")
	if err != nil {
		t.Fatalf("StoreSharedCredential error: %v", err)
	}

	err = client.DeleteSharedCredential("to-delete")
	if err != nil {
		t.Fatalf("DeleteSharedCredential error: %v", err)
	}

	got, err := client.ReadSharedCredential("to-delete")
	if err != nil {
		t.Fatalf("ReadSharedCredential error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty after delete, got %q", got)
	}
}

func TestWriteReadProjectPolicy(t *testing.T) {
	client, _ := testClient(t)
	projectID := uuid.MustNew()

	err := client.WriteProjectPolicy(projectID)
	if err != nil {
		t.Fatalf("WriteProjectPolicy error: %v", err)
	}

	policyName := ProjectPolicyName(projectID)
	rules, err := client.PolicyRead(policyName)
	if err != nil {
		t.Fatalf("PolicyRead error: %v", err)
	}
	if rules == "" {
		t.Error("expected non-empty policy rules")
	}
	expectedPath := "secret/data/projects/" + projectID.String()
	if !containsSubstring(rules, expectedPath) {
		t.Errorf("policy rules should contain %q, got %q", expectedPath, rules)
	}
}

func TestDeleteProjectPolicy(t *testing.T) {
	client, _ := testClient(t)
	projectID := uuid.MustNew()

	err := client.WriteProjectPolicy(projectID)
	if err != nil {
		t.Fatalf("WriteProjectPolicy error: %v", err)
	}

	err = client.DeleteProjectPolicy(projectID)
	if err != nil {
		t.Fatalf("DeleteProjectPolicy error: %v", err)
	}

	policyName := ProjectPolicyName(projectID)
	rules, err := client.PolicyRead(policyName)
	if err != nil {
		t.Fatalf("PolicyRead error: %v", err)
	}
	if rules != "" {
		t.Errorf("expected empty after delete, got %q", rules)
	}
}

func TestProjectPolicyName(t *testing.T) {
	id := uuid.MustNew()
	name := ProjectPolicyName(id)
	if !containsSubstring(name, "convocate-project-") {
		t.Errorf("policy name should start with convocate-project-, got %q", name)
	}
	if !containsSubstring(name, id.String()) {
		t.Errorf("policy name should contain project ID, got %q", name)
	}
}

func TestProjectSecretsNoCustom(t *testing.T) {
	client, _ := testClient(t)
	projectID := uuid.MustNew()

	secrets := ProjectSecrets{
		SSHPrivateKey: "key",
		GitHubPAT:     "pat",
	}
	err := client.StoreProjectSecrets(projectID, secrets)
	if err != nil {
		t.Fatalf("StoreProjectSecrets error: %v", err)
	}

	got, err := client.ReadProjectSecrets(projectID)
	if err != nil {
		t.Fatalf("ReadProjectSecrets error: %v", err)
	}
	if len(got.CustomSecrets) != 0 {
		t.Errorf("expected no custom secrets, got %v", got.CustomSecrets)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
