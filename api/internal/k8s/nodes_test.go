package k8s

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/asymmetric-effort/convocate/internal/types"
)

func setupFakeClient() *fake.Clientset {
	cs := fake.NewSimpleClientset()
	Client = cs
	return cs
}

func makeNode(name string, ready bool, unschedulable bool, labels map[string]string) *corev1.Node {
	condStatus := corev1.ConditionTrue
	if !ready {
		condStatus = corev1.ConditionFalse
	}
	if labels == nil {
		labels = map[string]string{}
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1.NodeSpec{
			Unschedulable: unschedulable,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: condStatus},
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "192.168.1.10"},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: "v1.31.0",
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceMemory:           resource.MustParse("16Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				corev1.ResourceCPU:              resource.MustParse("4"),
				corev1.ResourcePods:             resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory:           resource.MustParse("15Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("90Gi"),
				corev1.ResourceCPU:              resource.MustParse("3800m"),
				corev1.ResourcePods:             resource.MustParse("110"),
			},
		},
	}
}

func TestK8sNodeToNode_Ready(t *testing.T) {
	n := makeNode("node1", true, false, map[string]string{
		"convocate.io/location": "us-east-1",
	})
	result := k8sNodeToNode(n, nil)

	if result.ID != "node1" {
		t.Fatalf("expected ID node1, got %s", result.ID)
	}
	if result.Status != types.NodeReady {
		t.Fatalf("expected Ready status, got %s", result.Status)
	}
	if result.Location != "us-east-1" {
		t.Fatalf("expected location us-east-1, got %s", result.Location)
	}
	if result.IP != "192.168.1.10" {
		t.Fatalf("expected IP 192.168.1.10, got %s", result.IP)
	}
	if result.KubeletVersion != "v1.31.0" {
		t.Fatalf("expected kubelet v1.31.0, got %s", result.KubeletVersion)
	}
	if result.LoadAvg.One != -1 {
		t.Fatalf("expected loadAvg.One -1 without metrics, got %f", result.LoadAvg.One)
	}
	if result.MemUsedGB != -1 {
		t.Fatalf("expected memUsedGB -1 without metrics, got %f", result.MemUsedGB)
	}
}

func TestK8sNodeToNode_NotReady(t *testing.T) {
	n := makeNode("node2", false, false, nil)
	result := k8sNodeToNode(n, nil)
	if result.Status != types.NodeNotReady {
		t.Fatalf("expected NotReady status, got %s", result.Status)
	}
}

func TestK8sNodeToNode_Unschedulable(t *testing.T) {
	n := makeNode("node3", true, true, nil)
	result := k8sNodeToNode(n, nil)
	if result.Status != types.NodeSchedulingDisabled {
		t.Fatalf("expected SchedulingDisabled status, got %s", result.Status)
	}
}

func TestK8sNodeToNode_NoLocationLabel(t *testing.T) {
	n := makeNode("node4", true, false, nil)
	result := k8sNodeToNode(n, nil)
	if result.Location != "unspecified" {
		t.Fatalf("expected unspecified location, got %s", result.Location)
	}
}

func TestK8sNodeToNode_WithMetrics(t *testing.T) {
	n := makeNode("node5", true, false, nil)
	metrics := map[string]metricsUsage{
		"node5": {CPUCores: 2.5, MemBytes: 8 * 1024 * 1024 * 1024},
	}
	result := k8sNodeToNode(n, metrics)
	if result.LoadAvg.One != 2.5 {
		t.Fatalf("expected loadAvg.One 2.5, got %f", result.LoadAvg.One)
	}
	if result.MemUsedGB < 7.9 || result.MemUsedGB > 8.1 {
		t.Fatalf("expected memUsedGB ~8.0, got %f", result.MemUsedGB)
	}
}

func TestK8sNodeToNode_MetricsNoMatch(t *testing.T) {
	n := makeNode("node6", true, false, nil)
	metrics := map[string]metricsUsage{
		"other-node": {CPUCores: 1.0, MemBytes: 1024},
	}
	result := k8sNodeToNode(n, metrics)
	if result.LoadAvg.One != -1 {
		t.Fatalf("expected loadAvg.One -1 when no matching metrics, got %f", result.LoadAvg.One)
	}
}

func TestK8sNodeToNode_Tags(t *testing.T) {
	n := makeNode("node7", true, false, map[string]string{
		"env": "prod",
		"az":  "zone-a",
	})
	result := k8sNodeToNode(n, nil)
	if len(result.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(result.Tags))
	}
}

