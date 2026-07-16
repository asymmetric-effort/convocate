package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const AgentNamespace = "convocate-agents"

var Client kubernetes.Interface
var DynClient dynamic.Interface

// newClientset creates a typed K8s client from config. Tests replace this.
var newClientset = func(config *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(config)
}

// newDynClient creates a dynamic K8s client from config. Tests replace this.
var newDynClient = func(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

// inClusterConfig attempts to load in-cluster K8s config. Tests replace this.
var inClusterConfig = rest.InClusterConfig

func Init() error {
	config, err := getConfig()
	if err != nil {
		return fmt.Errorf("k8s config: %w", err)
	}

	clientset, err := newClientset(config)
	if err != nil {
		return fmt.Errorf("k8s client: %w", err)
	}
	Client = clientset

	dynClient, err := newDynClient(config)
	if err != nil {
		return fmt.Errorf("k8s dynamic client: %w", err)
	}
	DynClient = dynClient

	return nil
}

func getConfig() (*rest.Config, error) {
	// In-cluster config (when running as a pod)
	config, err := inClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fallback: KUBECONFIG env or default path
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
