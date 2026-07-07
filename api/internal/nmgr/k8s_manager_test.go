package nmgr

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/asymmetric-effort/convocate/internal/db"
	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupFakeK8s(t *testing.T) {
	t.Helper()
	cs := fake.NewSimpleClientset()
	k8s.Client = cs

	ctx := context.Background()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: k8s.AgentNamespace}}
	cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})

	t.Cleanup(func() { k8s.Client = nil })
}

func TestK8sNodeManager_ListNodes(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			},
		},
	}
	k8s.Client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	nodes, err := mgr.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

func TestK8sNodeManager_GetNode(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			},
		},
	}
	k8s.Client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	n, err := mgr.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n.ID != "node-1" {
		t.Errorf("expected ID 'node-1', got %q", n.ID)
	}
}

func TestK8sNodeManager_GetNode_NotFound(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	_, err := mgr.GetNode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestK8sNodeManager_GetNodeDetail(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			},
		},
	}
	k8s.Client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	detail, err := mgr.GetNodeDetail(ctx, "node-1")
	if err != nil {
		t.Fatalf("GetNodeDetail: %v", err)
	}
	if detail.Node.ID != "node-1" {
		t.Errorf("expected ID 'node-1', got %q", detail.Node.ID)
	}
}

func TestK8sNodeManager_GetNodeDetail_NotFound(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	_, err := mgr.GetNodeDetail(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestK8sNodeManager_CordonNode(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	}
	k8s.Client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	err := mgr.CordonNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("CordonNode: %v", err)
	}
}

func TestK8sNodeManager_UncordonNode(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Spec:       corev1.NodeSpec{Unschedulable: true},
	}
	k8s.Client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	err := mgr.UncordonNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("UncordonNode: %v", err)
	}
}

func TestK8sNodeManager_CountAgentPodsOnNode(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	count, err := mgr.CountAgentPodsOnNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("CountAgentPodsOnNode: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestK8sNodeManager_ListAgentPodsOnNode(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	agents, err := mgr.ListAgentPodsOnNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("ListAgentPodsOnNode: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestPgNoteDB_HasDB_NilPool(t *testing.T) {
	ndb := pgNoteDB{}
	if ndb.HasDB() {
		t.Error("expected false when db.Pool is nil")
	}
}

func setupFakeDB(t *testing.T) {
	t.Helper()
	// Create a Pool with a bogus config that will fail on any query
	// but doesn't fail on pool creation (lazy connect).
	cfg, err := pgxpool.ParseConfig("postgres://bogus:bogus@127.0.0.1:1/bogus?connect_timeout=1")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	cfg.MaxConns = 1
	cfg.MinConns = 0
	// Create pool without actually connecting (lazy)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Skipf("cannot create pool: %v", err)
	}
	db.Pool = pool
	t.Cleanup(func() {
		pool.Close()
		db.Pool = nil
	})
}

func TestPgNoteDB_HasDB_WithPool(t *testing.T) {
	setupFakeDB(t)
	ndb := pgNoteDB{}
	if !ndb.HasDB() {
		t.Error("expected true when db.Pool is set")
	}
}

func TestPgNoteDB_ListNotes_Error(t *testing.T) {
	setupFakeDB(t)
	ndb := pgNoteDB{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := ndb.ListNotes(ctx, "node-001")
	if err == nil {
		t.Error("expected error from bogus connection")
	}
}

func TestPgNoteDB_AddNote_Error(t *testing.T) {
	setupFakeDB(t)
	ndb := pgNoteDB{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := ndb.AddNote(ctx, "node-001", "admin", "test note")
	if err == nil {
		t.Error("expected error from bogus connection")
	}
}

func TestK8sNodeManager_DrainAndDeleteNode(t *testing.T) {
	setupFakeK8s(t)
	mgr := k8sNodeManager{}
	ctx := context.Background()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-del"},
	}
	k8s.Client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})

	err := mgr.DrainAndDeleteNode(ctx, "node-del")
	if err != nil {
		t.Fatalf("DrainAndDeleteNode: %v", err)
	}
}