func TestK8sNodeToNode_DiskUsedGB(t *testing.T) {
	n := makeNode("node8", true, false, nil)
	result := k8sNodeToNode(n, nil)
	if result.DiskUsedGB < 9.9 || result.DiskUsedGB > 10.1 {
		t.Fatalf("expected diskUsedGB ~10.0, got %f", result.DiskUsedGB)
	}
	if result.DiskTotalGB < 99.9 || result.DiskTotalGB > 100.1 {
		t.Fatalf("expected diskTotalGB ~100.0, got %f", result.DiskTotalGB)
	}
}

func TestListNodes(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n1 := makeNode("node-a", true, false, nil)
	n2 := makeNode("node-b", false, false, nil)
	cs.CoreV1().Nodes().Create(ctx, n1, metav1.CreateOptions{})
	cs.CoreV1().Nodes().Create(ctx, n2, metav1.CreateOptions{})

	nodes, err := ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestGetNode(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("test-node", true, false, nil)
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	node, err := GetNode(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.ID != "test-node" {
		t.Fatalf("expected ID test-node, got %s", node.ID)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()

	_, err := GetNode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestGetNodeDetail(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("detail-node", true, false, map[string]string{
		"env": "test",
	})
	n.Spec.Taints = []corev1.Taint{
		{Key: "dedicated", Value: "agents", Effect: corev1.TaintEffectNoSchedule},
	}
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	detail, err := GetNodeDetail(ctx, "detail-node")
	if err != nil {
		t.Fatalf("GetNodeDetail: %v", err)
	}
	if detail.Node.ID != "detail-node" {
		t.Fatalf("expected ID detail-node, got %s", detail.Node.ID)
	}
	if len(detail.Labels) == 0 {
		t.Fatal("expected labels")
	}
	if detail.Labels["env"] != "test" {
		t.Fatalf("expected label env=test, got %s", detail.Labels["env"])
	}
	if len(detail.Taints) != 1 {
		t.Fatalf("expected 1 taint, got %d", len(detail.Taints))
	}
	if detail.Taints[0].Key != "dedicated" {
		t.Fatalf("expected taint key dedicated, got %s", detail.Taints[0].Key)
	}
	if len(detail.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(detail.Conditions))
	}
}

func TestGetNodeDetail_NotFound(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	_, err := GetNodeDetail(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestCordonNode(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("cordon-node", true, false, nil)
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	err := CordonNode(ctx, "cordon-node")
	if err != nil {
		t.Fatalf("CordonNode: %v", err)
	}

	updated, _ := cs.CoreV1().Nodes().Get(ctx, "cordon-node", metav1.GetOptions{})
	if !updated.Spec.Unschedulable {
		t.Fatal("expected node to be unschedulable after cordon")
	}
}

func TestUncordonNode(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("uncordon-node", true, true, nil)
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	err := UncordonNode(ctx, "uncordon-node")
	if err != nil {
		t.Fatalf("UncordonNode: %v", err)
	}

	updated, _ := cs.CoreV1().Nodes().Get(ctx, "uncordon-node", metav1.GetOptions{})
	if updated.Spec.Unschedulable {
		t.Fatal("expected node to be schedulable after uncordon")
	}
}

func TestUpdateNodeLabels(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("label-node", true, false, nil)
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	err := UpdateNodeLabels(ctx, "label-node", map[string]string{
		"env": "staging",
	})
	if err != nil {
		t.Fatalf("UpdateNodeLabels: %v", err)
	}

	updated, _ := cs.CoreV1().Nodes().Get(ctx, "label-node", metav1.GetOptions{})
	if updated.Labels["env"] != "staging" {
		t.Fatalf("expected label env=staging, got %s", updated.Labels["env"])
	}
}

func TestUpdateNodeLabels_NilLabels(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nil-label-node",
			Labels: nil,
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceMemory:           resource.MustParse("1Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory:           resource.MustParse("1Gi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	err := UpdateNodeLabels(ctx, "nil-label-node", map[string]string{"new": "label"})
	if err != nil {
		t.Fatalf("UpdateNodeLabels with nil labels: %v", err)
	}
}

func TestCountAgentPodsOnNode(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	for i := 0; i < 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-" + string(rune('a'+i)),
				Namespace: AgentNamespace,
			},
			Spec: corev1.PodSpec{
				NodeName:   "test-node",
				Containers: []corev1.Container{{Name: "agent", Image: "test"}},
			},
		}
		cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})
	}

	count, err := CountAgentPodsOnNode(ctx, "test-node")
	if err != nil {
		t.Fatalf("CountAgentPodsOnNode: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 pod, got %d", count)
	}
}

func TestListAgentPodsOnNode(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-test",
			Namespace: AgentNamespace,
			Labels: map[string]string{
				"convocate.io/project": "myproject",
				"convocate.io/owner":   "admin",
			},
		},
		Spec: corev1.PodSpec{
			NodeName:   "test-node",
			Containers: []corev1.Container{{Name: "agent", Image: "test"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	agents, err := ListAgentPodsOnNode(ctx, "test-node")
	if err != nil {
		t.Fatalf("ListAgentPodsOnNode: %v", err)
	}
	if len(agents) < 1 {
		t.Fatal("expected at least 1 agent")
	}

	a := agents[0]
	if a.ID != "agent-test" {
		t.Fatalf("expected agent ID agent-test, got %s", a.ID)
	}
	if a.Project != "myproject" {
		t.Fatalf("expected project myproject, got %s", a.Project)
	}
	if a.Status != types.AgentRunning {
		t.Fatalf("expected running status, got %s", a.Status)
	}
}

func TestPodToAgent_Phases(t *testing.T) {
	tests := []struct {
		phase  corev1.PodPhase
		expect types.AgentStatus
	}{
		{corev1.PodRunning, types.AgentRunning},
		{corev1.PodPending, types.AgentMigrating},
		{corev1.PodFailed, types.AgentStopped},
		{corev1.PodSucceeded, types.AgentStopped},
	}

	for _, tt := range tests {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-pod",
				Labels: map[string]string{},
			},
			Spec: corev1.PodSpec{NodeName: "node1"},
			Status: corev1.PodStatus{
				Phase: tt.phase,
			},
		}
		agent := podToAgent(pod)
		if agent.Status != tt.expect {
			t.Errorf("phase %s: expected status %s, got %s", tt.phase, tt.expect, agent.Status)
		}
	}
}

func TestParseQuantity(t *testing.T) {
	tests := []struct {
		input  string
		expect float64
	}{
		{"", 0},
		{"2", 2},
		{"500m", 0.5},
		{"250m", 0.25},
		{"1000m", 1.0},
		{"100n", 0.0000001},
	}
	for _, tt := range tests {
		got := parseQuantity(tt.input)
		if got < tt.expect-0.001 || got > tt.expect+0.001 {
			t.Errorf("parseQuantity(%q): expected %f, got %f", tt.input, tt.expect, got)
		}
	}
}

func TestParseQuantityBytes(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"", 0},
		{"1024", 1024},
		{"1024Ki", 1024 * 1024},
		{"512Mi", 512 * 1024 * 1024},
		{"2Gi", 2 * 1024 * 1024 * 1024},
	}
	for _, tt := range tests {
		got := parseQuantityBytes(tt.input)
		if got != tt.expect {
			t.Errorf("parseQuantityBytes(%q): expected %d, got %d", tt.input, tt.expect, got)
		}
	}
}

