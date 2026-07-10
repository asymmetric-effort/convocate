package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
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

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	kc := filepath.Join(dir, "kubeconfig")
	content := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    token: fake-token
`
	os.WriteFile(kc, []byte(content), 0600)
	return kc
}

func TestInit_Success(t *testing.T) {
	origClient := Client
	origDyn := DynClient
	origNewCS := newClientset
	origNewDyn := newDynClient
	defer func() {
		Client = origClient
		DynClient = origDyn
		newClientset = origNewCS
		newDynClient = origNewDyn
	}()

	kc := writeTestKubeconfig(t)
	os.Setenv("KUBECONFIG", kc)
	defer os.Unsetenv("KUBECONFIG")

	// Mock the client constructors to avoid real connections
	newClientset = func(config *rest.Config) (kubernetes.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	newDynClient = func(config *rest.Config) (dynamic.Interface, error) {
		return nil, nil // nil is acceptable for tests
	}

	err := Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Client == nil {
		t.Fatal("expected Client to be set")
	}
}

func TestInit_ClientsetError(t *testing.T) {
	origNewCS := newClientset
	origNewDyn := newDynClient
	defer func() {
		newClientset = origNewCS
		newDynClient = origNewDyn
	}()

	kc := writeTestKubeconfig(t)
	os.Setenv("KUBECONFIG", kc)
	defer os.Unsetenv("KUBECONFIG")

	newClientset = func(config *rest.Config) (kubernetes.Interface, error) {
		return nil, fmt.Errorf("clientset creation failed")
	}

	err := Init()
	if err == nil {
		t.Fatal("expected error for clientset creation failure")
	}
}

func TestInit_DynClientError(t *testing.T) {
	origNewCS := newClientset
	origNewDyn := newDynClient
	defer func() {
		newClientset = origNewCS
		newDynClient = origNewDyn
	}()

	kc := writeTestKubeconfig(t)
	os.Setenv("KUBECONFIG", kc)
	defer os.Unsetenv("KUBECONFIG")

	newClientset = func(config *rest.Config) (kubernetes.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	newDynClient = func(config *rest.Config) (dynamic.Interface, error) {
		return nil, fmt.Errorf("dynamic client creation failed")
	}

	err := Init()
	if err == nil {
		t.Fatal("expected error for dynamic client creation failure")
	}
}

func TestGetConfig_InClusterSuccess(t *testing.T) {
	origInCluster := inClusterConfig
	defer func() { inClusterConfig = origInCluster }()

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "https://kubernetes.default.svc:443"}, nil
	}

	config, err := getConfig()
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if config.Host != "https://kubernetes.default.svc:443" {
		t.Fatalf("expected in-cluster host, got %s", config.Host)
	}
}

func TestGetConfig_ValidKubeconfig(t *testing.T) {
	kc := writeTestKubeconfig(t)
	os.Setenv("KUBECONFIG", kc)
	defer os.Unsetenv("KUBECONFIG")

	config, err := getConfig()
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.Host != "https://127.0.0.1:6443" {
		t.Fatalf("expected host https://127.0.0.1:6443, got %s", config.Host)
	}
}
