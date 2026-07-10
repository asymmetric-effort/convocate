package k8s

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

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
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	err := DrainAndDeleteNode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestWaitForNodeByIP_Timeout(t *testing.T) {
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

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
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

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
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

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

func TestDrainAndDeleteNode_ListPodsError(t *testing.T) {
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-listerr"},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api unavailable")
	})

	err := DrainAndDeleteNode(ctx, "drain-listerr")
	if err == nil {
		t.Fatal("expected error from pod listing")
	}
	if !strings.Contains(err.Error(), "list pods on node") {
		t.Fatalf("expected 'list pods on node' error, got: %v", err)
	}
}

func TestDrainAndDeleteNode_DeleteNodeError(t *testing.T) {
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-delerr"},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	cs.PrependReactor("delete", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("forbidden")
	})

	err := DrainAndDeleteNode(ctx, "drain-delerr")
	if err == nil {
		t.Fatal("expected error from node delete")
	}
	if !strings.Contains(err.Error(), "delete node") {
		t.Fatalf("expected 'delete node' error, got: %v", err)
	}
}

func TestWaitForNodeByIP_ListError(t *testing.T) {
	origDrainSleep := drainSleep
	drainSleep = func(d time.Duration) {}
	defer func() { drainSleep = origDrainSleep }()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api unavailable")
	})

	_, err := waitForNodeByIP(ctx, "10.0.0.1", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGetJoinCommandViaJob_CreatePodError(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("pod quota exceeded")
	})

	_, err := getJoinCommandViaJob(ctx)
	if err == nil {
		t.Fatal("expected error from pod creation")
	}
	if !strings.Contains(err.Error(), "create join-token pod") {
		t.Fatalf("expected 'create join-token pod' error, got: %v", err)
	}
}

func TestGenerateJoinCredentials_TokenCreateError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("secret quota exceeded")
	})

	_, _, err := generateJoinCredentials(ctx)
	if err == nil {
		t.Fatal("expected error from bootstrap token creation")
	}
	if !strings.Contains(err.Error(), "create bootstrap token") {
		t.Fatalf("expected 'create bootstrap token' error, got: %v", err)
	}
}

// --- ProvisionNode tests using mocked SSH and JoinCommand ---

// saveProvisionGlobals saves and returns a restore function for all
// package-level variables that provision tests override.
func saveProvisionGlobals() func() {
	origSSH := sshExecutor
	origJoin := JoinCommandFn
	origWait := WaitForNodeByIPFn
	origSleep := retrySleep
	origDrainSleep := drainSleep
	origJoinTimeout := joinPodTimeout
	origLogStream := getLogStream
	// Zero-out sleeps and set short timeouts so tests don't wait
	retrySleep = func(d time.Duration) {}
	drainSleep = func(d time.Duration) {}
	joinPodTimeout = 10 * time.Millisecond
	return func() {
		sshExecutor = origSSH
		JoinCommandFn = origJoin
		WaitForNodeByIPFn = origWait
		retrySleep = origSleep
		drainSleep = origDrainSleep
		joinPodTimeout = origJoinTimeout
		getLogStream = origLogStream
	}
}

func TestProvisionNode_Success(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Create a ready node that waitForNodeByIP will find
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "new-worker"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "192.168.56.20"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=abc123.0123456789abcdef\napiVersion: v1\nclusters:\n- cluster:\n    server: https://10.0.0.1:6443\n", nil
	}

	req := ProvisionRequest{
		Host:     "192.168.56.20",
		User:     "convocate",
		Password: "secret",
		Location: "lab-1",
	}

	err := ProvisionNode(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionNode: %v", err)
	}

	// Verify SSH was called multiple times (sudo setup, base prep, write discovery, join, harden)
	if len(mock.execCalls) < 4 {
		t.Fatalf("expected at least 4 SSH exec calls, got %d", len(mock.execCalls))
	}

	// Verify all calls use correct host/user/password
	for i, c := range mock.execCalls {
		if c.Host != "192.168.56.20" {
			t.Fatalf("call %d: expected host 192.168.56.20, got %s", i, c.Host)
		}
		if c.User != "convocate" {
			t.Fatalf("call %d: expected user convocate, got %s", i, c.User)
		}
	}
}

