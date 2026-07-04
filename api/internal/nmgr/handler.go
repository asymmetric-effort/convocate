package nmgr

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/asymmetric-effort/convocate/internal/events"
	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/types"
)

// NoteDB abstracts the database operations for node notes so they can
// be replaced with mocks in tests.
type NoteDB interface {
	// HasDB returns true when a database backend is available.
	HasDB() bool
	// ListNotes returns all notes for the given node from the database.
	ListNotes(ctx context.Context, nodeID string) ([]types.Note, error)
	// AddNote inserts a note and returns its created_at timestamp.
	AddNote(ctx context.Context, nodeID, author, text string) (time.Time, error)
}

// NodeManager abstracts the K8s operations used by the handler so
// they can be replaced with mocks in tests.
type NodeManager interface {
	ListNodes(ctx context.Context) ([]types.Node, error)
	GetNode(ctx context.Context, name string) (*types.Node, error)
	GetNodeDetail(ctx context.Context, name string) (*types.NodeDetail, error)
	CordonNode(ctx context.Context, name string) error
	UncordonNode(ctx context.Context, name string) error
	CountAgentPodsOnNode(ctx context.Context, nodeName string) (int, error)
	ListAgentPodsOnNode(ctx context.Context, nodeName string) ([]types.Agent, error)
	DrainAndDeleteNode(ctx context.Context, nodeName string) error
	ProvisionNode(ctx context.Context, req k8s.ProvisionRequest) error
}

// metricsEntry wraps a DaemonSet metrics report with its receive time
// so we can detect stale data.
type metricsEntry struct {
	report   types.NodeMetricsReport
	received time.Time
}

type Handler struct {
	store  *Store
	useK8s bool
	mgr    NodeManager
	noteDB NoteDB
	// nodeMetrics holds the latest metrics report from each node's
	// DaemonSet pod, keyed by node name.
	nodeMetrics sync.Map // map[string]metricsEntry
}

func Register(mux *http.ServeMux) {
	h := &Handler{
		store:  NewStore(),
		useK8s: k8s.Client != nil,
		mgr:    k8sNodeManager{},
		noteDB: pgNoteDB{},
	}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/nmgr/node", middleware.Chain(http.HandlerFunc(h.list), auth, middleware.RBAC("node-view")))
	mux.Handle("POST /api/v1/nmgr/node", middleware.Chain(http.HandlerFunc(h.create), auth, middleware.RBAC("node-create")))
	mux.Handle("GET /api/v1/nmgr/node/{nodeId}", middleware.Chain(http.HandlerFunc(h.get), auth, middleware.RBAC("node-view")))
	mux.Handle("PATCH /api/v1/nmgr/node/{nodeId}", middleware.Chain(http.HandlerFunc(h.update), auth, middleware.RBAC("node-update")))
	mux.Handle("DELETE /api/v1/nmgr/node/{nodeId}", middleware.Chain(http.HandlerFunc(h.del), auth, middleware.RBAC("node-delete")))
	mux.Handle("POST /api/v1/nmgr/node/{nodeId}/start", middleware.Chain(http.HandlerFunc(h.start), auth, middleware.RBAC("node-update")))
	mux.Handle("POST /api/v1/nmgr/node/{nodeId}/stop", middleware.Chain(http.HandlerFunc(h.stop), auth, middleware.RBAC("node-update")))
	mux.Handle("GET /api/v1/nmgr/node/{nodeId}/note", middleware.Chain(http.HandlerFunc(h.listNotes), auth, middleware.RBAC("node-view")))
	mux.Handle("POST /api/v1/nmgr/node/{nodeId}/note", middleware.Chain(http.HandlerFunc(h.addNote), auth, middleware.RBAC("node-update")))

	// Metrics ingest endpoint — called by the node-metrics DaemonSet
	mux.Handle("POST /api/v1/nmgr/metrics", middleware.Chain(http.HandlerFunc(h.ingestMetrics), middleware.InternalAuth))

	go h.publishMetrics()
}

// publishMetrics periodically fetches node metrics and publishes them
// to the nmgr/status event channel so connected clients get real-time
// updates for memory, load average, and disk usage.
func (h *Handler) publishMetrics() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.collectAndPublishMetrics()
	}
}

