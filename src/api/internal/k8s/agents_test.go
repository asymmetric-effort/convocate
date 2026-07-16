package k8s

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/asymmetric-effort/convocate/internal/types"
)

func TestAgentNamespaceValue(t *testing.T) {
	if AgentNamespace != "convocate-agents" {
		t.Fatalf("expected namespace convocate-agents, got %s", AgentNamespace)
	}
}

func TestEnsureAgentNamespace_AlreadyExists(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := EnsureAgentNamespace(ctx)
	if err != nil {
		t.Fatalf("EnsureAgentNamespace (exists): %v", err)
	}
}

func TestEnsureAgentNamespace_Creates(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	err := EnsureAgentNamespace(ctx)
	if err != nil {
		t.Fatalf("EnsureAgentNamespace (create): %v", err)
	}

	_, err = cs.CoreV1().Namespaces().Get(ctx, AgentNamespace, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("namespace not created: %v", err)
	}
}

func TestListAgentPods(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-proj1",
			Namespace: AgentNamespace,
			Labels: map[string]string{
				"convocate.io/type":    "agent",
				"convocate.io/project": "proj1",
				"convocate.io/owner":   "testuser",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "agent", Image: "test"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	agents, err := ListAgentPods(ctx)
	if err != nil {
		t.Fatalf("ListAgentPods: %v", err)
	}
	if len(agents) < 1 {
		t.Fatal("expected at least 1 agent")
	}
}

func TestGetAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-proj2",
			Namespace: AgentNamespace,
			Labels:    map[string]string{"convocate.io/project": "proj2"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "agent", Image: "test"}},
		},
	}
	cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	agent, err := GetAgentPod(ctx, "agent-proj2")
	if err != nil {
		t.Fatalf("GetAgentPod: %v", err)
	}
	if agent.ID != "agent-proj2" {
		t.Fatalf("expected ID agent-proj2, got %s", agent.ID)
	}
}

func TestGetAgentPod_NotFound(t *testing.T) {
	Client = fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := GetAgentPod(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent pod")
	}
}

func TestCreateAgentPVC(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentPVC(ctx, "test-pod", "5Gi")
	if err != nil {
		t.Fatalf("CreateAgentPVC: %v", err)
	}

	pvc, err := cs.CoreV1().PersistentVolumeClaims(AgentNamespace).Get(ctx, "pvc-test-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("PVC not created: %v", err)
	}
	if pvc.Labels["convocate.io/type"] != "agent-pvc" {
		t.Fatal("expected agent-pvc label")
	}
}

func TestCreateAgentPVC_DefaultStorage(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentPVC(ctx, "test-pod-def", "")
	if err != nil {
		t.Fatalf("CreateAgentPVC with default: %v", err)
	}
}

func TestCreateAgentConfigMap(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentConfigMap(ctx, "test-pod", "# Custom CLAUDE.md")
	if err != nil {
		t.Fatalf("CreateAgentConfigMap: %v", err)
	}

	cm, err := cs.CoreV1().ConfigMaps(AgentNamespace).Get(ctx, "cm-test-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}
	if cm.Data["CLAUDE.md"] != "# Custom CLAUDE.md" {
		t.Fatal("expected custom CLAUDE.md content")
	}
}

func TestCreateAgentConfigMap_Default(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentConfigMap(ctx, "test-pod-def", "")
	if err != nil {
		t.Fatalf("CreateAgentConfigMap with default: %v", err)
	}

	cm, _ := cs.CoreV1().ConfigMaps(AgentNamespace).Get(ctx, "cm-test-pod-def", metav1.GetOptions{})
	if cm.Data["CLAUDE.md"] != defaultClaudeMd {
		t.Fatal("expected default CLAUDE.md content")
	}
}

func TestUpdateAgentConfigMap(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	CreateAgentConfigMap(ctx, "test-pod", "original")

	err := UpdateAgentConfigMap(ctx, "test-pod", "updated content")
	if err != nil {
		t.Fatalf("UpdateAgentConfigMap: %v", err)
	}

	cm, _ := cs.CoreV1().ConfigMaps(AgentNamespace).Get(ctx, "cm-test-pod", metav1.GetOptions{})
	if cm.Data["CLAUDE.md"] != "updated content" {
		t.Fatal("expected updated content")
	}
}

func TestUpdateAgentConfigMap_NotFound(t *testing.T) {
	Client = fake.NewSimpleClientset()
	ctx := context.Background()

	err := UpdateAgentConfigMap(ctx, "nonexistent", "content")
	if err == nil {
		t.Fatal("expected error for nonexistent configmap")
	}
}

func TestCreateAgentSecret(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentSecret(ctx, "test-pod", "sk-test-key")
	if err != nil {
		t.Fatalf("CreateAgentSecret: %v", err)
	}

	secret, err := cs.CoreV1().Secrets(AgentNamespace).Get(ctx, "secret-test-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Secret not created: %v", err)
	}
	if secret.Labels["convocate.io/type"] != "agent-secret" {
		t.Fatal("expected agent-secret label")
	}
}

