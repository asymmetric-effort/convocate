const BASE_URL = "/ui/api";

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const options: RequestInit = {
    method,
    headers: { "Content-Type": "application/json" },
  };
  if (body !== undefined) {
    options.body = JSON.stringify(body);
  }
  const response = await fetch(`${BASE_URL}${path}`, options);
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json();
}

export interface ProjectInfo {
  project_id: string;
  repository: string;
  host_id: string;
  container_id: string;
  container_state: string;
  container_image: string;
  upgrade_ready: boolean;
  active_jobs: number;
  created_at: string;
}

export interface CreateProjectRequest {
  repository: string;
  ssh_private_key: string;
  github_pat: string;
  custom_secrets?: Record<string, string>;
}

export interface CreateProjectResponse {
  project_id: string;
  repository: string;
  api_token: string;
  host_id: string;
  container_id: string;
  container_state: string;
}

export interface DeleteProjectRequest {
  project_id: string;
  force_terminate: boolean;
}

export interface JobMetadata {
  job_id: string;
  repository: string;
  issue_number: number;
  issue_title: string;
  status: string;
  pr_url: string;
  ad_hoc: boolean;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}

export interface HostHealthInfo {
  host_id: string;
  container_count: number;
  cpu_percent: number;
  memory_percent: number;
  last_heartbeat_unix: number;
  healthy: boolean;
}

export interface SetClusterAuthRequest {
  mode: "anthropic_api_key" | "claude_session";
  api_key?: string;
  session_token?: string;
}

export interface AdHocSubmissionRequest {
  project_id: string;
  prompt: string;
}

export const api = {
  listProjects: () => request<ProjectInfo[]>("GET", "/projects"),
  createProject: (req: CreateProjectRequest) =>
    request<CreateProjectResponse>("POST", "/projects/create", req),
  deleteProject: (req: DeleteProjectRequest) =>
    request<{ deleted: boolean }>("POST", "/projects/delete", req),
  upgradeContainer: (projectId: string) =>
    request<unknown>("POST", "/projects/upgrade", { project_id: projectId }),
  upgradeAllIdle: () =>
    request<unknown>("POST", "/projects/upgrade-all-idle"),
  listJobs: () => request<JobMetadata[]>("GET", "/jobs"),
  listHosts: () => request<HostHealthInfo[]>("GET", "/hosts"),
  getClusterAuth: () =>
    request<{ mode: string; updated: boolean }>("GET", "/auth"),
  setClusterAuth: (req: SetClusterAuthRequest) =>
    request<{ mode: string; updated: boolean }>("POST", "/auth", req),
  submitAdHoc: (req: AdHocSubmissionRequest) =>
    request<{ job_id: string }>("POST", "/adhoc", req),
};
