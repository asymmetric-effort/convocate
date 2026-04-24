package agentserver

import (
	"encoding/json"
	"fmt"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/container"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/statusproto"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

// StatusPublisher is a minimal hook interface so the orchestrator can emit
// status events to the shell without importing agentclient directly. Nil
// publishers are fine — the orchestrator no-ops emission.
type StatusPublisher interface {
	Publish(statusproto.Event)
}

// Orchestrator is the agent's grip on the host's container lifecycle. It
// wraps the session Manager plus the deps needed to construct a Runner when
// a restart/create op asks for one. Exposed as an interface primarily so
// tests can substitute a fake.
type Orchestrator interface {
	List() ([]session.Metadata, error)
	Get(id string) (session.Metadata, error)
	Create(opts session.CreateOptions, name string) (session.Metadata, error)
	Update(id string, opts session.UpdateOptions) (session.Metadata, error)
	Clone(sourceID, newName string) (session.Metadata, error)
	Delete(id string) error
	OverrideLock(id string) error
	Kill(id string) error
	Background(id string) error
	Restart(id string) error
}

// SessionOrchestrator is the production Orchestrator, wired to session.Manager
// and the container package. DNSServer is appended to every container started
// via Restart so resolution flows through the host's dnsmasq.
type SessionOrchestrator struct {
	Mgr       *session.Manager
	User      user.Info
	Paths     config.Paths
	DNSServer string
	AgentID   string

	// Publisher receives a status event for each lifecycle action. nil = no
	// emission (useful in tests that don't care about events).
	Publisher StatusPublisher

	// Stop / DetachClients are overridable for tests; default to the
	// production docker-shelling helpers.
	StopFn   func(sessionID string) error
	DetachFn func(sessionID string) error

	// NewRunner is overridable for tests.
	NewRunner func(sessionID, sessionDir string, u user.Info, p config.Paths) *container.Runner
}

// NewSessionOrchestrator returns an Orchestrator with production defaults.
// Publisher can be nil — when unset, CRUD ops run identically but emit no
// status events. AgentID is embedded in every event so the shell can route
// to the right per-agent log file.
func NewSessionOrchestrator(mgr *session.Manager, u user.Info, p config.Paths, dnsServer, agentID string, pub StatusPublisher) *SessionOrchestrator {
	return &SessionOrchestrator{
		Mgr:       mgr,
		User:      u,
		Paths:     p,
		DNSServer: dnsServer,
		AgentID:   agentID,
		Publisher: pub,
		StopFn:    container.StopContainer,
		DetachFn:  container.DetachClients,
		NewRunner: container.NewRunner,
	}
}

func (o *SessionOrchestrator) List() ([]session.Metadata, error) { return o.Mgr.List() }
func (o *SessionOrchestrator) Get(id string) (session.Metadata, error) {
	return o.Mgr.Get(id)
}
func (o *SessionOrchestrator) Create(opts session.CreateOptions, name string) (session.Metadata, error) {
	meta, err := o.Mgr.CreateWithOptions(name, opts)
	if err == nil {
		o.emit(statusproto.TypeContainerCreated, meta.UUID, metaData(meta))
	}
	return meta, err
}
func (o *SessionOrchestrator) Update(id string, opts session.UpdateOptions) (session.Metadata, error) {
	meta, err := o.Mgr.UpdateWithOptions(id, opts)
	if err == nil {
		o.emit(statusproto.TypeContainerEdited, id, metaData(meta))
	}
	return meta, err
}
func (o *SessionOrchestrator) Clone(sourceID, newName string) (session.Metadata, error) {
	meta, err := o.Mgr.Clone(sourceID, newName)
	if err == nil {
		o.emit(statusproto.TypeContainerCreated, meta.UUID, metaData(meta))
	}
	return meta, err
}
func (o *SessionOrchestrator) Delete(id string) error {
	err := o.Mgr.Delete(id)
	if err == nil {
		o.emit(statusproto.TypeContainerDeleted, id, nil)
	}
	return err
}
func (o *SessionOrchestrator) OverrideLock(id string) error { return o.Mgr.OverrideLock(id) }
func (o *SessionOrchestrator) Kill(id string) error {
	err := o.StopFn(id)
	if err == nil {
		o.emit(statusproto.TypeContainerStopped, id, nil)
	}
	return err
}
func (o *SessionOrchestrator) Background(id string) error { return o.DetachFn(id) }

// Restart starts the container in detached mode without attaching a
// terminal. Mirrors the claude-host-side restartSessionDetached flow: refuse
// if the session doesn't exist or is already running, otherwise docker run
// with the session's persisted port/protocol/dns configuration.
func (o *SessionOrchestrator) Restart(id string) error {
	meta, err := o.Mgr.Get(id)
	if err != nil {
		return err
	}
	if err := o.Mgr.Touch(id); err != nil {
		// Best-effort: carry on even if we can't update the timestamp.
		_ = err
	}
	r := o.NewRunner(id, o.Mgr.SessionDir(id), o.User, o.Paths)
	r.SetPort(meta.Port)
	r.SetProtocol(meta.EffectiveProtocol())
	r.SetDNSServer(o.DNSServer)

	running, err := r.IsRunning()
	if err != nil {
		return fmt.Errorf("check running: %w", err)
	}
	if running {
		return fmt.Errorf("session %q already running", meta.Name)
	}
	if err := r.StartDetached(); err != nil {
		return err
	}
	o.emit(statusproto.TypeContainerStarted, id, metaData(meta))
	return nil
}

// emit is the central hook: no-op when no publisher is wired, otherwise
// stamps the agent ID and publishes.
func (o *SessionOrchestrator) emit(typ, sessionID string, data json.RawMessage) {
	if o.Publisher == nil {
		return
	}
	ev := statusproto.NewEvent(typ, o.AgentID, sessionID)
	ev.Data = data
	o.Publisher.Publish(ev)
}

// metaData is a tiny convenience: json-encode a Metadata value for the
// Event.Data field. Returns nil on failure because Data is optional —
// emitting a status update with a missing data body still beats dropping
// the notification entirely.
func metaData(m session.Metadata) json.RawMessage {
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

// --- RPC request / response types ------------------------------------------

type IDRequest struct {
	ID string `json:"id"`
}

type CreateRequest struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	DNSName  string `json:"dns_name"`
}

type EditRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	DNSName  string `json:"dns_name"`
}