func TestCreateAgentSecret_EmptyKey(t *testing.T) {
	Client = fake.NewSimpleClientset()
	ctx := context.Background()

	err := CreateAgentSecret(ctx, "test-pod", "")
	if err != nil {
		t.Fatal("expected no error for empty API key")
	}
}

func TestCreateAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{
		Project:     "myproject",
		NodeID:      "worker-1",
		ClaudeFlags: []string{"--model", "opus"},
		Logging:     true,
	}

	agent, err := CreateAgentPod(ctx, req, "admin")
	if err != nil {
		t.Fatalf("CreateAgentPod: %v", err)
	}
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
	if agent.Project != "myproject" {
		t.Fatalf("expected project myproject, got %s", agent.Project)
	}

	pod, err := cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-myproject", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("pod not created: %v", err)
	}
	if pod.Spec.NodeSelector["kubernetes.io/hostname"] != "worker-1" {
		t.Fatal("expected node selector for worker-1")
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Fatal("expected RestartPolicyNever")
	}
}

func TestCreateAgentPod_DefaultImage(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{Project: "proj-default"}
	agent, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod: %v", err)
	}

	pod, _ := cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-proj-default", metav1.GetOptions{})
	if pod.Spec.Containers[0].Image != defaultAgentImage {
		t.Fatalf("expected default image %s, got %s", defaultAgentImage, pod.Spec.Containers[0].Image)
	}
	if agent.Owner != "user1" {
		t.Fatalf("expected owner user1, got %s", agent.Owner)
	}
}

func TestCreateAgentPod_CustomImage(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{
		Project: "proj-custom",
		Image:   "custom-image:v1",
	}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod: %v", err)
	}

	pod, _ := cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-proj-custom", metav1.GetOptions{})
	if pod.Spec.Containers[0].Image != "custom-image:v1" {
		t.Fatalf("expected custom-image:v1, got %s", pod.Spec.Containers[0].Image)
	}
}

func TestCreateAgentPod_WithResources(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{
		Project: "proj-res",
		Resources: &types.AgentResources{
			CPURequest:    "1",
			CPULimit:      "4",
			MemoryRequest: "1Gi",
			MemoryLimit:   "4Gi",
			StorageSize:   "10Gi",
		},
	}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod with resources: %v", err)
	}
}

func TestCreateAgentPod_WithSecurity(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{
		Project: "proj-sec",
		Security: &types.AgentSecurity{
			Capabilities: []string{"NET_ADMIN"},
			DockerAccess: true,
			AdditionalMounts: []types.AgentMount{
				{HostPath: "/data", MountPath: "/mnt/data", ReadOnly: true},
			},
		},
	}
	_, err := CreateAgentPod(ctx, req, "admin")
	if err != nil {
		t.Fatalf("CreateAgentPod with security: %v", err)
	}

	pod, _ := cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-proj-sec", metav1.GetOptions{})
	foundDocker := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "docker-sock" {
			foundDocker = true
		}
	}
	if !foundDocker {
		t.Fatal("expected docker-sock volume with DockerAccess=true")
	}
}

func TestCreateAgentPod_WithAPIKey(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{
		Project:         "proj-key",
		AnthropicApiKey: "sk-test-12345",
	}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod with API key: %v", err)
	}

	pod, _ := cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-proj-key", metav1.GetOptions{})
	foundKeyEnv := false
	for _, e := range pod.Spec.Containers[0].Env {
		if e.Name == "ANTHROPIC_API_KEY" {
			foundKeyEnv = true
		}
	}
	if !foundKeyEnv {
		t.Fatal("expected ANTHROPIC_API_KEY env var when key provided")
	}
}

func TestCreateAgentPod_NoNodeSelector(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{Project: "proj-nonode"}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod: %v", err)
	}

	pod, _ := cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-proj-nonode", metav1.GetOptions{})
	if pod.Spec.NodeSelector != nil {
		t.Fatal("expected no node selector when NodeID is empty")
	}
}

func TestDeleteAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-del", Namespace: AgentNamespace},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "agent", Image: "test"}}},
	}
	cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	err := DeleteAgentPod(ctx, "agent-del")
	if err != nil {
		t.Fatalf("DeleteAgentPod: %v", err)
	}

	_, err = cs.CoreV1().Pods(AgentNamespace).Get(ctx, "agent-del", metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected pod to be deleted")
	}
}

func TestStopAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-stop", Namespace: AgentNamespace},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "agent", Image: "test"}}},
	}
	cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	err := StopAgentPod(ctx, "agent-stop")
	if err != nil {
		t.Fatalf("StopAgentPod: %v", err)
	}
}

func TestGetOpts(t *testing.T) {
	opts := GetOpts()
	if opts.ResourceVersion != "" {
		t.Fatal("expected empty resource version in default GetOptions")
	}
}

func TestValueOr(t *testing.T) {
	v := valueOr(nil, func(r *types.AgentResources) string { return r.CPURequest }, "default")
	if v != "default" {
		t.Fatalf("expected default, got %s", v)
	}

	r := &types.AgentResources{}
	v = valueOr(r, func(r *types.AgentResources) string { return r.CPURequest }, "default")
	if v != "default" {
		t.Fatalf("expected default for empty field, got %s", v)
	}

	r = &types.AgentResources{CPURequest: "1"}
	v = valueOr(r, func(r *types.AgentResources) string { return r.CPURequest }, "default")
	if v != "1" {
		t.Fatalf("expected 1, got %s", v)
	}
}

func TestBoolPtr(t *testing.T) {
	p := boolPtr(true)
	if p == nil || !*p {
		t.Fatal("expected *true")
	}
	p = boolPtr(false)
	if p == nil || *p {
		t.Fatal("expected *false")
	}
}

func TestIntstr8443(t *testing.T) {
	v := intstr8443()
	if v.IntValue() != 8443 {
		t.Fatalf("expected 8443, got %d", v.IntValue())
	}
}

func TestCreateAgentCertificate_NilDynClient(t *testing.T) {
	DynClient = nil
	ctx := context.Background()

	err := CreateAgentCertificate(ctx, "test-pod")
	if err != nil {
		t.Fatalf("expected nil error with nil DynClient, got %v", err)
	}
}

func TestListAgentPods_Error(t *testing.T) {
	// Use a clientset where the namespace doesn't exist — fake client
	// doesn't actually error on missing namespace for List, so we test
	// that we get an empty list gracefully.
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	agents, err := ListAgentPods(ctx)
	if err != nil {
		t.Fatalf("ListAgentPods: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

func TestDeleteAgentPod_WithDynClient(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	// Set DynClient to a fake dynamic client
	DynClient = fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	defer func() { DynClient = nil }()
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-dyn", Namespace: AgentNamespace},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "agent", Image: "test"}}},
	}
	cs.CoreV1().Pods(AgentNamespace).Create(ctx, pod, metav1.CreateOptions{})

	// DeleteAgentPod logs warnings for missing associated resources but still succeeds
	err := DeleteAgentPod(ctx, "agent-dyn")
	if err != nil {
		t.Fatalf("DeleteAgentPod: %v", err)
	}
}

func TestDeleteAgentPod_NotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	// Deleting a nonexistent pod — err is returned from the pod delete
	err := DeleteAgentPod(ctx, "nonexistent-pod")
	// The function returns the pod delete error even though it continues cleanup
	if err == nil {
		t.Fatal("expected error for nonexistent pod")
	}
}

func TestCreateAgentPod_ClaudeMdDefault(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{
		Project: "proj-claudemd",
		// ClaudeMd empty — should use default
	}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod: %v", err)
	}

	cm, _ := cs.CoreV1().ConfigMaps(AgentNamespace).Get(ctx, "cm-agent-proj-claudemd", metav1.GetOptions{})
	if cm.Data["CLAUDE.md"] != defaultClaudeMd {
		t.Fatal("expected default CLAUDE.md content")
	}
}

func TestCreateAgentCertificate_WithDynClient(t *testing.T) {
	// Test with a fake dynamic client
	DynClient = fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	defer func() { DynClient = nil }()
	ctx := context.Background()

	// This will attempt to create a Certificate resource
	// The fake dynamic client won't error on creation
	err := CreateAgentCertificate(ctx, "test-cert-pod")
	if err != nil {
		t.Fatalf("CreateAgentCertificate with DynClient: %v", err)
	}
}

func TestCreateAgentPVC_AlreadyExists(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	// Create twice — second should not error
	err := CreateAgentPVC(ctx, "dup-pod", "1Gi")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	err = CreateAgentPVC(ctx, "dup-pod", "1Gi")
	if err != nil {
		t.Fatalf("second create should not error: %v", err)
	}
}