// collectAndPublishMetrics performs a single metrics collection and publish
// cycle.  Called by publishMetrics on each ticker tick.
func (h *Handler) collectAndPublishMetrics() {
	var nodes []types.Node

	if h.useK8s {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		k8sNodes, err := h.mgr.ListNodes(ctx)
		cancel()
		if err != nil {
			return
		}
		nodes = k8sNodes
	} else {
		// Mock mode: jitter metrics to simulate real activity
		h.store.JitterMetrics()
		storeNodes := h.store.List()
		for _, sn := range storeNodes {
			nodes = append(nodes, types.Node{
				ID:          sn.ID,
				IP:          sn.IP,
				Status:      types.NodeStatus(sn.Status),
				Agents:      sn.Agents,
				LoadAvg:     types.LoadAvg(sn.LoadAvg),
				MemUsedGB:   sn.MemUsedGB,
				MemTotalGB:  sn.MemTotalGB,
				DiskUsedGB:  sn.DiskUsedGB,
				DiskTotalGB: sn.DiskTotalGB,
			})
		}
	}

	// Overlay real DaemonSet metrics onto each node
	for i := range nodes {
		h.mergeNodeMetrics(&nodes[i])
	}

	if len(nodes) > 0 {
		events.DefaultHub.Publish("nmgr/status", "node.metrics", nodes)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		nodes, err := h.mgr.ListNodes(ctx)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "k8s_error", err.Error())
			return
		}
		for i := range nodes {
			count, _ := h.mgr.CountAgentPodsOnNode(ctx, nodes[i].ID)
			nodes[i].Agents = count
		}
		// Merge in pending/provisioning nodes from the store that
		// are not yet visible in K8s.
		k8sIDs := make(map[string]bool, len(nodes))
		for _, n := range nodes {
			k8sIDs[n.ID] = true
			k8sIDs[n.IP] = true
		}
		for _, sn := range h.store.List() {
			if !k8sIDs[sn.ID] && !k8sIDs[sn.IP] {
				nodes = append(nodes, types.Node{
					ID:     sn.ID,
					IP:     sn.IP,
					Status: types.NodeStatus(sn.Status),
					Agents: sn.Agents,
				})
			}
		}
		// Overlay real DaemonSet metrics for each node
		for i := range nodes {
			h.mergeNodeMetrics(&nodes[i])
		}
		httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(nodes, offset, limit))
		return
	}

	nodes := h.store.List()
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(nodes, offset, limit))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Host     string   `json:"host"`
		User     string   `json:"user"`
		Password string   `json:"password,omitempty"`
		Location string   `json:"location"`
		Tags     []string `json:"tags"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}

	if req.Host == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "host is required")
		return
	}
	if req.User == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "user is required")
		return
	}

	// Validate node name format: lowercase alphanumeric + hyphens, no leading/trailing hyphens
	if req.Name != "" {
		for _, ch := range req.Name {
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
				httputil.WriteError(w, http.StatusBadRequest, "validation_failed",
					"node name must contain only lowercase letters, digits, and hyphens")
				return
			}
		}
		if req.Name[0] == '-' || req.Name[len(req.Name)-1] == '-' {
			httputil.WriteError(w, http.StatusBadRequest, "validation_failed",
				"node name must not start or end with a hyphen")
			return
		}
		if len(req.Name) > 63 {
			httputil.WriteError(w, http.StatusBadRequest, "validation_failed",
				"node name must be 63 characters or fewer")
			return
		}
	}

	// Validate host is an IP or hostname (basic sanity check)
	if len(req.Host) > 253 {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed",
			"host must be 253 characters or fewer")
		return
	}

	// Check for name uniqueness — reject if a node with this name
	// already exists in K8s or in the pending store
	if req.Name != "" {
		if _, exists := h.store.Get(req.Name); exists {
			httputil.WriteError(w, http.StatusConflict, "conflict",
				"a node with this name already exists or is being provisioned")
			return
		}
		if h.useK8s {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			_, err := h.mgr.GetNode(ctx, req.Name)
			cancel()
			if err == nil {
				httputil.WriteError(w, http.StatusConflict, "conflict",
					"a node with this name already exists in the cluster")
				return
			}
		}
	}

	// Check for host uniqueness — reject if a node with this IP/host
	// is already in the cluster or pending
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		existingNodes, err := h.mgr.ListNodes(ctx)
		cancel()
		if err == nil {
			for _, n := range existingNodes {
				if n.IP == req.Host {
					httputil.WriteError(w, http.StatusConflict, "conflict",
						"a node with this IP address already exists in the cluster")
					return
				}
			}
		}
	}
	for _, sn := range h.store.List() {
		if sn.IP == req.Host {
			httputil.WriteError(w, http.StatusConflict, "conflict",
				"a node with this IP address is already being provisioned")
			return
		}
	}

	if h.useK8s && req.Password != "" {
		// Real provisioning: SSH to target, install K8s, join cluster.
		// Store the pending node so it appears in list responses while
		// provisioning is in progress.
		node := h.store.Create(Node{
			ID:       req.Name,
			IP:       req.Host,
			Location: req.Location,
			Tags:     req.Tags,
			Status:   "Pending",
		})
		events.DefaultHub.Publish("nmgr/status", "node.pending", node)
		httputil.WriteJSON(w, http.StatusAccepted, node)

		// Run provisioning asynchronously
		go func() {
			provReq := k8s.ProvisionRequest{
				Host:     req.Host,
				User:     req.User,
				Password: req.Password,
				Location: req.Location,
			}
			if err := h.mgr.ProvisionNode(context.Background(), provReq); err != nil {
				log.Printf("[provision] ERROR: %v", err)
				h.store.SetStatus(node.ID, "Error")
				events.DefaultHub.Publish("nmgr/status", "node.error", map[string]string{
					"id":    node.ID,
					"error": err.Error(),
				})
				return
			}
			// Node is now in K8s — remove from pending store
			h.store.Delete(node.ID)
			events.DefaultHub.Publish("nmgr/status", "node.ready", map[string]string{
				"id":     node.ID,
				"status": "Ready",
			})
		}()
		return
	}

	node := h.store.Create(Node{ID: req.Name, IP: req.Host, Location: req.Location, Tags: req.Tags})
	events.DefaultHub.Publish("nmgr/status", "node.pending", node)

	// Mock mode: transition to Ready after a short delay
	go func() {
		time.Sleep(3 * time.Second)
		if h.store.SetStatus(node.ID, "Ready") {
			node.Status = "Ready"
			events.DefaultHub.Publish("nmgr/status", "node.ready", node)
		}
	}()

	httputil.WriteJSON(w, http.StatusAccepted, node)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		detail, err := h.mgr.GetNodeDetail(ctx, id)
		if err == nil {
			agents, _ := h.mgr.ListAgentPodsOnNode(ctx, id)
			detail.Node.Agents = len(agents)
			h.mergeNodeMetrics(&detail.Node)
			detail.AgentList = agents
			detail.Notes = h.getNotesFromDB(id)
			httputil.WriteJSON(w, http.StatusOK, detail)
			return
		}
		// Fall through to check the store for pending/provisioning nodes
		// that are not yet visible in K8s.
	}

	node, ok := h.store.Get(id)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	detail := struct {
		Node
		AgentList []any  `json:"agentList"`
		Notes     []Note `json:"notes"`
	}{Node: node, AgentList: []any{}, Notes: h.store.ListNotes(id)}
	httputil.WriteJSON(w, http.StatusOK, detail)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	var req struct {
		Location *string  `json:"location"`
		Tags     []string `json:"tags"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	node, ok := h.store.Update(id, req.Location, req.Tags)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, node)
}

