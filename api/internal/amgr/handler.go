package amgr

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/types"
)

// k8sSATokenHeader is the header used for K8s SA token mutual auth
// between the API and agent wrapper.
const k8sSATokenHeader = "X-K8s-SA-Token"

type Handler struct {
	store   *Store
	useK8s  bool
	saToken string // API's own K8s SA token for agent auth
}

func Register(mux *http.ServeMux) {
	// Load the API's own K8s SA token for authenticating with agent pods
	saToken := ""
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		saToken = strings.TrimSpace(string(data))
	}

	h := &Handler{
		store:   NewStore(),
		useK8s:  k8s.Client != nil,
		saToken: saToken,
	}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/amgr/agent", middleware.Chain(http.HandlerFunc(h.list), auth, middleware.RBAC("agent-view")))
	mux.Handle("POST /api/v1/amgr/agent", middleware.Chain(http.HandlerFunc(h.create), auth, middleware.RBAC("agent-update")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}", middleware.Chain(http.HandlerFunc(h.get), auth, middleware.RBAC("agent-view")))
	mux.Handle("PATCH /api/v1/amgr/agent/{agentId}", middleware.Chain(http.HandlerFunc(h.update), auth, middleware.RBAC("agent-update")))
	mux.Handle("DELETE /api/v1/amgr/agent/{agentId}", middleware.Chain(http.HandlerFunc(h.del), auth, middleware.RBAC("agent-update")))
	mux.Handle("POST /api/v1/amgr/agent/{agentId}/start", middleware.Chain(http.HandlerFunc(h.start), auth, middleware.RBAC("agent-update")))
	mux.Handle("POST /api/v1/amgr/agent/{agentId}/stop", middleware.Chain(http.HandlerFunc(h.stop), auth, middleware.RBAC("agent-update")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}/shell", middleware.Chain(http.HandlerFunc(h.shell), auth, middleware.RBAC("agent-view")))

	// I/O proxy routes — relay to the agent wrapper's endpoints
	mux.Handle("POST /api/v1/amgr/agent/{agentId}/stdin", middleware.Chain(http.HandlerFunc(h.proxyStdin), auth, middleware.RBAC("agent-update")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}/stdout", middleware.Chain(http.HandlerFunc(h.proxyStdout), auth, middleware.RBAC("agent-view")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}/stderr", middleware.Chain(http.HandlerFunc(h.proxyStderr), auth, middleware.RBAC("agent-view")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}/metrics", middleware.Chain(http.HandlerFunc(h.proxyMetrics), auth, middleware.RBAC("agent-view")))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		agents, err := k8s.ListAgentPods(ctx)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "k8s_error", err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(agents, offset, limit))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.List(), offset, limit))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req types.CreateAgentRequest
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}

	// Admin-only: reject security overrides from non-admin callers
	if req.Security != nil {
		p, _ := httputil.PrincipalFromContext(r.Context())
		isAdmin := false
		if p != nil {
			for _, role := range p.Roles {
				if role == "admin" {
					isAdmin = true
					break
				}
			}
		}
		if !isAdmin {
			httputil.WriteError(w, http.StatusForbidden, "forbidden", "security overrides require admin role")
			return
		}
	}

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		p, _ := httputil.PrincipalFromContext(r.Context())
		owner := "system"
		if p != nil {
			owner = p.Username
		}
		agent, err := k8s.CreateAgentPod(ctx, req, owner)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, agent)
		return
	}
	agent := h.store.Create(Agent{Project: req.Project, NodeID: req.NodeID, Owner: "admin:admins"})
	httputil.WriteJSON(w, http.StatusCreated, agent)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		agent, err := k8s.GetAgentPod(ctx, id)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, agent)
		return
	}
	agent, ok := h.store.Get(id)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, agent)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	var req types.ConfigureAgentRequest
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}

	// Admin-only: reject security overrides from non-admin callers
	if req.Security != nil {
		p, _ := httputil.PrincipalFromContext(r.Context())
		isAdmin := false
		if p != nil {
			for _, role := range p.Roles {
				if role == "admin" {
					isAdmin = true
					break
				}
			}
		}
		if !isAdmin {
			httputil.WriteError(w, http.StatusForbidden, "forbidden", "security overrides require admin role")
			return
		}
	}

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Update CLAUDE.md via ConfigMap if provided
		if req.ClaudeMd != nil {
			if err := k8s.UpdateAgentConfigMap(ctx, id, *req.ClaudeMd); err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "update_failed", err.Error())
				return
			}
		}

		// Return the current agent state
		agent, err := k8s.GetAgentPod(ctx, id)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, agent)
		return
	}

	agent, ok := h.store.Update(id, req.Project, req.NodeID, req.Expose)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, agent)
}

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := k8s.DeleteAgentPod(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !h.store.Delete(id) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if h.useK8s {
		// Re-create the pod if it was previously stopped.
		// The PVC and ConfigMap still exist from the original create.
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Check if the pod already exists
		_, err := k8s.GetAgentPod(ctx, id)
		if err == nil {
			httputil.WriteError(w, http.StatusConflict, "conflict", "agent is already running")
			return
		}

		// Re-create with default settings (the PVC and ConfigMap persist)
		p, _ := httputil.PrincipalFromContext(r.Context())
		owner := "system"
		if p != nil {
			owner = p.Username
		}
		// Extract project name from agent ID (agent-{project})
		project := id
		if len(id) > 6 && id[:6] == "agent-" {
			project = id[6:]
		}
		_, createErr := k8s.CreateAgentPod(ctx, types.CreateAgentRequest{
			Project: project,
		}, owner)
		if createErr != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "start_failed", createErr.Error())
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "running") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		// Stop = delete the pod only (not PVC/ConfigMap)
		if err := k8s.StopAgentPod(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "stopping") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// shell proxies a WebSocket connection to the agent's stdout + stdin
