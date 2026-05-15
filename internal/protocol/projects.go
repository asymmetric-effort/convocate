package protocol

import (
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// CreateProjectRequest is the Web UI payload for creating a new project.
type CreateProjectRequest struct {
	CustomSecrets map[string]string `json:"custom_secrets,omitempty"`
	Repository    string            `json:"repository"`
	SSHPrivateKey string            `json:"ssh_private_key"`
	GitHubPAT     string            `json:"github_pat"`
}

// CreateProjectResponse is returned after a successful project creation.
type CreateProjectResponse struct {
	Repository     string         `json:"repository"`
	APIToken       string         `json:"api_token"`
	HostID         string         `json:"host_id"`
	ContainerID    string         `json:"container_id"`
	ContainerState ContainerState `json:"container_state"`
	ProjectID      uuid.UUID      `json:"project_id"`
}

// DeleteProjectRequest is the Web UI payload for deleting a project.
type DeleteProjectRequest struct {
	ProjectID      uuid.UUID `json:"project_id"`
	ForceTerminate bool      `json:"force_terminate"`
}

// DeleteProjectResponse is returned after a successful project deletion.
type DeleteProjectResponse struct {
	Repository string    `json:"repository"`
	ProjectID  uuid.UUID `json:"project_id"`
	Deleted    bool      `json:"deleted"`
}

// ProjectInfo represents a project in list/detail views.
type ProjectInfo struct {
	CreatedAt      time.Time      `json:"created_at"`
	Repository     string         `json:"repository"`
	HostID         string         `json:"host_id"`
	ContainerID    string         `json:"container_id"`
	ContainerState ContainerState `json:"container_state"`
	ContainerImage string         `json:"container_image"`
	ActiveJobs     int            `json:"active_jobs"`
	ProjectID      uuid.UUID      `json:"project_id"`
	UpgradeReady   bool           `json:"upgrade_ready"`
}

// UpgradeContainerRequest is the Web UI payload for upgrading a project's
// container to a new image.
type UpgradeContainerRequest struct {
	ProjectID uuid.UUID `json:"project_id"`
}

// UpgradeContainerResponse is returned after a container upgrade.
type UpgradeContainerResponse struct {
	ContainerID    string         `json:"container_id"`
	ContainerState ContainerState `json:"container_state"`
	ProjectID      uuid.UUID      `json:"project_id"`
}

// UpgradeAllIdleResponse is returned after upgrading all idle containers.
type UpgradeAllIdleResponse struct {
	Upgraded []uuid.UUID `json:"upgraded"`
	Skipped  []uuid.UUID `json:"skipped"`
}

// ContainerMapEntry represents a single entry in the Router API's container map.
type ContainerMapEntry struct {
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	ContainerID string         `json:"container_id"`
	HostID      string         `json:"host_id"`
	State       ContainerState `json:"state"`
	Image       string         `json:"image"`
	ProjectID   uuid.UUID      `json:"project_id"`
}

// ProjectRouteEntry represents a binding in the project routing table.
type ProjectRouteEntry struct {
	Repository  string    `json:"repository"`
	HostID      string    `json:"host_id"`
	ContainerID string    `json:"container_id"`
	ProjectID   uuid.UUID `json:"project_id"`
}