func TestFetchNodeMetrics_FakeClient(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	result := fetchNodeMetrics(ctx)
	if result != nil {
		t.Fatal("expected nil from fetchNodeMetrics with fake client")
	}
}

func TestCordonNode_NotFound(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	err := CordonNode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestUncordonNode_NotFound(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	err := UncordonNode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestUpdateNodeLabels_NotFound(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	err := UpdateNodeLabels(ctx, "nonexistent", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestCountAgentPodsOnNode_Empty(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	count, err := CountAgentPodsOnNode(ctx, "no-such-node")
	if err != nil {
		t.Fatalf("CountAgentPodsOnNode: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestListAgentPodsOnNode_Empty(t *testing.T) {
	setupFakeClient()
	ctx := context.Background()
	agents, err := ListAgentPodsOnNode(ctx, "no-such-node")
	if err != nil {
		t.Fatalf("ListAgentPodsOnNode: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

func TestK8sNodeToNode_ZeroDiskCapacity(t *testing.T) {
	// Node with zero capacity
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "zero-node"},
		Status: corev1.NodeStatus{
			Capacity:    corev1.ResourceList{},
			Allocatable: corev1.ResourceList{},
		},
	}
	result := k8sNodeToNode(n, nil)
	if result.DiskUsedGB != -1 {
		t.Fatalf("expected diskUsedGB -1 for zero capacity, got %f", result.DiskUsedGB)
	}
}

func TestK8sNodeToNode_MetricsZeroMem(t *testing.T) {
	n := makeNode("node-zeromem", true, false, nil)
	metrics := map[string]metricsUsage{
		"node-zeromem": {CPUCores: 1.0, MemBytes: 0},
	}
	result := k8sNodeToNode(n, metrics)
	if result.MemUsedGB != -1 {
		t.Fatalf("expected memUsedGB -1 for zero mem bytes, got %f", result.MemUsedGB)
	}
}

func TestPodToAgent_NilLabels(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-no-labels"},
		Spec:       corev1.PodSpec{NodeName: "node1"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	agent := podToAgent(pod)
	if agent.ID != "pod-no-labels" {
		t.Fatalf("expected ID pod-no-labels, got %s", agent.ID)
	}
	if agent.Project != "" {
		t.Fatalf("expected empty project, got %s", agent.Project)
	}
}

func TestListNodes_Error(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api server unavailable")
	})

	_, err := ListNodes(ctx)
	if err == nil {
		t.Fatal("expected error from ListNodes")
	}
}

func TestCountAgentPodsOnNode_Error(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api error")
	})

	_, err := CountAgentPodsOnNode(ctx, "test-node")
	if err == nil {
		t.Fatal("expected error from CountAgentPodsOnNode")
	}
}

func TestListAgentPodsOnNode_Error(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api error")
	})

	_, err := ListAgentPodsOnNode(ctx, "test-node")
	if err == nil {
		t.Fatal("expected error from ListAgentPodsOnNode")
	}
}

func TestUncordonNode_UpdateError(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("uncordon-err", true, true, nil)
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	cs.PrependReactor("update", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("update failed")
	})

	err := UncordonNode(ctx, "uncordon-err")
	if err == nil {
		t.Fatal("expected error from UncordonNode update")
	}
}

func TestUpdateNodeLabels_UpdateError(t *testing.T) {
	cs := setupFakeClient()
	ctx := context.Background()

	n := makeNode("label-err", true, false, nil)
	cs.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})

	cs.PrependReactor("update", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("update failed")
	})

	err := UpdateNodeLabels(ctx, "label-err", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error from UpdateNodeLabels update")
	}
}

