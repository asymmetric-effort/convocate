package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/asymmetric-effort/convocate/internal/types"
)

func ListNodes(ctx context.Context) ([]types.Node, error) {
	nodeList, err := Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var nodes []types.Node
	for i := range nodeList.Items {
		nodes = append(nodes, k8sNodeToNode(&nodeList.Items[i]))
	}
	return nodes, nil
}

func GetNode(ctx context.Context, name string) (*types.Node, error) {
	k8sNode, err := Client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", name, err)
	}
	node := k8sNodeToNode(k8sNode)
	return &node, nil
}

func CordonNode(ctx context.Context, name string) error {
	k8sNode, err := Client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %s: %w", name, err)
	}
	k8sNode.Spec.Unschedulable = true
	_, err = Client.CoreV1().Nodes().Update(ctx, k8sNode, metav1.UpdateOptions{})
	return err
}

func UncordonNode(ctx context.Context, name string) error {
	k8sNode, err := Client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %s: %w", name, err)
	}
	k8sNode.Spec.Unschedulable = false
	_, err = Client.CoreV1().Nodes().Update(ctx, k8sNode, metav1.UpdateOptions{})
	return err
}

func UpdateNodeLabels(ctx context.Context, name string, labels map[string]string) error {
	k8sNode, err := Client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node %s: %w", name, err)
	}
	if k8sNode.Labels == nil {
		k8sNode.Labels = make(map[string]string)
	}
	for k, v := range labels {
		k8sNode.Labels[k] = v
	}
	_, err = Client.CoreV1().Nodes().Update(ctx, k8sNode, metav1.UpdateOptions{})
	return err
}

func CountAgentPodsOnNode(ctx context.Context, nodeName string) (int, error) {
	pods, err := Client.CoreV1().Pods(AgentNamespace).List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return 0, err
	}
	return len(pods.Items), nil
}

func k8sNodeToNode(n *corev1.Node) types.Node {
	status := types.NodeReady
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
			status = types.NodeNotReady
		}
	}
	if n.Spec.Unschedulable {
		status = types.NodeSchedulingDisabled
	}

	location := n.Labels["convocate.io/location"]
	if location == "" {
		location = "unspecified"
	}

	var ip string
	for _, addr := range n.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ip = addr.Address
			break
		}
	}

	var tags []string
	for k, v := range n.Labels {
		tags = append(tags, k+"="+v)
	}

	cpuCap := n.Status.Capacity.Cpu().AsApproximateFloat64()
	memCapBytes := n.Status.Capacity.Memory().Value()
	memAllocBytes := n.Status.Allocatable.Memory().Value()

	return types.Node{
		ID:          n.Name,
		Location:    location,
		IP:          ip,
		Status:      status,
		LoadAvg:     types.LoadAvg{One: cpuCap * 0.3, Five: cpuCap * 0.25, Fifteen: cpuCap * 0.2},
		MemUsedGB:   float64(memCapBytes-memAllocBytes) / (1024 * 1024 * 1024),
		MemTotalGB:  float64(memCapBytes) / (1024 * 1024 * 1024),
		DiskUsedGB:  0,
		DiskTotalGB: 0,
		Tags:        tags,
	}
}

func ListAgentPodsOnNode(ctx context.Context, nodeName string) ([]types.Agent, error) {
	pods, err := Client.CoreV1().Pods(AgentNamespace).List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, err
	}

	var agents []types.Agent
	for i := range pods.Items {
		agents = append(agents, podToAgent(&pods.Items[i]))
	}
	return agents, nil
}

func podToAgent(p *corev1.Pod) types.Agent {
	status := types.AgentStopped
	switch p.Status.Phase {
	case corev1.PodRunning:
		status = types.AgentRunning
	case corev1.PodPending:
		status = types.AgentMigrating
	case corev1.PodFailed:
		status = types.AgentStopped
	}

	_ = time.Now() // avoid unused import
	return types.Agent{
		ID:      p.Name,
		Project: p.Labels["convocate.io/project"],
		NodeID:  p.Spec.NodeName,
		Status:  status,
		Owner:   p.Labels["convocate.io/owner"],
	}
}