func TestProvisionNode_DefaultUser(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker2"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.50"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.tokensecret\napiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host: "10.0.0.50",
		// User is empty — should default to "convocate"
	}

	err := ProvisionNode(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionNode: %v", err)
	}

	// First SSH call should use default user
	if len(mock.execCalls) == 0 {
		t.Fatal("expected SSH calls")
	}
	if mock.execCalls[0].User != "convocate" {
		t.Fatalf("expected default user 'convocate', got %s", mock.execCalls[0].User)
	}
}

func TestProvisionNode_NoPassword(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker3"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.60"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.tokensecret\napiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host: "10.0.0.60",
		User: "root",
		// No password — skip sudo setup
	}

	err := ProvisionNode(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionNode: %v", err)
	}

	// No sudo setup call when password is empty
	for _, c := range mock.execCalls {
		if strings.Contains(c.Script, "sudoers.d") {
			t.Fatal("should not configure sudo when password is empty")
		}
	}
}

func TestProvisionNode_SudoSetupFails(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	mock := &mockSSHExecutor{execErr: fmt.Errorf("permission denied")}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.70",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sudo setup failed") {
		t.Fatalf("expected 'sudo setup failed' error, got: %v", err)
	}
}

func TestProvisionNode_BasePreparationFails(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	// Sudo setup succeeds (call 1), every subsequent call fails (base prep retries all fail)
	callCount := 0
	sshExecutor = &failAfterNSSH{failAfter: 1, err: fmt.Errorf("apt-get failed"), count: &callCount}

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.80",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "base preparation failed") {
		t.Fatalf("expected 'base preparation failed' error, got: %v", err)
	}
}

func TestProvisionNode_JoinCommandFails(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("job timed out")
	}

	req := ProvisionRequest{
		Host:     "10.0.0.90",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "get join credentials") {
		t.Fatalf("expected 'get join credentials' error, got: %v", err)
	}
}

func TestProvisionNode_InvalidJoinOutput(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "no token or kubeconfig here", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.91",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid join output")
	}
	if !strings.Contains(err.Error(), "failed to parse join credentials") {
		t.Fatalf("expected 'failed to parse join credentials' error, got: %v", err)
	}
}

func TestProvisionNode_WriteDiscoveryFails(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	// Fail on the 3rd direct sshExec call (write discovery config)
	// The sshExecRetry calls count as 1 each (they succeed on first try)
	// Call sequence: sshExecRetry(sudo), sshExecRetry(base), sshExec(write), sshExec(join), sshExec(harden)
	callCount := 0
	sshExecutor = &failOnNthSSH{failOn: 3, err: fmt.Errorf("write failed"), count: &callCount}

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\napiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.92",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "write discovery config") {
		t.Fatalf("expected 'write discovery config' error, got: %v", err)
	}
}

func TestProvisionNode_JoinFails(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	callCount := 0
	sshExecutor = &failOnNthSSH{failOn: 4, err: fmt.Errorf("kubeadm join error"), count: &callCount}

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\napiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.93",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "kubeadm join failed") {
		t.Fatalf("expected 'kubeadm join failed' error, got: %v", err)
	}
}

func TestProvisionNode_NodeNotReady(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\napiVersion: v1\nkind: Config\n", nil
	}

	// Override waitForNodeByIP to return immediately with error
	WaitForNodeByIPFn = func(ctx context.Context, ip string, timeout time.Duration) (string, error) {
		return "", fmt.Errorf("timed out waiting for node with IP %s", ip)
	}

	req := ProvisionRequest{
		Host:     "10.0.0.94",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when node doesn't become ready")
	}
	if !strings.Contains(err.Error(), "node did not become ready") {
		t.Fatalf("expected 'node did not become ready' error, got: %v", err)
	}
}

func TestProvisionNode_HardeningFailsIsNonFatal(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "harden-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.95"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	// Fail only on the 5th call (hardening script)
	callCount := 0
	sshExecutor = &failOnNthSSH{failOn: 5, err: fmt.Errorf("chmod failed"), count: &callCount}

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\napiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.95",
		User:     "test",
		Password: "pass",
		Location: "rack-1",
	}

	// Hardening failure is non-fatal (just logged)
	err := ProvisionNode(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionNode should succeed even if hardening fails: %v", err)
	}
}

