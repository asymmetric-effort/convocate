package redis

import (
	"encoding/json"
	"fmt"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// DispatchStore provides the per-host Dispatch Service Redis namespace.
// Each Dispatch Service is the single writer for its own host's keys
// and never reads or writes the Router API namespace.
const dispatchPrefix = "dispatch:"

// DispatchStore wraps a Conn and prefixes all keys with "dispatch:<hostID>:".
type DispatchStore struct {
	conn   Doer
	hostID string
}

// NewDispatchStore creates a DispatchStore for a specific host.
func NewDispatchStore(conn Doer, hostID string) *DispatchStore {
	return &DispatchStore{conn: conn, hostID: hostID}
}

func (s *DispatchStore) key(parts ...string) string {
	result := dispatchPrefix + s.hostID
	for _, part := range parts {
		result += ":" + part
	}
	return result
}

// DispatchJobState holds the in-flight lifecycle state of a job on this host.
type DispatchJobState struct {
	ContainerID string            `json:"container_id"`
	State       protocol.JobState `json:"state"`
	Repository  string            `json:"repository"`
	IssueNumber int               `json:"issue_number"`
	JobID       uuid.UUID         `json:"job_id"`
}

// SetJobState writes the in-flight state for a job.
func (s *DispatchStore) SetJobState(state DispatchJobState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("redis/dispatch: marshal job state: %w", err)
	}
	_, err = s.conn.Do("SET", s.key("job", state.JobID.String()), string(data))
	return err
}

// GetJobState reads the in-flight state for a job.
func (s *DispatchStore) GetJobState(jobID uuid.UUID) (*DispatchJobState, error) {
	val, err := String(s.conn.Do("GET", s.key("job", jobID.String())))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var state DispatchJobState
	err = json.Unmarshal([]byte(val), &state)
	if err != nil {
		return nil, fmt.Errorf("redis/dispatch: unmarshal job state: %w", err)
	}
	return &state, nil
}

// DeleteJobState removes the in-flight state for a job.
func (s *DispatchStore) DeleteJobState(jobID uuid.UUID) error {
	_, err := s.conn.Do("DEL", s.key("job", jobID.String()))
	return err
}

// EnqueueDispatch adds a dispatch event to the host's queue.
func (s *DispatchStore) EnqueueDispatch(event *protocol.DispatchEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("redis/dispatch: marshal dispatch event: %w", err)
	}
	_, err = s.conn.Do("RPUSH", s.key("queue"), string(data))
	return err
}

// DequeueDispatch pops the next dispatch event from the host's queue.
// Returns nil if the queue is empty.
func (s *DispatchStore) DequeueDispatch() (*protocol.DispatchEvent, error) {
	val, err := String(s.conn.Do("LPOP", s.key("queue")))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var event protocol.DispatchEvent
	err = json.Unmarshal([]byte(val), &event)
	if err != nil {
		return nil, fmt.Errorf("redis/dispatch: unmarshal dispatch event: %w", err)
	}
	return &event, nil
}

// QueueLength returns the number of pending dispatch events for this host.
func (s *DispatchStore) QueueLength() (int64, error) {
	return Int64(s.conn.Do("LLEN", s.key("queue")))
}

// Ping checks the connection is alive.
func (s *DispatchStore) Ping() error {
	val, err := String(s.conn.Do("PING"))
	if err != nil {
		return err
	}
	if val != pong {
		return fmt.Errorf("redis/dispatch: unexpected PING response: %q", val)
	}
	return nil
}

// FlushNamespace deletes all keys in this host's dispatch namespace.
// WARNING: destructive. Intended only for testing.
func (s *DispatchStore) FlushNamespace() error {
	prefix := s.key("")
	cursor := "0"
	for {
		result, err := s.conn.Do("SCAN", cursor, "MATCH", prefix+"*", "COUNT", "100")
		if err != nil {
			return err
		}
		arr, ok := result.([]interface{})
		if !ok || len(arr) != 2 {
			return fmt.Errorf("redis/dispatch: unexpected SCAN result type")
		}
		nextCursor, ok := arr[0].(string)
		if !ok {
			return fmt.Errorf("redis/dispatch: unexpected SCAN cursor type")
		}
		keys, ok := arr[1].([]interface{})
		if !ok {
			return fmt.Errorf("redis/dispatch: unexpected SCAN keys type")
		}
		for _, keyIface := range keys {
			keyStr, strOk := keyIface.(string)
			if !strOk {
				continue
			}
			_, delErr := s.conn.Do("DEL", keyStr)
			if delErr != nil {
				return delErr
			}
		}
		cursor = nextCursor
		if cursor == "0" {
			break
		}
	}
	return nil
}
