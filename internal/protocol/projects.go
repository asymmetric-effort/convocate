package protocol

import (
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// CreateProjectRequest is the Web UI payload for creating a new project.
type CreateProjectRequest struct {
	Repository    string            `json:"repository"`
	SSHPrivateKey string            `json:"ssh_private_key"`
	GitHubPAT     string            `json:"github_pat"`
	CustomSecrets map[string]string `json:"custom_secrets,omitempty"`
}

// CreateProjectResponse is returned after a successful project creation.
type CreateProjectResponse struct {
	ProjectID       uuid.UUID      `json:"project_id"`
	Repository      string         `json:"repository"`
	APIToken        string         `json:"api_token"`
	HostID          string         `json:"host_id"`
	ContainerID     string         `json:"container_id"`
	ContainerState  ContainerState `json:"container_state"`
}

// DeleteProjectRequest is the Web UI payload for deleting a project.
type DeleteProjectRequest struct {
	ProjectID      uuid.UUID `json:"project_id"`
	ForceTerminate bool      `json:"force_terminate"`
}

// DeleteProjectResponse is returned after a successful project deletion.
type DeleteProjectResponse struct {
	ProjectID  uuid.UUID `json:"project_id"`
	Repository string    `json:"repository"`
	Deleted    bool      `json:"deleted"`
}

// ProjectInfo represents a project in list/detail views.
type ProjectInfo struct {
	ProjectID      uuid.UUID      `json:"project_id"`
	Repository     string         `json:"repository"`
	HostID         string         `json:"host_id"`
	ContainerID    string         `json:"container_id"`
	ContainerState ContainerState `json:"container_state"`
	ContainerImage string         `json:"container_image"`
	UpgradeReady   bool           `json:"upgrade_ready"`
	ActiveJobs     int            `json:"active_jobs"`
	CreatedAt      time.Time      `json:"created_at"`
}

// UpgradeContainerRequest is the Web UI payload for upgrading a project's
// container to a new image.
type UpgradeContainerRequest struct {
	ProjectID uuid.UUID `json:"project_id"`
}

// UpgradeContainerResponse is returned after a container upgrade.
type UpgradeContainerResponse struct {
	ProjectID      uuid.UUID      `json:"project_id"`
	ContainerID    string         `json:"container_id"`
	ContainerState ContainerState `json:"container_state"`
}

// UpgradeAllIdleResponse is returned after upgrading all idle containers.
type UpgradeAllIdleResponse struct {
	Upgraded []uuid.UUID `json:"upgraded"`
	Skipped  []uuid.UUID `json:"skipped"`
}

// ContainerMapEntry represents a single entry in the Router API's container map.
type ContainerMapEntry struct {
	ContainerID string         `json:"container_id"`
	HostID      string         `json:"host_id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	State       ContainerState `json:"state"`
	Image       string         `json:"image"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ProjectRouteEntry represents a binding in the project routing table.
type ProjectRouteEntry struct {
	ProjectID   uuid.UUID `json:"project_id"`
	Repository  string    `json:"repository"`
	HostID      string    `json:"host_id"`
	ContainerID string    `json:"container_id"`
}