func TestFetchNodeMetrics_Success(t *testing.T) {
	origFetcher := metricsRawFetcher
	defer func() { metricsRawFetcher = origFetcher }()

	metricsJSON := `{
		"items": [
			{
				"metadata": {"name": "node-1"},
				"usage": {"cpu": "500m", "memory": "2Gi"}
			},
			{
				"metadata": {"name": "node-2"},
				"usage": {"cpu": "2", "memory": "4096Mi"}
			}
		]
	}`

	metricsRawFetcher = func(ctx context.Context) ([]byte, error) {
		return []byte(metricsJSON), nil
	}

	result := fetchNodeMetrics(context.Background())
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes in metrics, got %d", len(result))
	}
	m1 := result["node-1"]
	if m1.CPUCores < 0.49 || m1.CPUCores > 0.51 {
		t.Fatalf("expected node-1 CPU ~0.5, got %f", m1.CPUCores)
	}
	m2 := result["node-2"]
	if m2.CPUCores < 1.99 || m2.CPUCores > 2.01 {
		t.Fatalf("expected node-2 CPU ~2.0, got %f", m2.CPUCores)
	}
}

func TestFetchNodeMetrics_FetchError(t *testing.T) {
	origFetcher := metricsRawFetcher
	defer func() { metricsRawFetcher = origFetcher }()

	metricsRawFetcher = func(ctx context.Context) ([]byte, error) {
		return nil, fmt.Errorf("metrics API unavailable")
	}

	result := fetchNodeMetrics(context.Background())
	if result != nil {
		t.Fatal("expected nil result on error")
	}
}

func TestFetchNodeMetrics_EmptyItems(t *testing.T) {
	origFetcher := metricsRawFetcher
	defer func() { metricsRawFetcher = origFetcher }()

	metricsRawFetcher = func(ctx context.Context) ([]byte, error) {
		return []byte(`{"items":[]}`), nil
	}

	result := fetchNodeMetrics(context.Background())
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}

func TestFetchNodeMetrics_InvalidJSON(t *testing.T) {
	origFetcher := metricsRawFetcher
	defer func() { metricsRawFetcher = origFetcher }()

	metricsRawFetcher = func(ctx context.Context) ([]byte, error) {
		return []byte("not valid json{{{"), nil
	}

	result := fetchNodeMetrics(context.Background())
	if result != nil {
		t.Fatal("expected nil result for invalid JSON")
	}
}
