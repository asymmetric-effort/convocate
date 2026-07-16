package amgr

import (
	"context"
	"fmt"

	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/types"
)

// k8sAgentManager is the production implementation that delegates to the
// k8s package functions.
type k8sAgentManager struct{}

func (k8sAgentManager) ListAgentPods(ctx context.Context) ([]types.Agent, error) {
	return k8s.ListAgentPods(ctx)
}
func (k8sAgentManager) GetAgentPod(ctx context.Context, name string) (*types.Agent, error) {
	return k8s.GetAgentPod(ctx, name)
}
func (k8sAgentManager) CreateAgentPod(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
	return k8s.CreateAgentPod(ctx, req, owner)
}
func (k8sAgentManager) DeleteAgentPod(ctx context.Context, name string) error {
	return k8s.DeleteAgentPod(ctx, name)
}
func (k8sAgentManager) StopAgentPod(ctx context.Context, name string) error {
	return k8s.StopAgentPod(ctx, name)
}
func (k8sAgentManager) UpdateAgentConfigMap(ctx context.Context, podName, claudeMd string) error {
	return k8s.UpdateAgentConfigMap(ctx, podName, claudeMd)
}
func (k8sAgentManager) GetAgentPodIP(ctx context.Context, name string) (string, error) {
	pod, err := k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Get(ctx, name, k8s.GetOpts())
	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}
	if pod.Status.PodIP == "" {
		return "", fmt.Errorf("agent pod has no IP (not yet running)")
	}
	return pod.Status.PodIP, nil
}
