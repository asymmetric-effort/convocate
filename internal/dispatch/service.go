package dispatch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// Service is the per-host Dispatch Service.
type Service struct {
	containerMgr ContainerManager
	httpClient   *http.Client
	store        *redis.DispatchStore
	logger       *log.Logger
	activeJobs   map[uuid.UUID]context.CancelFunc
	stopCh       chan struct{}
	hostID       string
	controlURL   string
	mu           sync.RWMutex
}

// Config holds the Dispatch Service configuration.
type Config struct {
	ContainerMgr ContainerManager
	HTTPClient   *http.Client
	Store        *redis.DispatchStore
	Logger       *log.Logger
	HostID       string
	ControlURL   string
}

// ContainerManager is the interface the Dispatch Service uses to manage
// Agent Containers. In production this wraps the Docker SDK; in tests
// it's a mock.
type ContainerManager interface {
	// DeliverPrompt delivers a prompt to the given container as a
	// background task. Non-blocking.
	DeliverPrompt(containerID string, jobID uuid.UUID, prompt string) error

	// StopContainer stops and removes a container.
	StopContainer(containerID string) error

	// ProvisionContainer creates a new Agent Container.
	ProvisionContainer(containerID string, projectID uuid.UUID) error

	// IsHealthy checks whether a container is running and responsive.
	IsHealthy(containerID string) bool

	// TerminateTask signals the wrapper to abort a specific background task.
	TerminateTask(containerID string, jobID uuid.UUID) error

	// ContainerCount returns the number of running containers.
	ContainerCount() int

	// CPUPercent returns the aggregate CPU usage percentage.
	CPUPercent() float64

	// MemoryPercent returns the aggregate memory usage percentage.
	MemoryPercent() float64
}

// NewService creates a new Dispatch Service.
func NewService(config Config) (*Service, error) {
	if config.HostID == "" {
		return nil, fmt.Errorf("dispatch: host ID is required")
	}
	if config.ControlURL == "" {
		return nil, fmt.Errorf("dispatch: control URL is required")
	}
	if config.Store == nil {
		return nil, fmt.Errorf("dispatch: Redis store is required")
	}
	if config.ContainerMgr == nil {
		return nil, fmt.Errorf("dispatch: container manager is required")
	}
	logger := config.Logger
	if logger == nil {
		logger = log.Default()
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Service{
		hostID:       config.HostID,
		controlURL:   config.ControlURL,
		httpClient:   httpClient,
		store:        config.Store,
		containerMgr: config.ContainerMgr,
		logger:       logger,
		activeJobs:   make(map[uuid.UUID]context.CancelFunc),
		stopCh:       make(chan struct{}),
	}, nil
}

// HandleDispatchEvent processes a single dispatch event from the Router API.
func (s *Service) HandleDispatchEvent(event *protocol.DispatchEvent) error {
	s.logger.Printf("dispatch: handling job %s for container %s", event.JobID, event.ContainerID)

	// Record in dispatch namespace.
	err := s.store.SetJobState(redis.DispatchJobState{
		JobID:       event.JobID,
		ContainerID: event.ContainerID,
		State:       protocol.JobClaimed,
		Repository:  event.Repository,
		IssueNumber: event.IssueNumber,
	})
	if err != nil {
		return fmt.Errorf("dispatch: record job state: %w", err)
	}

	// Check container health.
	if !s.containerMgr.IsHealthy(event.ContainerID) {
		s.logger.Printf("dispatch: container %s unhealthy, rejecting dispatch", event.ContainerID)
		s.ReportStatus(event.JobID, event.ContainerID, protocol.JobClaimed, protocol.JobFailed, "container unhealthy")
		return fmt.Errorf("dispatch: container %s unhealthy", event.ContainerID)
	}

	// Build the prompt.
	prompt := buildPrompt(event)

	// Deliver the prompt.
	err = s.containerMgr.DeliverPrompt(event.ContainerID, event.JobID, prompt)
	if err != nil {
		s.logger.Printf("dispatch: deliver prompt to %s: %v", event.ContainerID, err)
		s.ReportStatus(event.JobID, event.ContainerID, protocol.JobClaimed, protocol.JobFailed, "prompt delivery failed: "+err.Error())
		return fmt.Errorf("dispatch: deliver prompt: %w", err)
	}

	// Transition to running.
	err = s.store.SetJobState(redis.DispatchJobState{
		JobID:       event.JobID,
		ContainerID: event.ContainerID,
		State:       protocol.JobRunning,
		Repository:  event.Repository,
		IssueNumber: event.IssueNumber,
	})
	if err != nil {
		s.logger.Printf("dispatch: update job state to running: %v", err)
	}

	s.ReportStatus(event.JobID, event.ContainerID, protocol.JobClaimed, protocol.JobRunning, "")

	// Track active job.
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.activeJobs[event.JobID] = cancel
	s.mu.Unlock()

	// The cancel function is used by TerminateJob to signal abort.
	_ = ctx

	return nil
}

// TerminateJob signals the container to abort a specific background task.
func (s *Service) TerminateJob(jobID uuid.UUID) error {
	s.mu.Lock()
	cancel, exists := s.activeJobs[jobID]
	if exists {
		cancel()
		delete(s.activeJobs, jobID)
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("dispatch: job %s not active on this host", jobID)
	}

	// Look up the job to find its container.
	state, err := s.store.GetJobState(jobID)
	if err != nil {
		return fmt.Errorf("dispatch: get job state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("dispatch: job %s not found in dispatch store", jobID)
	}

	// Signal the wrapper.
	err = s.containerMgr.TerminateTask(state.ContainerID, jobID)
	if err != nil {
		s.logger.Printf("dispatch: terminate task %s in %s: %v", jobID, state.ContainerID, err)
	}

	// Update state.
	_ = s.store.SetJobState(redis.DispatchJobState{
		JobID:       jobID,
		ContainerID: state.ContainerID,
		State:       protocol.JobTerminated,
		Repository:  state.Repository,
		IssueNumber: state.IssueNumber,
	})
	s.ReportStatus(jobID, state.ContainerID, state.State, protocol.JobTerminated, "terminated by operator")

	return nil
}

// CompleteJob marks a job as complete and cleans up.
func (s *Service) CompleteJob(jobID uuid.UUID, containerID, prURL string) {
	s.mu.Lock()
	cancel, exists := s.activeJobs[jobID]
	if exists {
		cancel()
		delete(s.activeJobs, jobID)
	}
	s.mu.Unlock()

	state, err := s.store.GetJobState(jobID)
	if err != nil || state == nil {
		s.logger.Printf("dispatch: complete job %s: state not found", jobID)
		return
	}

	_ = s.store.SetJobState(redis.DispatchJobState{
		JobID:       jobID,
		ContainerID: containerID,
		State:       protocol.JobComplete,
		Repository:  state.Repository,
		IssueNumber: state.IssueNumber,
	})
	s.ReportStatus(jobID, containerID, protocol.JobRunning, protocol.JobComplete, prURL)
}

// FailJob marks a job as failed.
func (s *Service) FailJob(jobID uuid.UUID, containerID, reason string) {
	s.mu.Lock()
	cancel, exists := s.activeJobs[jobID]
	if exists {
		cancel()
		delete(s.activeJobs, jobID)
	}
	s.mu.Unlock()

	state, err := s.store.GetJobState(jobID)
	if err != nil || state == nil {
		s.logger.Printf("dispatch: fail job %s: state not found", jobID)
		return
	}

	_ = s.store.SetJobState(redis.DispatchJobState{
		JobID:       jobID,
		ContainerID: containerID,
		State:       protocol.JobFailed,
		Repository:  state.Repository,
		IssueNumber: state.IssueNumber,
	})
	s.ReportStatus(jobID, containerID, state.State, protocol.JobFailed, reason)
}

// ActiveJobCount returns the number of active jobs on this host.
func (s *Service) ActiveJobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeJobs)
}

