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

func Init() error {
	config, err := getConfig()
	if err != nil {
		return fmt.Errorf("k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("k8s client: %w", err)
	}
	Client = clientset

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("k8s dynamic client: %w", err)
	}
	DynClient = dynClient

	return nil
}

func getConfig() (*rest.Config, error) {
	// In-cluster config (when running as a pod)
	config, err := rest.InClusterConfig()
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
