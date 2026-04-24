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
// registered agents. As of v2.x, the Router no longer runs containers on
// the claude-shell host; Local's session metadata files are surfaced as
// read-only "orphan" entries pending migration to an agent, and every
// write op that targets an orphan returns ErrOrphanNeedsMigration.
type Router struct {
	Local  LocalManager
	Agents []AgentRef

	mu       sync.RWMutex
	routing  map[string]*AgentRef // session UUID -> owning agent; missing = orphan
	running  map[string]bool      // session UUID -> last-known running status from list response
	attached map[string]bool      // session UUID -> last-known attach state (any operator connected)
}

// ErrOrphanNeedsMigration is returned by every write op invoked against a
// session that lives in the local session.Manager but not on any
// registered agent. Operators see it in the TUI when they try to kill,
// restart, edit, clone, etc. a leftover local session from the pre-v2.0
// world.
var errOrphanNeedsMigration = fmt.Errorf("session is a local orphan; migrate it to a claude-agent before operating on it")

// List returns the combined metadata for every local and remote session.
// Remote entries have AgentID + AgentHost stamped in so the TUI can
// distinguish them and the router can route subsequent ops by ID.
func (r *Router) List() ([]session.Metadata, error) {
	localSessions, err := r.Local.List()
	if err != nil {
		return nil, fmt.Errorf("local list: %w", err)
	}

	newRouting := make(map[string]*AgentRef, len(r.Agents)*4)
	newRunning := make(map[string]bool, len(r.Agents)*4)
	newAttached := make(map[string]bool, len(r.Agents)*4)
	all := make([]session.Metadata, 0, len(localSessions))
	// Local sessions are orphans — the shell no longer runs containers
	// itself, so Running is always false and write ops are blocked until
	// the operator migrates them to a claude-agent.
	for i := range localSessions {
		localSessions[i].Running = false
		localSessions[i].Attached = false
		newRunning[localSessions[i].UUID] = false
		newAttached[localSessions[i].UUID] = false
	}
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
			newRunning[remote[j].UUID] = remote[j].Running
			newAttached[remote[j].UUID] = remote[j].Attached
		}
		all = append(all, remote...)
	}

	r.mu.Lock()
	r.routing = newRouting
	r.running = newRunning
	r.attached = newAttached
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

// AgentFor exposes agentFor to callers outside the package. Used by the
// claude-shell TUI to reach the right SSH connection when starting a
// remote attach.
func (r *Router) AgentFor(id string) *AgentRef { return r.agentFor(id) }

// IsLocked reports attach state (repurposed from v1 lock semantics):
// true when any operator is currently connected to the session's
// container via claude-agent-attach. The TUI reads this to render the
// "C" (connected) indicator — which now means "someone is attached
// right now" rather than the old "lockfile exists". Orphans always
// return false because attach isn't possible on them.
func (r *Router) IsLocked(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.attached[id]
}

// IsRunning reports the last-known running state recorded on the most
// recent List response. Orphan sessions always report false.
func (r *Router) IsRunning(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running[id]
}

// OverrideLock: remote sessions route to the agent's Override op;
// orphans reject (no local lock to override).
func (r *Router) OverrideLock(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Override(id)
	}
	return errOrphanNeedsMigration
}

// Kill routes via the agent for remote sessions; orphans reject.
func (r *Router) Kill(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Kill(id)
	}
	return errOrphanNeedsMigration
}

// Background detaches the user's attachment on the remote agent.
// Orphan sessions reject (no attachment exists to detach).
func (r *Router) Background(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Background(id)
	}
	return errOrphanNeedsMigration
}

// Restart routes via the agent for remote sessions; orphans reject.
func (r *Router) Restart(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Restart(id)
	}
	return errOrphanNeedsMigration
}

// Delete routes remote deletes through the agent. Orphan deletes go
// through the local Manager so the user can clean up stale metadata
// after migrating the container off the shell host — this is the one
// local write we still permit.
func (r *Router) Delete(id string) error {
	if a := r.agentFor(id); a != nil {
		return a.Client.Delete(id)
	}
	return r.Local.Delete(id)
}

// Update routes edit ops for remote sessions. Orphan edits are blocked —
// there's no point renaming a session that can't be operated on.
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
	return errOrphanNeedsMigration
}

// Create routes a new-session request to the agent identified by agentID.
// agentID is required — v2.x removed the local-create fallback because the
// shell host no longer runs containers.
func (r *Router) Create(agentID string, opts session.CreateOptions, name string) (session.Metadata, error) {
	if agentID == "" {
		return session.Metadata{}, fmt.Errorf("agent-id required: pick a target agent (create was removed from the shell host)")
	}
	for i := range r.Agents {
		a := &r.Agents[i]
		if a.Record.ID != agentID {
			continue
		}
		meta, err := a.Client.Create(agentserver.CreateRequest{
			Name:     name,
			Port:     opts.Port,
			Protocol: opts.Protocol,
			DNSName:  opts.DNSName,
		})
		if err != nil {
			return meta, err
		}
		meta.AgentID = a.Record.ID
		meta.AgentHost = a.Record.Host
		// Register the new UUID in the routing map so subsequent ops
		// route here without waiting for the next List refresh.
		r.mu.Lock()
		if r.routing == nil {
			r.routing = map[string]*AgentRef{}
		}
		r.routing[meta.UUID] = a
		r.mu.Unlock()
		return meta, nil
	}
	return session.Metadata{}, fmt.Errorf("agent %q not registered", agentID)
}

// AgentIDs returns the IDs of every registered agent in the order they
// were registered. Useful for UI dropdowns that need a stable,
// human-pickable list.
func (r *Router) AgentIDs() []string {
	out := make([]string, 0, len(r.Agents))
	for i := range r.Agents {
		out = append(out, r.Agents[i].Record.ID)
	}
	return out
}

// Clone duplicates a session on the agent that hosts the source. Cloning
// a local orphan is rejected — you can't clone something that isn't
// under active management.
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
	return session.Metadata{}, errOrphanNeedsMigration
}
