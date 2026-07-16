package nmgr

import (
	"context"

	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/types"
)

// k8sNodeManager is the production implementation that delegates to the
// k8s package functions.
type k8sNodeManager struct{}

func (k8sNodeManager) ListNodes(ctx context.Context) ([]types.Node, error) {
	return k8s.ListNodes(ctx)
}
func (k8sNodeManager) GetNode(ctx context.Context, name string) (*types.Node, error) {
	return k8s.GetNode(ctx, name)
}
func (k8sNodeManager) GetNodeDetail(ctx context.Context, name string) (*types.NodeDetail, error) {
	return k8s.GetNodeDetail(ctx, name)
}
func (k8sNodeManager) CordonNode(ctx context.Context, name string) error {
	return k8s.CordonNode(ctx, name)
}
func (k8sNodeManager) UncordonNode(ctx context.Context, name string) error {
	return k8s.UncordonNode(ctx, name)
}
func (k8sNodeManager) CountAgentPodsOnNode(ctx context.Context, nodeName string) (int, error) {
	return k8s.CountAgentPodsOnNode(ctx, nodeName)
}
func (k8sNodeManager) ListAgentPodsOnNode(ctx context.Context, nodeName string) ([]types.Agent, error) {
	return k8s.ListAgentPodsOnNode(ctx, nodeName)
}
func (k8sNodeManager) DrainAndDeleteNode(ctx context.Context, nodeName string) error {
	return k8s.DrainAndDeleteNode(ctx, nodeName)
}
func (k8sNodeManager) ProvisionNode(ctx context.Context, req k8s.ProvisionRequest) error {
	return k8s.ProvisionNode(ctx, req)
}