// countReadyNodes returns the number of nodes in Ready status.
// Used to enforce the minimum-3-Ready-nodes safety rule.
func (h *Handler) countReadyNodes(ctx context.Context) int {
	if h.useK8s {
		nodes, err := h.mgr.ListNodes(ctx)
		if err != nil {
			return 0
		}
		count := 0
		for _, n := range nodes {
			if n.Status == types.NodeReady {
				count++
			}
		}
		return count
	}
	count := 0
	for _, n := range h.store.List() {
		if n.Status == "Ready" {
			count++
		}
	}
	return count
}

const minReadyNodes = 4 // must have at least this many Ready nodes to stop/delete one

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")

	// Check the in-memory store first — pending/error nodes from provisioning
	// only exist there.  These don't affect cluster capacity so the
	// minimum-node-count guard doesn't apply.
	if h.store.Delete(id) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if h.useK8s {
		// Safety: require at least minReadyNodes Ready nodes before deleting
		// a real K8s node
		ctx0, cancel0 := context.WithTimeout(r.Context(), 5*time.Second)
		readyCount := h.countReadyNodes(ctx0)
		cancel0()
		if readyCount < minReadyNodes {
			httputil.WriteError(w, http.StatusConflict, "insufficient_nodes",
				"cannot delete: cluster must maintain at least 3 Ready nodes")
			return
		}

		// Real K8s node: drain all pods then remove from cluster
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		if err := h.mgr.DrainAndDeleteNode(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := h.mgr.UncordonNode(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "Ready") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")

	// Safety: require at least minReadyNodes Ready nodes before allowing stop
	ctx0, cancel0 := context.WithTimeout(r.Context(), 5*time.Second)
	readyCount := h.countReadyNodes(ctx0)
	cancel0()
	if readyCount < minReadyNodes {
		httputil.WriteError(w, http.StatusConflict, "insufficient_nodes",
			"cannot stop: cluster must maintain at least 3 Ready nodes")
		return
	}

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := h.mgr.CordonNode(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "SchedulingDisabled") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if h.noteDB != nil && h.noteDB.HasDB() {
		httputil.WriteJSON(w, http.StatusOK, h.getNotesFromDB(id))
		return
	}
	notes := h.store.ListNotes(id)
	httputil.WriteJSON(w, http.StatusOK, notes)
}

func (h *Handler) addNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	var req struct {
		Text string `json:"text"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil || req.Text == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "text is required")
		return
	}
	p, _ := httputil.PrincipalFromContext(r.Context())
	author := "system"
	if p != nil {
		author = p.Username
	}

	if h.noteDB != nil && h.noteDB.HasDB() {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		createdAt, err := h.noteDB.AddNote(ctx, id, author, req.Text)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, types.Note{
			Author: author, CreatedAt: createdAt.UTC().Format(time.RFC3339), Text: req.Text,
		})
		return
	}

	note := h.store.AddNote(id, Note{Author: author, Text: req.Text})
	httputil.WriteJSON(w, http.StatusCreated, note)
}

func (h *Handler) getNotesFromDB(nodeID string) []types.Note {
	if h.noteDB == nil || !h.noteDB.HasDB() {
		mockNotes := h.store.ListNotes(nodeID)
		var notes []types.Note
		for _, n := range mockNotes {
			notes = append(notes, types.Note{Author: n.Author, CreatedAt: n.CreatedAt, Text: n.Text})
		}
		return notes
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	notes, err := h.noteDB.ListNotes(ctx, nodeID)
	if err != nil {
		return nil
	}
	return notes
}

// ingestMetrics receives a NodeMetricsReport from the node-metrics
// DaemonSet and stores it for the next publishMetrics cycle to merge.
func (h *Handler) ingestMetrics(w http.ResponseWriter, r *http.Request) {
	var report types.NodeMetricsReport
	if err := httputil.ReadJSON(r, &report); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid metrics report")
		return
	}
	if report.NodeName == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "nodeName is required")
		return
	}
	h.nodeMetrics.Store(report.NodeName, metricsEntry{
		report:   report,
		received: time.Now(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// mergeNodeMetrics overlays real DaemonSet metrics onto a node if a
// fresh report (< 10s old) exists.
func (h *Handler) mergeNodeMetrics(node *types.Node) {
	val, ok := h.nodeMetrics.Load(node.ID)
	if !ok {
		return
	}
	entry := val.(metricsEntry)
	if time.Since(entry.received) > 10*time.Second {
		return // stale data, skip
	}
	r := entry.report
	node.LoadAvg = r.LoadAvg
	if r.MemTotalBytes > 0 {
		node.MemUsedGB = float64(r.MemUsedBytes) / (1024 * 1024 * 1024)
		node.MemTotalGB = float64(r.MemTotalBytes) / (1024 * 1024 * 1024)
	}
	if r.SwapTotalBytes > 0 {
		node.SwapUsedGB = float64(r.SwapUsedBytes) / (1024 * 1024 * 1024)
		node.SwapTotalGB = float64(r.SwapTotalBytes) / (1024 * 1024 * 1024)
	}
	if r.DiskTotalBytes > 0 {
		node.DiskUsedGB = float64(r.DiskUsedBytes) / (1024 * 1024 * 1024)
		node.DiskTotalGB = float64(r.DiskTotalBytes) / (1024 * 1024 * 1024)
	}
	node.UptimeSeconds = r.UptimeSeconds
	if r.KubeletVersion != "" {
		node.KubeletVersion = r.KubeletVersion
	}
	if r.CPUCount > 0 {
		node.CPUCount = r.CPUCount
	}
}