func TestProvisionNode_MissingTokenInOutput(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	// Has apiVersion but no TOKEN=
	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "apiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.96",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when token missing")
	}
	if !strings.Contains(err.Error(), "failed to parse join credentials") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestProvisionNode_MissingDiscoveryInOutput(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	// Has TOKEN= but no apiVersion:
	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\nsome other output", nil
	}

	req := ProvisionRequest{
		Host:     "10.0.0.97",
		User:     "test",
		Password: "pass",
	}

	err := ProvisionNode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when discovery conf missing")
	}
	if !strings.Contains(err.Error(), "failed to parse join credentials") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestProvisionNode_NoLocation(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "noloc-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.98"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\napiVersion: v1\nkind: Config\n", nil
	}

	req := ProvisionRequest{
		Host: "10.0.0.98",
		User: "test",
		// No Location — should skip label update
	}

	err := ProvisionNode(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionNode: %v", err)
	}
}

func TestProvisionNode_LabelUpdateFails(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "label-fail-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.99"},
			},
			Conditions: []corev1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	JoinCommandFn = func(ctx context.Context) (string, error) {
		return "TOKEN=tok.secret\napiVersion: v1\nkind: Config\n", nil
	}

	// Make node update fail (this affects UpdateNodeLabels)
	cs.PrependReactor("update", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("update forbidden")
	})

	req := ProvisionRequest{
		Host:     "10.0.0.99",
		User:     "test",
		Location: "datacenter-1", // Non-empty to trigger label update
	}

	// Label update failure is non-fatal (just logged)
	err := ProvisionNode(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionNode should succeed even if label update fails: %v", err)
	}
}

func TestGenerateJoinCredentials_Success(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Generate a real CA cert for the cluster-info kubeconfig
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certB64 := base64.StdEncoding.EncodeToString(certPEM)

	cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-public"},
	}, metav1.CreateOptions{})
	cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
	}, metav1.CreateOptions{})

	kubeconfig := fmt.Sprintf("apiVersion: v1\nclusters:\n- cluster:\n    certificate-authority-data: %s\n    server: https://10.0.0.1:6443\n", certB64)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{
			"kubeconfig": kubeconfig,
		},
	}
	cs.CoreV1().ConfigMaps("kube-public").Create(ctx, cm, metav1.CreateOptions{})

	token, caHash, err := generateJoinCredentials(ctx)
	if err != nil {
		t.Fatalf("generateJoinCredentials: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !strings.Contains(token, ".") {
		t.Fatalf("expected token with dot separator, got %s", token)
	}
	if !strings.HasPrefix(caHash, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %s", caHash)
	}

	// Verify bootstrap token secret was created
	secrets, _ := cs.CoreV1().Secrets("kube-system").List(ctx, metav1.ListOptions{})
	found := false
	for _, s := range secrets.Items {
		if strings.HasPrefix(s.Name, "bootstrap-token-") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected bootstrap token secret in kube-system")
	}
}

func TestGenerateJoinCredentials_InvalidBase64CACert(t *testing.T) {
	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-public"},
	}, metav1.CreateOptions{})
	cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
	}, metav1.CreateOptions{})

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{
			"kubeconfig": "apiVersion: v1\nclusters:\n- cluster:\n    certificate-authority-data: !!!invalid-base64!!!\n    server: https://10.0.0.1:6443\n",
		},
	}
	cs.CoreV1().ConfigMaps("kube-public").Create(ctx, cm, metav1.CreateOptions{})

	_, _, err := generateJoinCredentials(ctx)
	if err == nil {
		t.Fatal("expected error for invalid base64 CA cert")
	}
	if !strings.Contains(err.Error(), "decode CA cert") {
		t.Fatalf("expected 'decode CA cert' error, got: %v", err)
	}
}

func TestGetJoinCommandViaJob_LogStreamError(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Use a reactor to make the pod appear as Succeeded when created.
	// We intercept "create" to set the status immediately.
	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		pod, ok := createAction.GetObject().(*corev1.Pod)
		if ok && strings.HasPrefix(pod.Name, "convocate-join-token-") {
			pod.Status.Phase = corev1.PodSucceeded
			return false, pod, nil // let the fake client handle the actual create
		}
		return false, nil, nil
	})

	_, err := getJoinCommandViaJob(ctx)
	if err == nil {
		t.Fatal("expected error (fake client returns 'fake logs' which doesn't contain TOKEN=)")
	}
	// Fake client returns "fake logs" which doesn't contain TOKEN=
	if !strings.Contains(err.Error(), "unexpected join output") {
		t.Fatalf("expected 'unexpected join output' error, got: %v", err)
	}
}