func (h *Handler) shell(w http.ResponseWriter, r *http.Request) {
	h.proxyStdout(w, r)
}

// ---------------------------------------------------------------------------
// I/O proxy handlers — relay requests to the agent wrapper pod
// ---------------------------------------------------------------------------

// getAgentPodIP looks up the agent pod's cluster IP for proxying.
func (h *Handler) getAgentPodIP(id string) (string, error) {
	if !h.useK8s {
		return "", fmt.Errorf("proxy not available in non-K8s mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pod, err := k8s.Client.CoreV1().Pods(k8s.AgentNamespace).Get(ctx, id, k8s.GetOpts())
	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}
	if pod.Status.PodIP == "" {
		return "", fmt.Errorf("agent pod has no IP (not yet running)")
	}
	return pod.Status.PodIP, nil
}

// proxyStdin forwards POST body to the agent wrapper's /stdin endpoint.
func (h *Handler) proxyStdin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	ip, err := h.getAgentPodIP(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	url := fmt.Sprintf("http://%s:8443/stdin", ip)
	proxyReq, _ := http.NewRequestWithContext(r.Context(), "POST", url, r.Body)
	proxyReq.Header.Set("Authorization", r.Header.Get("Authorization"))
	proxyReq.Header.Set("Content-Type", "application/octet-stream")
	if h.saToken != "" {
		proxyReq.Header.Set(k8sSATokenHeader, h.saToken)
	}

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "proxy_error", err.Error())
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// proxyStdout proxies the WebSocket/SSE stream from the agent's /stdout.
func (h *Handler) proxyStdout(w http.ResponseWriter, r *http.Request) {
	h.proxyStream(w, r, "stdout")
}

// proxyStderr proxies the WebSocket/SSE stream from the agent's /stderr.
func (h *Handler) proxyStderr(w http.ResponseWriter, r *http.Request) {
	h.proxyStream(w, r, "stderr")
}

// proxyMetrics forwards the GET request to the agent's /metrics.
func (h *Handler) proxyMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	ip, err := h.getAgentPodIP(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	url := fmt.Sprintf("http://%s:8443/metrics", ip)
	proxyReq, _ := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	proxyReq.Header.Set("Authorization", r.Header.Get("Authorization"))
	if h.saToken != "" {
		proxyReq.Header.Set(k8sSATokenHeader, h.saToken)
	}

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "proxy_error", err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// proxyStream proxies stdout or stderr from the agent wrapper as SSE.
func (h *Handler) proxyStream(w http.ResponseWriter, r *http.Request, stream string) {
	id := r.PathValue("agentId")
	ip, err := h.getAgentPodIP(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	// Use SSE fallback since the proxy chain may not support WebSocket
	url := fmt.Sprintf("http://%s:8443/%s", ip, stream)
	proxyReq, _ := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	proxyReq.Header.Set("Authorization", r.Header.Get("Authorization"))
	if h.saToken != "" {
		proxyReq.Header.Set(k8sSATokenHeader, h.saToken)
	}

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "proxy_error", err.Error())
		return
	}
	defer resp.Body.Close()

	// Stream the response
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if ok {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}