type CloneRequest struct {
	SourceID string `json:"source_id"`
	Name     string `json:"name"`
}

// --- registration ----------------------------------------------------------

// RegisterCRUDOps wires the container-CRUD ops onto dispatcher d. Callers
// must have already called RegisterCoreOps(d, ...) for ping/version.
func RegisterCRUDOps(d *Dispatcher, o Orchestrator) {
	d.Register("list", func(_ json.RawMessage) (any, error) {
		return o.List()
	})
	d.Register("get", func(p json.RawMessage) (any, error) {
		var req IDRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return o.Get(req.ID)
	})
	d.Register("create", func(p json.RawMessage) (any, error) {
		var req CreateRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return o.Create(session.CreateOptions{
			Port:     req.Port,
			Protocol: req.Protocol,
			DNSName:  req.DNSName,
		}, req.Name)
	})
	d.Register("edit", func(p json.RawMessage) (any, error) {
		var req EditRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return o.Update(req.ID, session.UpdateOptions{
			Name:     req.Name,
			Port:     req.Port,
			Protocol: req.Protocol,
			DNSName:  req.DNSName,
		})
	})
	d.Register("clone", func(p json.RawMessage) (any, error) {
		var req CloneRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return o.Clone(req.SourceID, req.Name)
	})
	d.Register("delete", func(p json.RawMessage) (any, error) {
		var req IDRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return struct{}{}, o.Delete(req.ID)
	})
	d.Register("kill", func(p json.RawMessage) (any, error) {
		var req IDRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return struct{}{}, o.Kill(req.ID)
	})
	d.Register("background", func(p json.RawMessage) (any, error) {
		var req IDRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return struct{}{}, o.Background(req.ID)
	})
	d.Register("override", func(p json.RawMessage) (any, error) {
		var req IDRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return struct{}{}, o.OverrideLock(req.ID)
	})
	d.Register("restart", func(p json.RawMessage) (any, error) {
		var req IDRequest
		if err := decodeStrict(p, &req); err != nil {
			return nil, err
		}
		return struct{}{}, o.Restart(req.ID)
	})
	// Settings placeholders — the shell doesn't have configurable settings
	// yet. These return empty responses so the client surface is stable.
	d.Register("settings-get", func(_ json.RawMessage) (any, error) {
		return struct{}{}, nil
	})
	d.Register("settings-set", func(_ json.RawMessage) (any, error) {
		return struct{}{}, nil
	})
}

// decodeStrict unmarshals into v and rejects unknown fields so a typo in a
// client request surfaces as an error instead of silently dropping data.
// Empty or null params decode cleanly into the zero value.
func decodeStrict(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	dec := json.NewDecoder(readerFromBytes(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode params: %w", err)
	}
	return nil
}
