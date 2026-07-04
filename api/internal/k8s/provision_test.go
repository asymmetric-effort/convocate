package k8s

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBootstrapTokenSecret(t *testing.T) {
	secret := bootstrapTokenSecret("abc123", "0123456789abcdef")

	if secret.Name != "bootstrap-token-abc123" {
		t.Fatalf("expected name bootstrap-token-abc123, got %s", secret.Name)
	}
	if secret.Namespace != "kube-system" {
		t.Fatalf("expected namespace kube-system, got %s", secret.Namespace)
	}
	if secret.Type != "bootstrap.kubernetes.io/token" {
		t.Fatalf("expected bootstrap token type, got %s", secret.Type)
	}
	if secret.StringData["token-id"] != "abc123" {
		t.Fatalf("expected token-id abc123, got %s", secret.StringData["token-id"])
	}
	if secret.StringData["token-secret"] != "0123456789abcdef" {
		t.Fatalf("expected token-secret 0123456789abcdef, got %s", secret.StringData["token-secret"])
	}
	if secret.StringData["usage-bootstrap-authentication"] != "true" {
		t.Fatal("expected usage-bootstrap-authentication true")
	}
	if secret.StringData["usage-bootstrap-signing"] != "true" {
		t.Fatal("expected usage-bootstrap-signing true")
	}
	if secret.StringData["expiration"] == "" {
		t.Fatal("expected non-empty expiration")
	}
}

func TestMinFunc(t *testing.T) {
	if min(1, 2) != 1 {
		t.Fatal("min(1,2) should be 1")
	}
	if min(5, 3) != 3 {
		t.Fatal("min(5,3) should be 3")
	}
	if min(4, 4) != 4 {
		t.Fatal("min(4,4) should be 4")
	}
	if min(0, 1) != 0 {
		t.Fatal("min(0,1) should be 0")
	}
	if min(-1, 0) != -1 {
		t.Fatal("min(-1,0) should be -1")
	}
}

func TestDrainAndDeleteNode(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Create a node
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-node"},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	// Create a pod on the node (not daemonset)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName:   "drain-node",
			Containers: []corev1.Container{{Name: "app", Image: "test"}},
		},
	}
	cs.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})

	// Create a daemonset pod (should be skipped)
	dsPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds-pod",
			Namespace: "kube-system",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "node-monitor"},
			},
		},
		Spec: corev1.PodSpec{
			NodeName:   "drain-node",
			Containers: []corev1.Container{{Name: "monitor", Image: "test"}},
		},
	}
	cs.CoreV1().Pods("kube-system").Create(ctx, dsPod, metav1.CreateOptions{})

	err := DrainAndDeleteNode(ctx, "drain-node")
	if err != nil {
		t.Fatalf("DrainAndDeleteNode: %v", err)
	}

	// Verify node is deleted
	_, err = cs.CoreV1().Nodes().Get(ctx, "drain-node", metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected node to be deleted")
	}
}

func TestDrainAndDeleteNode_NotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	err := DrainAndDeleteNode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestWaitForNodeByIP_Timeout(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Create a node but with wrong IP
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "wrong-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	_, err := waitForNodeByIP(ctx, "10.0.0.99", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestWaitForNodeByIP_NodeNotReady(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Create a node with matching IP but not Ready
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "not-ready-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.5"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "False"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	_, err := waitForNodeByIP(ctx, "10.0.0.5", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error for not-ready node")
	}
}

func TestWaitForNodeByIP_Found(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "ready-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.10"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	name, err := waitForNodeByIP(ctx, "10.0.0.10", 5*time.Second)
	if err != nil {
		t.Fatalf("waitForNodeByIP: %v", err)
	}
	if name != "ready-node" {
		t.Fatalf("expected ready-node, got %s", name)
	}
}

func TestGenerateJoinCredentials_NoClusterInfo(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// No cluster-info configmap exists
	_, _, err := generateJoinCredentials(ctx)
	if err == nil {
		t.Fatal("expected error when cluster-info doesn't exist")
	}
}

func TestGenerateJoinCredentials_EmptyKubeconfig(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Create cluster-info without kubeconfig
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{},
	}
	cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-public"},
	}, metav1.CreateOptions{})
	cs.CoreV1().ConfigMaps("kube-public").Create(ctx, cm, metav1.CreateOptions{})

	_, _, err := generateJoinCredentials(ctx)
	if err == nil {
		t.Fatal("expected error when kubeconfig is empty")
	}
}

func TestGenerateJoinCredentials_NoCACert(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-public"},
	}, metav1.CreateOptions{})

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{
			"kubeconfig": "apiVersion: v1\nclusters:\n- cluster:\n    server: https://10.0.0.1:6443\n",
		},
	}
	cs.CoreV1().ConfigMaps("kube-public").Create(ctx, cm, metav1.CreateOptions{})

	token, _, err := generateJoinCredentials(ctx)
	if err == nil {
		t.Fatal("expected error when no CA cert found")
	}
	// Token should still be created even if hash fails
	if token == "" {
		t.Fatal("expected non-empty token even on partial failure")
	}
}

func TestDrainAndDeleteNode_WithDeletingPods(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-node-2"},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	// Create a pod that's already being deleted
	now := metav1.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-pod",
			Namespace:         "default",
			DeletionTimestamp: &now,
		},
		Spec: corev1.PodSpec{
			NodeName:   "drain-node-2",
			Containers: []corev1.Container{{Name: "app", Image: "test"}},
		},
	}
	cs.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})

	err := DrainAndDeleteNode(ctx, "drain-node-2")
	if err != nil {
		t.Fatalf("DrainAndDeleteNode: %v", err)
	}
}
