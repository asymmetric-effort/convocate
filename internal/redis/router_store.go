package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// RouterStore provides the Router API's authoritative Redis namespace.
// It is the single writer for container registry, project routing table,
// repository allowlist, job ledger, and job metadata.
const routerPrefix = "router:"

// RouterStore wraps a Conn and prefixes all keys with "router:".
type RouterStore struct {
	conn Doer
}

// NewRouterStore creates a RouterStore from an existing connection.
func NewRouterStore(conn Doer) *RouterStore {
	return &RouterStore{conn: conn}
}

func (s *RouterStore) key(parts ...string) string {
	result := routerPrefix
	for i, part := range parts {
		if i > 0 {
			result += ":"
		}
		result += part
	}
	return result
}

// --- Container Map ---

// SetContainer writes a container map entry.
func (s *RouterStore) SetContainer(entry *protocol.ContainerMapEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("redis/router: marshal container: %w", err)
	}
	_, err = s.conn.Do("SET", s.key("container", entry.ContainerID), string(data))
	return err
}

// GetContainer reads a container map entry by container ID.
func (s *RouterStore) GetContainer(containerID string) (*protocol.ContainerMapEntry, error) {
	val, err := String(s.conn.Do("GET", s.key("container", containerID)))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var entry protocol.ContainerMapEntry
	err = json.Unmarshal([]byte(val), &entry)
	if err != nil {
		return nil, fmt.Errorf("redis/router: unmarshal container: %w", err)
	}
	return &entry, nil
}

// DeleteContainer removes a container map entry.
func (s *RouterStore) DeleteContainer(containerID string) error {
	_, err := s.conn.Do("DEL", s.key("container", containerID))
	return err
}

// --- Project Routing Table ---

// SetRoute writes a project → (host, container) binding.
func (s *RouterStore) SetRoute(entry protocol.ProjectRouteEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("redis/router: marshal route: %w", err)
	}
	_, err = s.conn.Do("SET", s.key("route", entry.ProjectID.String()), string(data))
	if err != nil {
		return err
	}
	// Also index by repository for job submission lookups.
	_, err = s.conn.Do("SET", s.key("route-by-repo", entry.Repository), string(data))
	return err
}

// GetRoute reads a project route by project ID.
func (s *RouterStore) GetRoute(projectID uuid.UUID) (*protocol.ProjectRouteEntry, error) {
	val, err := String(s.conn.Do("GET", s.key("route", projectID.String())))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var entry protocol.ProjectRouteEntry
	err = json.Unmarshal([]byte(val), &entry)
	if err != nil {
		return nil, fmt.Errorf("redis/router: unmarshal route: %w", err)
	}
	return &entry, nil
}

// GetRouteByRepo reads a project route by repository full name.
func (s *RouterStore) GetRouteByRepo(repository string) (*protocol.ProjectRouteEntry, error) {
	val, err := String(s.conn.Do("GET", s.key("route-by-repo", repository)))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var entry protocol.ProjectRouteEntry
	err = json.Unmarshal([]byte(val), &entry)
	if err != nil {
		return nil, fmt.Errorf("redis/router: unmarshal route: %w", err)
	}
	return &entry, nil
}

// DeleteRoute removes a project route entry.
func (s *RouterStore) DeleteRoute(projectID uuid.UUID, repository string) error {
	_, err := s.conn.Do("DEL", s.key("route", projectID.String()))
	if err != nil {
		return err
	}
	_, err = s.conn.Do("DEL", s.key("route-by-repo", repository))
	return err
}

// --- Repository Allowlist ---

// AllowlistAdd adds a repository to the allowlist.
func (s *RouterStore) AllowlistAdd(repository string) error {
	_, err := s.conn.Do("SADD", s.key("allowlist"), repository)
	return err
}

// AllowlistRemove removes a repository from the allowlist.
func (s *RouterStore) AllowlistRemove(repository string) error {
	_, err := s.conn.Do("SREM", s.key("allowlist"), repository)
	return err
}

// AllowlistContains checks whether a repository is in the allowlist.
func (s *RouterStore) AllowlistContains(repository string) (bool, error) {
	return Bool(s.conn.Do("SISMEMBER", s.key("allowlist"), repository))
}

// --- Job Ledger ---

// RecordJob writes a job to the ledger with its idempotency key.
func (s *RouterStore) RecordJob(idempotencyKey protocol.IdempotencyKey, jobID uuid.UUID) error {
	_, err := s.conn.Do("SET", s.key("ledger", idempotencyKey.String()), jobID.String())
	return err
}

