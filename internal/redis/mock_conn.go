package redis

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// MockConn is an in-memory Redis mock for unit testing. It supports the
// subset of Redis commands used by RouterStore and DispatchStore.
type MockConn struct {
	mu      sync.Mutex
	data    map[string]string
	lists   map[string][]string
	sets    map[string]map[string]bool
	ttls    map[string]int
	closed  bool
}

// NewMockConn creates a new in-memory mock connection.
func NewMockConn() *MockConn {
	return &MockConn{
		data:  make(map[string]string),
		lists: make(map[string][]string),
		sets:  make(map[string]map[string]bool),
		ttls:  make(map[string]int),
	}
}

// toConn wraps the mock into a Conn-compatible interface by creating a Conn
// whose Do method is intercepted. Since Conn uses a real TCP connection, we
// instead provide a DoFunc-based adapter.
//
// For testing, callers use mockDo directly via RouterStore/DispatchStore
// constructors that accept an interface.

// Do executes a mock Redis command.
func (m *MockConn) Do(args ...string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("redis: connection closed")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("redis: empty command")
	}

	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "PING":
		return "PONG", nil

	case "SET":
		if len(args) < 3 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'set' command"}
		}
		m.data[args[1]] = args[2]
		// Handle EX option.
		for i := 3; i < len(args)-1; i++ {
			if strings.ToUpper(args[i]) == "EX" {
				ttl, parseErr := strconv.Atoi(args[i+1])
				if parseErr == nil {
					m.ttls[args[1]] = ttl
				}
			}
		}
		return "OK", nil

	case "GET":
		if len(args) < 2 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'get' command"}
		}
		val, exists := m.data[args[1]]
		if !exists {
			return nil, nil
		}
		return val, nil

	case "DEL":
		if len(args) < 2 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'del' command"}
		}
		deleted := int64(0)
		for _, key := range args[1:] {
			if _, exists := m.data[key]; exists {
				delete(m.data, key)
				delete(m.ttls, key)
				deleted++
			}
			if _, exists := m.lists[key]; exists {
				delete(m.lists, key)
				deleted++
			}
			if _, exists := m.sets[key]; exists {
				delete(m.sets, key)
				deleted++
			}
		}
		return deleted, nil

	case "SADD":
		if len(args) < 3 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'sadd' command"}
		}
		key := args[1]
		if m.sets[key] == nil {
			m.sets[key] = make(map[string]bool)
		}
		added := int64(0)
		for _, member := range args[2:] {
			if !m.sets[key][member] {
				m.sets[key][member] = true
				added++
			}
		}
		return added, nil

	case "SREM":
		if len(args) < 3 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'srem' command"}
		}
		key := args[1]
		removed := int64(0)
		if m.sets[key] != nil {
			for _, member := range args[2:] {
				if m.sets[key][member] {
					delete(m.sets[key], member)
					removed++
				}
			}
		}
		return removed, nil

	case "SISMEMBER":
		if len(args) < 3 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'sismember' command"}
		}
		key := args[1]
		member := args[2]
		if m.sets[key] != nil && m.sets[key][member] {
			return int64(1), nil
		}
		return int64(0), nil

	case "RPUSH":
		if len(args) < 3 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'rpush' command"}
		}
		key := args[1]
		m.lists[key] = append(m.lists[key], args[2:]...)
		return int64(len(m.lists[key])), nil

	case "LPOP":
		if len(args) < 2 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'lpop' command"}
		}
		key := args[1]
		list := m.lists[key]
		if len(list) == 0 {
			return nil, nil
		}
		val := list[0]
		m.lists[key] = list[1:]
		return val, nil

	case "LLEN":
		if len(args) < 2 {
			return nil, &RedisError{Message: "ERR wrong number of arguments for 'llen' command"}
		}
		key := args[1]
		return int64(len(m.lists[key])), nil

	case "SCAN":
		// Simplified SCAN: returns all matching keys in one pass.
		pattern := "*"
		for i := 1; i < len(args)-1; i++ {
			if strings.ToUpper(args[i]) == "MATCH" {
				pattern = args[i+1]
			}
		}
		var matched []interface{}
		for key := range m.data {
			if matchGlob(pattern, key) {
				matched = append(matched, key)
			}
		}
		// Return cursor "0" (scan complete) and the matched keys.
		return []interface{}{"0", matched}, nil

	default:
		return nil, &RedisError{Message: fmt.Sprintf("ERR unknown command '%s'", cmd)}
	}
}

// Close marks the mock as closed.
func (m *MockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// matchGlob does a simple glob match (only supports trailing *).
func matchGlob(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(s, prefix)
	}
	return pattern == s
}

// mockReader creates a bufio.Reader that yields RESP-formatted mock responses.
// Not used directly — the mock intercepts at the Do level.
func mockReader(responses string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(responses))
}
