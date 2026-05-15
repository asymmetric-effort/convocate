package redis

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// ConnConfig holds connection parameters for a Redis TLS connection.
type ConnConfig struct {
	Address   string
	TLSConfig *tls.Config
	Timeout   time.Duration
}

// Conn is a single Redis connection with RESP3 protocol support.
// It is not safe for concurrent use from multiple goroutines — callers
// must serialize access or use a pool.
type Conn struct {
	conn    net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	mu      sync.Mutex
	closed  bool
}

// Dial establishes a TLS connection to the Redis server.
func Dial(config ConnConfig) (*Conn, error) {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.TLSConfig == nil {
		return nil, fmt.Errorf("redis: TLSConfig is required (TLS v1.3+)")
	}
	if config.TLSConfig.MinVersion == 0 {
		config.TLSConfig.MinVersion = tls.VersionTLS13
	}

	dialer := &net.Dialer{Timeout: config.Timeout}
	rawConn, err := tls.DialWithDialer(dialer, "tcp", config.Address, config.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("redis: dial %s: %w", config.Address, err)
	}

	c := &Conn{
		conn:   rawConn,
		reader: bufio.NewReader(rawConn),
		writer: bufio.NewWriter(rawConn),
	}
	return c, nil
}

// Do sends a command and reads the response. It holds the connection lock
// for the entire round-trip.
func (c *Conn) Do(args ...string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("redis: connection closed")
	}

	err := WriteCommand(c.writer, args...)
	if err != nil {
		return nil, fmt.Errorf("redis: write command: %w", err)
	}
	err = c.writer.Flush()
	if err != nil {
		return nil, fmt.Errorf("redis: flush: %w", err)
	}
	return ReadResponse(c.reader)
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.conn.Close()
}

// String helper: reads a RESP response as a string, returning "" for nil.
func String(val interface{}, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("redis: expected string, got %T", val)
	}
	return s, nil
}

// Int64 helper: reads a RESP response as an int64.
func Int64(val interface{}, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, fmt.Errorf("redis: nil response")
	}
	n, ok := val.(int64)
	if !ok {
		return 0, fmt.Errorf("redis: expected int64, got %T", val)
	}
	return n, nil
}

// Strings helper: reads a RESP array response as a []string.
func Strings(val interface{}, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil, fmt.Errorf("redis: expected array, got %T", val)
	}
	result := make([]string, len(arr))
	for i, elem := range arr {
		if elem == nil {
			result[i] = ""
			continue
		}
		s, strOk := elem.(string)
		if !strOk {
			return nil, fmt.Errorf("redis: array element %d: expected string, got %T", i, elem)
		}
		result[i] = s
	}
	return result, nil
}

// Bool helper: reads a RESP integer response as a bool (1=true, 0=false).
func Bool(val interface{}, err error) (bool, error) {
	n, intErr := Int64(val, err)
	if intErr != nil {
		return false, intErr
	}
	return n != 0, nil
}
