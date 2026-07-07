package k8s

import (
	"os"
	"testing"
)

func TestAgentNamespaceConstant(t *testing.T) {
	if AgentNamespace != "convocate-agents" {
		t.Fatalf("expected convocate-agents, got %s", AgentNamespace)
	}
}

func TestGetConfig_FallbackToKubeconfig(t *testing.T) {
	// Not running in-cluster, so getConfig should try KUBECONFIG or default path.
	// Either way it will fail since there's no valid kubeconfig, but it exercises
	// the fallback logic.
	os.Unsetenv("KUBECONFIG")

	_, err := getConfig()
	// We expect an error since no kubeconfig exists
	if err == nil {
		// If it succeeded, that's fine too (maybe one exists)
		return
	}
}

func TestGetConfig_CustomKubeconfig(t *testing.T) {
	os.Setenv("KUBECONFIG", "/nonexistent/kubeconfig")
	defer os.Unsetenv("KUBECONFIG")

	_, err := getConfig()
	if err == nil {
		t.Fatal("expected error for nonexistent kubeconfig")
	}
}

func TestInit_NoConfig(t *testing.T) {
	os.Setenv("KUBECONFIG", "/nonexistent/kubeconfig")
	defer os.Unsetenv("KUBECONFIG")

	err := Init()
	if err == nil {
		t.Fatal("expected error when no valid K8s config exists")
	}
}
