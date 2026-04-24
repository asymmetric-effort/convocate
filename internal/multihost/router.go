// Package multihost aggregates session state from the local claude-shell
// host and any number of remote claude-agent hosts into a single view for
// the TUI. Lookups are routed by session UUID — the shell-side routing map
// is rebuilt on every List so newly-added agents or newly-created remote
// sessions become addressable without a restart.
package multihost

import (
	"fmt"
	"sync"

	"github.com/asymmetric-effort/claude-shell/internal/agentclient"
	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
	"github.com/asymmetric-effort/claude-shell/internal/session"
)

// AgentClient abstracts the subset of agentclient.CRUDClient methods the
// Router needs. Tests substitute a fake; production code uses the SSH-backed
// implementation.
type AgentClient interface {
	List() ([]session.Metadata, error)
	Create(req agentserver.CreateRequest) (session.Metadata, error)
	Edit(req agentserver.EditRequest) (session.Metadata, error)
	Clone(sourceID, name string) (session.Metadata, error)
	Delete(id string) error
	Kill(id string) error
	Background(id string) error
	Override(id string) error
	Restart(id string) error
	Close() error
}

// AgentRef binds an AgentRecord to its live client. The Router never owns
// the lifecycle of the underlying connection — callers dial once at startup
// and pass the result here.
type AgentRef struct {
	Record agentclient.AgentRecord
	Client AgentClient
}

// LocalManager is the subset of session.Manager surface the Router needs.
// Kept as an interface so the Router package doesn't have to import the
// parts of session we never use and tests don't have to stand up a real
// Manager on disk.
type LocalManager interface {
	List() ([]session.Metadata, error)
	Get(id string) (session.Metadata, error)
	Delete(id string) error
	OverrideLock(id string) error
	UpdateWithOptions(id string, opts session.UpdateOptions) (session.Metadata, error)
	CreateWithOptions(name string, opts session.CreateOptions) (session.Metadata, error)
	Clone(sourceID, name string) (session.Metadata, error)
	IsLocked(id string) bool
}

// Router dispatches session CRUD ops across the local Manager and any
// registered agents. An empty Agents slice reduces the Router to a thin
// forwarder around the local Manager — safe default when no multi-host
// peering has been set up yet.
type Router struct {
	Local  LocalManager
	Agents []AgentRef

	// LocalKill/Background/Restart run local docker operations and must be
	// supplied by the caller (they live in the container package, which
	// this package deliberately avoids importing to keep compilation
	// lightweight).
	LocalKill       func(id string) error
	LocalBackground func(id string) error
	LocalRestart    func(id string) error
	LocalIsRunning  func(id string) bool

	mu      sync.RWMutex
	routing map[string]*AgentRef // session UUID -> owning agent; missing = local
}

// List returns the combined metadata for every local and remote session.
// Remote entries have AgentID + AgentHost stamped in so the TUI can
// distinguish them and the router can route subsequent ops by ID.
func (r *Router) List() ([]session.Metadata, error) {
	localSessions, err := r.Local.List()
	if err != nil {
		return nil, fmt.Errorf("local list: %w", err)
	}

	newRouting := make(map[string]*AgentRef, len(r.Agents)*4)
	all := make([]session.Metadata, 0, len(localSessions))
	all = append(all, localSessions...)

	for i := range r.Agents {
		ref := &r.Agents[i]
		remote, err := ref.Client.List()
		if err != nil {
			// One broken agent shouldn't black-hole the whole view. Skip it
			// and surface a sentinel row so the operator knows something is
			// wrong. The sentinel carries the agent ID + the error text in
			// Name so it shows up in the UI.
			all = append(all, session.Metadata{
				UUID:      "agent-unreachable:" + ref.Record.ID,
				Name:      fmt.Sprintf("[agent %s unreachable: %v]", ref.Record.ID, err),
				AgentID:   ref.Record.ID,
				AgentHost: ref.Record.Host,
			})
			continue
		}
		for j := range remote {
			remote[j].AgentID = ref.Record.ID
			remote[j].AgentHost = ref.Record.Host
			newRouting[remote[j].UUID] = ref
		}
		all = append(all, remote...)
	}

	r.mu.Lock()
	r.routing = newRouting
	r.mu.Unlock()
	return all, nil
}

// agentFor returns the registered agent for id, or nil if the id routes
// locally (default when unknown).
func (r *Router) agentFor(id string) *AgentRef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.routing[id]
}

// IsLocked reports lock state. Remote sessions always report false — the
// agent manages its own locks and we don't surface them to the TUI today.
func (r *Router) IsLocked(id string) bool {
	if a := r.agentFor(id); a != nil {
		return false
	}
	return r.Local.IsLocked(id)
}

// IsRunning delegates to the local container runtime for local sessions.
// Remote sessions return false because the shell has no direct view of the
// agent's docker daemon today; the status emitter will eventually feed a
// shell-side cache.
func (r *Router) IsRunning(id string) bool {
	if a := r.agentFor(id); a != nil {
		_ = a
		return false
	}
	if r.LocalIsRunning != nil {
		return r.LocalIsRunning(id)
	}
	return false
}

// OverrideLock works for local sessions; remote override is an explicit
// no-op (agents manage their own locks).
func (r *Router) OverrideLock(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Override(id)
	}
	return r.Local.OverrideLock(id)
}

// Kill routes via the agent for remote sessions; falls back to the
// container helper for local.
func (r *Router) Kill(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Kill(id)
	}
	if r.LocalKill == nil {
		return fmt.Errorf("local kill handler not configured")
	}
	return r.LocalKill(id)
}

func (r *Router) Background(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Background(id)
	}
	if r.LocalBackground == nil {
		return fmt.Errorf("local background handler not configured")
	}
	return r.LocalBackground(id)
}

func (r *Router) Restart(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Restart(id)
	}
	if r.LocalRestart == nil {
		return fmt.Errorf("local restart handler not configured")
	}
	return r.LocalRestart(id)
}

// Delete routes by ID; remote deletes go through the agent's CRUD op.
func (r *Router) Delete(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Delete(id)
	}
	return r.Local.Delete(id)
}

// Update routes edit ops. For remote sessions the shell uses agentserver's
// EditRequest shape; for local it updates via the Manager.
func (r *Router) Update(id, name, protocol, dnsName string, port int) error {
	if a := r.agentFor(id); a != nil {
		_, err := a.Client.Edit(agentserver.EditRequest{
			ID:       id,
			Name:     name,
			Port:     port,
			Protocol: protocol,
			DNSName:  dnsName,
		})
		return err
	}
	_, err := r.Local.UpdateWithOptions(id, session.UpdateOptions{
		Name:     name,
		Port:     port,
		Protocol: protocol,
		DNSName:  dnsName,
	})
	return err
}

// Clone always creates the new session on the same host as the source. A
// remote clone invokes the agent's Clone op; the result stays on that
// agent. Cross-host clones aren't supported — they'd require copying
// session state between hosts, which isn't in scope for v1.0.0.
func (r *Router) Clone(sourceID, name string) (session.Metadata, error) {
	if a := r.agentFor(sourceID); a != nil {
		meta, err := a.Client.Clone(sourceID, name)
		if err != nil {
			return meta, err
		}
		meta.AgentID = a.Record.ID
		meta.AgentHost = a.Record.Host
		return meta, nil
	}
	return r.Local.Clone(sourceID, name)
}