// LookupJobByKey looks up a job ID by idempotency key. Returns zero UUID if
// not found.
func (s *RouterStore) LookupJobByKey(idempotencyKey protocol.IdempotencyKey) (uuid.UUID, error) {
	val, err := String(s.conn.Do("GET", s.key("ledger", idempotencyKey.String())))
	if err != nil {
		return uuid.UUID{}, err
	}
	if val == "" {
		return uuid.UUID{}, nil
	}
	return uuid.Parse(val)
}

// --- Job Metadata ---

// SetJobMetadata writes job metadata.
func (s *RouterStore) SetJobMetadata(meta *protocol.JobMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("redis/router: marshal job metadata: %w", err)
	}
	_, err = s.conn.Do("SET", s.key("job", meta.JobID.String()), string(data))
	return err
}

// GetJobMetadata reads job metadata by job ID.
func (s *RouterStore) GetJobMetadata(jobID uuid.UUID) (*protocol.JobMetadata, error) {
	val, err := String(s.conn.Do("GET", s.key("job", jobID.String())))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var meta protocol.JobMetadata
	err = json.Unmarshal([]byte(val), &meta)
	if err != nil {
		return nil, fmt.Errorf("redis/router: unmarshal job metadata: %w", err)
	}
	return &meta, nil
}

// DeleteJobMetadata removes job metadata.
func (s *RouterStore) DeleteJobMetadata(jobID uuid.UUID) error {
	_, err := s.conn.Do("DEL", s.key("job", jobID.String()))
	return err
}

// --- API Token Registry ---

// SetAPIToken stores a CONVOCATE_API_TOKEN for a repository.
func (s *RouterStore) SetAPIToken(repository, token string) error {
	_, err := s.conn.Do("SET", s.key("token", repository), token)
	return err
}

// GetAPIToken reads a CONVOCATE_API_TOKEN for a repository.
func (s *RouterStore) GetAPIToken(repository string) (string, error) {
	return String(s.conn.Do("GET", s.key("token", repository)))
}

// ValidateAPIToken checks that a token matches the stored one for a repository.
func (s *RouterStore) ValidateAPIToken(repository, token string) (bool, error) {
	stored, err := s.GetAPIToken(repository)
	if err != nil {
		return false, err
	}
	return stored != "" && stored == token, nil
}

// DeleteAPIToken removes a CONVOCATE_API_TOKEN for a repository.
func (s *RouterStore) DeleteAPIToken(repository string) error {
	_, err := s.conn.Do("DEL", s.key("token", repository))
	return err
}

// --- Heartbeat Cache ---

// CacheHeartbeat stores the latest heartbeat for a host.
func (s *RouterStore) CacheHeartbeat(heartbeat protocol.HeartbeatRequest) error {
	data, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("redis/router: marshal heartbeat: %w", err)
	}
	// Set with a 60-second TTL (4x the 15-second interval).
	_, err = s.conn.Do("SET", s.key("heartbeat", heartbeat.HostID), string(data), "EX", "60")
	return err
}

// GetHeartbeat reads the latest heartbeat for a host.
func (s *RouterStore) GetHeartbeat(hostID string) (*protocol.HeartbeatRequest, error) {
	val, err := String(s.conn.Do("GET", s.key("heartbeat", hostID)))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var heartbeat protocol.HeartbeatRequest
	err = json.Unmarshal([]byte(val), &heartbeat)
	if err != nil {
		return nil, fmt.Errorf("redis/router: unmarshal heartbeat: %w", err)
	}
	return &heartbeat, nil
}

// --- Project Info ---

// SetProjectInfo stores project info.
func (s *RouterStore) SetProjectInfo(info *protocol.ProjectInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("redis/router: marshal project info: %w", err)
	}
	_, err = s.conn.Do("SET", s.key("project", info.ProjectID.String()), string(data))
	if err != nil {
		return err
	}
	// Index by repository.
	_, err = s.conn.Do("SET", s.key("project-by-repo", info.Repository), info.ProjectID.String())
	return err
}

// GetProjectInfo reads project info by project ID.
func (s *RouterStore) GetProjectInfo(projectID uuid.UUID) (*protocol.ProjectInfo, error) {
	val, err := String(s.conn.Do("GET", s.key("project", projectID.String())))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var info protocol.ProjectInfo
	err = json.Unmarshal([]byte(val), &info)
	if err != nil {
		return nil, fmt.Errorf("redis/router: unmarshal project info: %w", err)
	}
	return &info, nil
}

// GetProjectIDByRepo reads a project ID by repository full name.
func (s *RouterStore) GetProjectIDByRepo(repository string) (uuid.UUID, error) {
	val, err := String(s.conn.Do("GET", s.key("project-by-repo", repository)))
	if err != nil {
		return uuid.UUID{}, err
	}
	if val == "" {
		return uuid.UUID{}, nil
	}
	return uuid.Parse(val)
}