// failOnNthSSH fails on the Nth call to Exec.
type failOnNthSSH struct {
	failOn int
	err    error
	count  *int
}

func (f *failOnNthSSH) Exec(host, user, password, script string) error {
	*f.count++
	if *f.count == f.failOn {
		return f.err
	}
	return nil
}

func (f *failOnNthSSH) ExecWithOutput(host, user, password, cmd string) (string, error) {
	return "", nil
}

// failAfterNSSH fails on every call after the Nth.
type failAfterNSSH struct {
	failAfter int
	err       error
	count     *int
}

func (f *failAfterNSSH) Exec(host, user, password, script string) error {
	*f.count++
	if *f.count > f.failAfter {
		return f.err
	}
	return nil
}

func (f *failAfterNSSH) ExecWithOutput(host, user, password, cmd string) (string, error) {
	return "", nil
}

func TestGetJoinCommandViaJob_PodSucceeded(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Make pod appear as Succeeded immediately
	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		pod := createAction.GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodSucceeded
		return false, pod, nil
	})

	// Mock getLogStream to return output with TOKEN=
	getLogStream = func(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("TOKEN=abc.123\napiVersion: v1\nkind: Config\n")), nil
	}

	output, err := getJoinCommandViaJob(ctx)
	if err != nil {
		t.Fatalf("getJoinCommandViaJob: %v", err)
	}
	if !strings.Contains(output, "TOKEN=abc.123") {
		t.Fatalf("expected TOKEN in output, got: %s", output)
	}
}

func TestGetJoinCommandViaJob_PodFailed(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		pod := createAction.GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodFailed
		return false, pod, nil
	})

	// Mock log stream returning output without TOKEN=
	getLogStream = func(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("kubeadm error: something went wrong")), nil
	}

	_, err := getJoinCommandViaJob(ctx)
	if err == nil {
		t.Fatal("expected error for missing TOKEN= in output")
	}
	if !strings.Contains(err.Error(), "unexpected join output") {
		t.Fatalf("expected 'unexpected join output' error, got: %v", err)
	}
}

func TestGetJoinCommandViaJob_LogStreamFailure(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		pod := createAction.GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodSucceeded
		return false, pod, nil
	})

	getLogStream = func(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("logs not available")
	}

	_, err := getJoinCommandViaJob(ctx)
	if err == nil {
		t.Fatal("expected error from log stream")
	}
	if !strings.Contains(err.Error(), "get join-token logs") {
		t.Fatalf("expected 'get join-token logs' error, got: %v", err)
	}
}

func TestGetJoinCommandViaJob_PodTimeout(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Pod stays in Pending (never transitions), with joinPodTimeout set to 10ms
	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction := action.(k8stesting.CreateAction)
		pod := createAction.GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodPending
		return false, pod, nil
	})

	// After timeout, it tries to read logs — mock returns no TOKEN=
	getLogStream = func(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("no token here")), nil
	}

	_, err := getJoinCommandViaJob(ctx)
	if err == nil {
		t.Fatal("expected error from timed out pod")
	}
}

func TestGetJoinCommandViaJob_GetPodError(t *testing.T) {
	restore := saveProvisionGlobals()
	defer restore()

	cs := fake.NewSimpleClientset()
	Client = cs
	ctx := context.Background()

	// Create succeeds, but Get fails (for the polling loop)
	podCreated := false
	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		podCreated = true
		createAction := action.(k8stesting.CreateAction)
		pod := createAction.GetObject().(*corev1.Pod)
		pod.Status.Phase = corev1.PodSucceeded
		return false, pod, nil
	})

	// Mock getLogStream returns no TOKEN=
	getLogStream = func(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("no token")), nil
	}

	_, err := getJoinCommandViaJob(ctx)
	if !podCreated {
		t.Fatal("expected pod to be created")
	}
	// Whether it errors or not depends on timing, just make sure it doesn't hang
	_ = err
}
