import { useState, useEffect } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";
import type { ProjectInfo, JobMetadata, HostHealthInfo } from "../api/client";

export function Dashboard() {
  const [projects, setProjects] = useState<ProjectInfo[]>([]);
  const [jobs, setJobs] = useState<JobMetadata[]>([]);
  const [hosts, setHosts] = useState<HostHealthInfo[]>([]);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    api.listProjects().then(setProjects).catch((err: Error) => setError(err.message));
    api.listJobs().then(setJobs).catch((err: Error) => setError(err.message));
    api.listHosts().then(setHosts).catch((err: Error) => setError(err.message));
  }, []);

  return (
    <div className="dashboard">
      <h1>Dashboard</h1>

      {error ? <div className="error">{error}</div> : null}

      <section>
        <h2>Projects</h2>
        {projects.length === 0 ? (
          <p>No projects configured.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Repository</th>
                <th>Host</th>
                <th>State</th>
                <th>Active Jobs</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {projects.map((project) => (
                <tr key={project.project_id}>
                  <td>{project.repository}</td>
                  <td>{project.host_id}</td>
                  <td>{project.container_state}</td>
                  <td>{String(project.active_jobs)}</td>
                  <td>
                    {project.upgrade_ready ? (
                      <button onClick={() => api.upgradeContainer(project.project_id)}>
                        Upgrade
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h2>Recent Jobs</h2>
        {jobs.length === 0 ? (
          <p>No jobs.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Job ID</th>
                <th>Repository</th>
                <th>Issue</th>
                <th>Status</th>
                <th>PR</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job) => (
                <tr key={job.job_id}>
                  <td>{job.job_id.substring(0, 8)}</td>
                  <td>{job.repository}</td>
                  <td>{job.ad_hoc ? "ad-hoc" : `#${job.issue_number}`}</td>
                  <td>{job.status}</td>
                  <td>
                    {job.pr_url ? (
                      <a href={job.pr_url} target="_blank">View PR</a>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h2>Agent Fleet Health</h2>
        {hosts.length === 0 ? (
          <p>No agent hosts registered.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Host</th>
                <th>Containers</th>
                <th>CPU %</th>
                <th>Memory %</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {hosts.map((host) => (
                <tr key={host.host_id}>
                  <td>{host.host_id}</td>
                  <td>{String(host.container_count)}</td>
                  <td>{host.cpu_percent.toFixed(1)}</td>
                  <td>{host.memory_percent.toFixed(1)}</td>
                  <td>{host.healthy ? "Healthy" : "Unhealthy"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
