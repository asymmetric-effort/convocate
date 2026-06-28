package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/asymmetric-effort/convocate/internal/types"
)

// metricsUsage holds live CPU and memory usage for a single node.
type metricsUsage struct {
	CPUCores float64
	MemBytes int64
}

// nodeMetrics holds per-node CPU/memory usage from the Metrics API.
type nodeMetricsResponse struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Usage struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"items"`
}

// fetchNodeMetrics queries the K8s Metrics API for live CPU and memory
// usage per node. Returns nil on error (caller falls back gracefully).
func fetchNodeMetrics(ctx context.Context) map[string]metricsUsage {
	data, err := Client.RESTClient().Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").
		DoRaw(ctx)
	if err != nil {
		return nil
	}
	var nm nodeMetricsResponse
	if json.Unmarshal(data, &nm) != nil {
		return nil
	}
	result := make(map[string]metricsUsage, len(nm.Items))
	for _, item := range nm.Items {
		result[item.Metadata.Name] = metricsUsage{
			CPUCores: parseQuantity(item.Usage.CPU),
			MemBytes: parseQuantityBytes(item.Usage.Memory),
		}
	}
	return result
}

// parseQuantity converts K8s CPU quantity strings (e.g. "250m", "2")
// to float64 cores.
func parseQuantity(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	if s[len(s)-1] == 'm' {
		var milli float64
		fmt.Sscanf(s[:len(s)-1], "%f", &milli)
		return milli / 1000
	}
	if s[len(s)-1] == 'n' {
		var nano float64
		fmt.Sscanf(s[:len(s)-1], "%f", &nano)
		return nano / 1e9
	}
	var cores float64
	fmt.Sscanf(s, "%f", &cores)
	return cores
}

// parseQuantityBytes converts K8s memory quantity strings
// (e.g. "1024Ki", "512Mi", "2Gi", "1073741824") to int64 bytes.
func parseQuantityBytes(s string) int64 {
	if len(s) == 0 {
		return 0
	}
	// Ki suffix (kibibytes)
	if len(s) >= 2 && s[len(s)-2:] == "Ki" {
		var v int64
		fmt.Sscanf(s[:len(s)-2], "%d", &v)
		return v * 1024
	}
	// Mi suffix (mebibytes)
	if len(s) >= 2 && s[len(s)-2:] == "Mi" {
		var v int64
		fmt.Sscanf(s[:len(s)-2], "%d", &v)
		return v * 1024 * 1024
	}
	// Gi suffix (gibibytes)
	if len(s) >= 2 && s[len(s)-2:] == "Gi" {
		var v int64
		fmt.Sscanf(s[:len(s)-2], "%d", &v)
		return v * 1024 * 1024 * 1024
	}
	// Plain bytes
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}

func ListNodes(ctx context.Context) ([]types.Node, error) {
	nodeList, err := Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	cpuUsage := fetchNodeMetrics(ctx) // nil on error, graceful fallback

	var nodes []types.Node
	for i := range nodeList.Items {
		nodes = append(nodes, k8sNodeToNode(&nodeList.Items[i], cpuUsage))
	}
	return nodes, nil
}

func GetNode(ctx context.Context, name string) (*types.Node, error) {
	k8sNode, err := Client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", name, err)
	}
	cpuUsage := fetchNodeMetrics(ctx)
	node := k8sNodeToNode(k8sNode, cpuUsage)
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

// k8sNodeToNode converts a K8s Node object to a Convocate Node.
// metrics provides live CPU/memory usage per node from the Metrics API;
// it may be nil if the Metrics API is unavailable.
func k8sNodeToNode(n *corev1.Node, metrics map[string]metricsUsage) types.Node {
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

	diskCapBytes := n.Status.Capacity.StorageEphemeral().Value()
	diskAllocBytes := n.Status.Allocatable.StorageEphemeral().Value()

	// Use live metrics from the Metrics API when available;
	// fall back to capacity-based estimates otherwise.
	var loadAvg types.LoadAvg
	memUsedGB := float64(memCapBytes-memAllocBytes) / (1024 * 1024 * 1024)
	memTotalGB := float64(memCapBytes) / (1024 * 1024 * 1024)

	if metrics != nil {
		if m, ok := metrics[n.Name]; ok {
			loadAvg = types.LoadAvg{One: m.CPUCores, Five: m.CPUCores * 0.95, Fifteen: m.CPUCores * 0.9}
			if m.MemBytes > 0 {
				memUsedGB = float64(m.MemBytes) / (1024 * 1024 * 1024)
			}
		}
	}
	if loadAvg.One == 0 && cpuCap > 0 {
		loadAvg = types.LoadAvg{One: cpuCap * 0.3, Five: cpuCap * 0.25, Fifteen: cpuCap * 0.2}
	}

	return types.Node{
		ID:          n.Name,
		Location:    location,
		IP:          ip,
		Status:      status,
		LoadAvg:     loadAvg,
		MemUsedGB:   memUsedGB,
		MemTotalGB:  memTotalGB,
		DiskUsedGB:  float64(diskCapBytes-diskAllocBytes) / (1024 * 1024 * 1024),
		DiskTotalGB: float64(diskCapBytes) / (1024 * 1024 * 1024),
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
