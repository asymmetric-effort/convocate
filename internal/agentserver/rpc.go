package agentserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// readerFromBytes wraps a byte slice as an io.Reader. Used by decodeStrict
// so json.NewDecoder can disable unknown-field permissiveness.
func readerFromBytes(b []byte) io.Reader { return bytes.NewReader(b) }

// Request is the shape of a JSON-RPC request over claude-agent-rpc.
type Request struct {
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
}

// Response is the envelope written back to the client.
type Response struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// OpHandler runs one op. The raw params let each handler decode its own
// strongly-typed input struct.
type OpHandler func(params json.RawMessage) (any, error)

// Dispatcher routes op names to handlers. Only ops registered here can run;
// anything else is refused with an "unknown op" error.
type Dispatcher struct {
	ops map[string]OpHandler
}

// NewDispatcher builds an empty dispatcher. Callers register ops via Register.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{ops: map[string]OpHandler{}}
}

// Register adds an op handler. Duplicate registration panics — the op table
// is constructed at startup and should always be deterministic.
func (d *Dispatcher) Register(op string, h OpHandler) {
	if _, exists := d.ops[op]; exists {
		panic(fmt.Sprintf("agentserver: op %q already registered", op))
	}
	d.ops[op] = h
}

// Ops returns the list of registered op names, sorted for stable output.
func (d *Dispatcher) Ops() []string {
	names := make([]string, 0, len(d.ops))
	for k := range d.ops {
		names = append(names, k)
	}
	// Simple in-place sort — no need to pull in sort package just for this.
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}

// errUnknownOp is the canonical error when the client asked for something
// not in the op table.
var errUnknownOp = errors.New("unknown op")

// Handle reads a single request from r, runs it, writes a single response to
// w, and returns. The SSH subsystem owner is responsible for closing the
// channel after Handle returns.
func (d *Dispatcher) Handle(r io.Reader, w io.Writer) {
	var req Request
	dec := json.NewDecoder(r)
	if err := dec.Decode(&req); err != nil {
		writeResponse(w, Response{OK: false, Error: fmt.Sprintf("malformed request: %v", err)})
		return
	}
	handler, ok := d.ops[req.Op]
	if !ok {
		writeResponse(w, Response{OK: false, Error: fmt.Sprintf("%s: %q", errUnknownOp, req.Op)})
		return
	}
	result, err := handler(req.Params)
	if err != nil {
		writeResponse(w, Response{OK: false, Error: err.Error()})
		return
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		writeResponse(w, Response{OK: false, Error: fmt.Sprintf("marshal result: %v", err)})
		return
	}
	writeResponse(w, Response{OK: true, Result: resultBytes})
}

func writeResponse(w io.Writer, resp Response) {
	// json.NewEncoder already appends a newline which keeps streams framed for
	// clients that read line-by-line.
	_ = json.NewEncoder(w).Encode(resp)
}
