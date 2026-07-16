package amgr

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/types"
)

func setupFakeK8s(t *testing.T) {
	t.Helper()
	cs := fake.NewSimpleClientset()
	k8s.Client = cs

	ctx := context.Background()
	// Create the agent namespace
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: k8s.AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	t.Cleanup(func() { k8s.Client = nil })
}

func TestK8sAgentManager_ListAgentPods(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	// Create a pod in the agent namespace
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-test",
			Namespace: k8s.AgentNamespace,
			Labels:    map[string]string{"convocate.io/type": "agent"},
		},
	}
	k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	agents, err := mgr.ListAgentPods(ctx)
	if err != nil {
		t.Fatalf("ListAgentPods: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
}

func TestK8sAgentManager_GetAgentPod(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-test",
			Namespace: k8s.AgentNamespace,
			Labels:    map[string]string{"convocate.io/type": "agent"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	agent, err := mgr.GetAgentPod(ctx, "agent-test")
	if err != nil {
		t.Fatalf("GetAgentPod: %v", err)
	}
	if agent.ID != "agent-test" {
		t.Errorf("expected ID 'agent-test', got %q", agent.ID)
	}
}

func TestK8sAgentManager_GetAgentPod_NotFound(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	_, err := mgr.GetAgentPod(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent pod")
	}
}

func TestK8sAgentManager_CreateAgentPod(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	agent, err := mgr.CreateAgentPod(ctx, types.CreateAgentRequest{
		Project: "test-proj",
	}, "admin")
	if err != nil {
		t.Fatalf("CreateAgentPod: %v", err)
	}
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
}

func TestK8sAgentManager_DeleteAgentPod(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	// Create a pod first
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-del",
			Namespace: k8s.AgentNamespace,
		},
	}
	k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	err := mgr.DeleteAgentPod(ctx, "agent-del")
	if err != nil {
		t.Fatalf("DeleteAgentPod: %v", err)
	}
}

func TestK8sAgentManager_StopAgentPod(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-stop",
			Namespace: k8s.AgentNamespace,
		},
	}
	k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	err := mgr.StopAgentPod(ctx, "agent-stop")
	if err != nil {
		t.Fatalf("StopAgentPod: %v", err)
	}
}

func TestK8sAgentManager_UpdateAgentConfigMap(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	// Create a configmap first
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-agent-test",
			Namespace: k8s.AgentNamespace,
		},
		Data: map[string]string{"CLAUDE.md": "old content"},
	}
	k8s.Client.CoreV1().ConfigMaps(k8s.AgentNamespace).Create(ctx, cm, metav1.CreateOptions{})

	err := mgr.UpdateAgentConfigMap(ctx, "agent-test", "new content")
	if err != nil {
		t.Fatalf("UpdateAgentConfigMap: %v", err)
	}
}

func TestK8sAgentManager_GetAgentPodIP(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-ip",
			Namespace: k8s.AgentNamespace,
		},
		Status: corev1.PodStatus{PodIP: "10.0.0.5"},
	}
	k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	ip, err := mgr.GetAgentPodIP(ctx, "agent-ip")
	if err != nil {
		t.Fatalf("GetAgentPodIP: %v", err)
	}
	if ip != "10.0.0.5" {
		t.Errorf("expected IP '10.0.0.5', got %q", ip)
	}
}

func TestK8sAgentManager_GetAgentPodIP_NotFound(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	_, err := mgr.GetAgentPodIP(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent pod")
	}
}

func TestK8sAgentManager_GetAgentPodIP_NoIP(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sAgentManager{}
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-noip",
			Namespace: k8s.AgentNamespace,
		},
		Status: corev1.PodStatus{PodIP: ""},
	}
	k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	_, err := mgr.GetAgentPodIP(ctx, "agent-noip")
	if err == nil {
		t.Fatal("expected error for pod with no IP")
	}
}