// DeleteProjectInfo removes project info.
func (s *RouterStore) DeleteProjectInfo(projectID uuid.UUID, repository string) error {
	_, err := s.conn.Do("DEL", s.key("project", projectID.String()))
	if err != nil {
		return err
	}
	_, err = s.conn.Do("DEL", s.key("project-by-repo", repository))
	return err
}

// Ping checks the connection is alive.
func (s *RouterStore) Ping() error {
	val, err := String(s.conn.Do("PING"))
	if err != nil {
		return err
	}
	if val != pong {
		return fmt.Errorf("redis/router: unexpected PING response: %q", val)
	}
	return nil
}

// FlushNamespace deletes all keys in the router namespace. WARNING: destructive.
// Intended only for testing.
func (s *RouterStore) FlushNamespace() error {
	return s.flushByPrefix(routerPrefix)
}

func (s *RouterStore) flushByPrefix(prefix string) error {
	cursor := "0"
	for {
		result, err := s.conn.Do("SCAN", cursor, "MATCH", prefix+"*", "COUNT", "100")
		if err != nil {
			return err
		}
		arr, ok := result.([]interface{})
		if !ok || len(arr) != 2 {
			return fmt.Errorf("redis/router: unexpected SCAN result type")
		}
		nextCursor, ok := arr[0].(string)
		if !ok {
			return fmt.Errorf("redis/router: unexpected SCAN cursor type")
		}
		keys, ok := arr[1].([]interface{})
		if !ok {
			return fmt.Errorf("redis/router: unexpected SCAN keys type")
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

// --- Cluster Auth ---

// SetClusterAuth stores the cluster-wide Claude authentication.
func (s *RouterStore) SetClusterAuth(mode protocol.ClusterAuthMode, credential string) error {
	_, err := s.conn.Do("SET", s.key("cluster-auth-mode"), string(mode))
	if err != nil {
		return err
	}
	_, err = s.conn.Do("SET", s.key("cluster-auth-credential"), credential)
	return err
}

// GetClusterAuth reads the cluster-wide Claude authentication.
func (s *RouterStore) GetClusterAuth() (protocol.ClusterAuthMode, string, error) {
	mode, err := String(s.conn.Do("GET", s.key("cluster-auth-mode")))
	if err != nil {
		return "", "", err
	}
	credential, err := String(s.conn.Do("GET", s.key("cluster-auth-credential")))
	if err != nil {
		return "", "", err
	}
	return protocol.ClusterAuthMode(mode), credential, nil
}

// DeleteClusterAuth removes the cluster-wide Claude authentication.
func (s *RouterStore) DeleteClusterAuth() error {
	_, err := s.conn.Do("DEL", s.key("cluster-auth-mode"))
	if err != nil {
		return err
	}
	_, err = s.conn.Do("DEL", s.key("cluster-auth-credential"))
	return err
}

// CountContainersByHost counts containers assigned to a given host.
func (s *RouterStore) CountContainersByHost(hostID string) (int, error) {
	// This scans containers — acceptable for MVP since container counts are small.
	count := 0
	cursor := "0"
	for {
		result, err := s.conn.Do("SCAN", cursor, "MATCH", s.key("container", "*"), "COUNT", "100")
		if err != nil {
			return 0, err
		}
		arr, ok := result.([]interface{})
		if !ok || len(arr) != 2 {
			return 0, fmt.Errorf("redis/router: unexpected SCAN result type")
		}
		nextCursor, ok := arr[0].(string)
		if !ok {
			return 0, fmt.Errorf("redis/router: unexpected SCAN cursor type")
		}
		keys, ok := arr[1].([]interface{})
		if !ok {
			return 0, fmt.Errorf("redis/router: unexpected SCAN keys type")
		}
		for _, keyIface := range keys {
			keyStr, strOk := keyIface.(string)
			if !strOk {
				continue
			}
			val, getErr := String(s.conn.Do("GET", keyStr))
			if getErr != nil || val == "" {
				continue
			}
			var entry protocol.ContainerMapEntry
			if unmarshalErr := json.Unmarshal([]byte(val), &entry); unmarshalErr != nil {
				continue
			}
			if entry.HostID == hostID {
				count++
			}
		}
		cursor = nextCursor
		if cursor == "0" {
			break
		}
	}
	return count, nil
}

// Now returns the current time for timestamping. Exposed as a method so
// tests can substitute a fixed clock if needed.
func (s *RouterStore) Now() time.Time {
	return time.Now()
}