func TestCreateAgentConfigMap_AlreadyExists(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentConfigMap(ctx, "dup-pod", "content")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	err = CreateAgentConfigMap(ctx, "dup-pod", "content2")
	if err != nil {
		t.Fatalf("second create should not error: %v", err)
	}
}

func TestCreateAgentSecret_AlreadyExists(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	err := CreateAgentSecret(ctx, "dup-pod", "key1")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	err = CreateAgentSecret(ctx, "dup-pod", "key2")
	if err != nil {
		t.Fatalf("second create should not error: %v", err)
	}
}

func TestListAgentPods_ListError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api server unavailable")
	})

	_, err := ListAgentPods(ctx)
	if err == nil {
		t.Fatal("expected error from ListAgentPods")
	}
}

func TestCreateAgentPVC_NonExistsError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("storage class not found")
	})

	err := CreateAgentPVC(ctx, "test-pod", "5Gi")
	if err == nil {
		t.Fatal("expected error from CreateAgentPVC")
	}
}

func TestCreateAgentConfigMap_NonExistsError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("forbidden")
	})

	err := CreateAgentConfigMap(ctx, "test-pod", "content")
	if err == nil {
		t.Fatal("expected error from CreateAgentConfigMap")
	}
}

func TestCreateAgentSecret_NonExistsError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("quota exceeded")
	})

	err := CreateAgentSecret(ctx, "test-pod", "key1")
	if err == nil {
		t.Fatal("expected error from CreateAgentSecret")
	}
}

func TestCreateAgentPod_PVCError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	cs.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("storage error")
	})

	req := types.CreateAgentRequest{Project: "proj-fail-pvc"}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err == nil {
		t.Fatal("expected error from CreateAgentPod when PVC fails")
	}
}

func TestCreateAgentPod_ConfigMapError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	cs.PrependReactor("create", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("configmap error")
	})

	req := types.CreateAgentRequest{Project: "proj-fail-cm"}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err == nil {
		t.Fatal("expected error from CreateAgentPod when ConfigMap fails")
	}
}

func TestCreateAgentPod_SecretError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	cs.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("secret error")
	})

	req := types.CreateAgentRequest{Project: "proj-fail-secret", AnthropicApiKey: "sk-test"}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err == nil {
		t.Fatal("expected error from CreateAgentPod when Secret fails")
	}
}

func TestCreateAgentPod_PodCreateError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	DynClient = nil
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("pod quota exceeded")
	})

	req := types.CreateAgentRequest{Project: "proj-fail-pod"}
	_, err := CreateAgentPod(ctx, req, "user1")
	if err == nil {
		t.Fatal("expected error from CreateAgentPod when pod creation fails")
	}
}

func TestCreateAgentCertificate_AlreadyExists(t *testing.T) {
	scheme := runtime.NewScheme()
	DynClient = fakedynamic.NewSimpleDynamicClient(scheme)
	defer func() { DynClient = nil }()
	ctx := context.Background()

	// First create should succeed
	err := CreateAgentCertificate(ctx, "cert-dup")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Second create should succeed (already exists is tolerated)
	err = CreateAgentCertificate(ctx, "cert-dup")
	if err != nil {
		t.Fatalf("second create should not error: %v", err)
	}
}

func TestCreateAgentCertificate_NonExistsError(t *testing.T) {
	scheme := runtime.NewScheme()
	dc := fakedynamic.NewSimpleDynamicClient(scheme)
	DynClient = dc
	defer func() { DynClient = nil }()
	ctx := context.Background()

	dc.PrependReactor("create", "certificates", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cert-manager not installed")
	})

	err := CreateAgentCertificate(ctx, "cert-err")
	if err == nil {
		t.Fatal("expected error from CreateAgentCertificate")
	}
}

func TestCreateAgentPod_CertificateFailsNonFatal(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Set up DynClient that fails on certificate creation (non "already exists" error)
	scheme := runtime.NewScheme()
	dc := fakedynamic.NewSimpleDynamicClient(scheme)
	DynClient = dc
	defer func() { DynClient = nil }()

	dc.PrependReactor("create", "certificates", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cert-manager CRD not installed")
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	req := types.CreateAgentRequest{Project: "proj-certfail"}
	agent, err := CreateAgentPod(ctx, req, "user1")
	if err != nil {
		t.Fatalf("CreateAgentPod should succeed even if cert fails: %v", err)
	}
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
}