// ReportStatus sends a status transition to the Router API.
func (s *Service) ReportStatus(jobID uuid.UUID, containerID string, from, to protocol.JobState, reason string) {
	req := protocol.StatusTransitionRequest{
		HostID:      s.hostID,
		ContainerID: containerID,
		JobID:       jobID,
		FromState:   from,
		ToState:     to,
		Timestamp:   time.Now(),
		Reason:      reason,
	}
	if to == protocol.JobComplete && reason != "" {
		req.PullURL = reason
		req.Reason = ""
	}

	data := mustMarshalJSON(req)

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.controlURL+"/v1/status", bytes.NewReader(data))
	if err != nil {
		s.logger.Printf("dispatch: create status request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.logger.Printf("dispatch: report status: %v", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
}

// SendHeartbeat sends a heartbeat to the Router API.
func (s *Service) SendHeartbeat() {
	req := protocol.HeartbeatRequest{
		HostID:         s.hostID,
		ContainerCount: s.containerMgr.ContainerCount(),
		CPUPercent:     s.containerMgr.CPUPercent(),
		MemoryPercent:  s.containerMgr.MemoryPercent(),
		Timestamp:      time.Now(),
	}

	data := mustMarshalJSON(req)

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.controlURL+"/v1/heartbeat", bytes.NewReader(data))
	if err != nil {
		s.logger.Printf("dispatch: create heartbeat request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.logger.Printf("dispatch: send heartbeat: %v", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
}

// StartHeartbeatLoop starts sending heartbeats every 15 seconds.
func (s *Service) StartHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	s.SendHeartbeat() // Initial heartbeat on start.
	for {
		select {
		case <-ticker.C:
			s.SendHeartbeat()
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		}
	}
}

// Stop signals the service to shut down.
func (s *Service) Stop() {
	close(s.stopCh)
}

// buildPrompt constructs the prompt string from a dispatch event.
func buildPrompt(event *protocol.DispatchEvent) string {
	if event.AdHoc {
		return "Background task: " + event.Prompt
	}
	prompt := fmt.Sprintf("Background task: Implement the following GitHub issue for repository %s.\n\n", event.Repository)
	prompt += fmt.Sprintf("Issue #%d: %s\n\n", event.IssueNumber, event.IssueTitle)
	prompt += event.IssueBody
	if event.IssueAuthor != "" {
		prompt += fmt.Sprintf("\n\nOriginal author: @%s", event.IssueAuthor)
	}
	return prompt
}
