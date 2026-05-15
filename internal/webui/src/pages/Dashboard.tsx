import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
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

  return createElement("div", { className: "dashboard" },
    createElement("h1", null, "Dashboard"),

    error ? createElement("div", { className: "error" }, error) : null,

    createElement("section", null,
      createElement("h2", null, "Projects"),
      projects.length === 0
        ? createElement("p", null, "No projects configured.")
        : createElement("table", null,
            createElement("thead", null,
              createElement("tr", null,
                createElement("th", null, "Repository"),
                createElement("th", null, "Host"),
                createElement("th", null, "State"),
                createElement("th", null, "Active Jobs"),
                createElement("th", null, "Actions"),
              )
            ),
            createElement("tbody", null,
              ...projects.map((project) =>
                createElement("tr", { key: project.project_id },
                  createElement("td", null, project.repository),
                  createElement("td", null, project.host_id),
                  createElement("td", null, project.container_state),
                  createElement("td", null, String(project.active_jobs)),
                  createElement("td", null,
                    project.upgrade_ready
                      ? createElement("button", {
                          onClick: () => api.upgradeContainer(project.project_id),
                        }, "Upgrade")
                      : null
                  ),
                )
              )
            ),
          )
    ),

    createElement("section", null,
      createElement("h2", null, "Recent Jobs"),
      jobs.length === 0
        ? createElement("p", null, "No jobs.")
        : createElement("table", null,
            createElement("thead", null,
              createElement("tr", null,
                createElement("th", null, "Job ID"),
                createElement("th", null, "Repository"),
                createElement("th", null, "Issue"),
                createElement("th", null, "Status"),
                createElement("th", null, "PR"),
              )
            ),
            createElement("tbody", null,
              ...jobs.map((job) =>
                createElement("tr", { key: job.job_id },
                  createElement("td", null, job.job_id.substring(0, 8)),
                  createElement("td", null, job.repository),
                  createElement("td", null, job.ad_hoc ? "ad-hoc" : `#${job.issue_number}`),
                  createElement("td", null, job.status),
                  createElement("td", null,
                    job.pr_url
                      ? createElement("a", { href: job.pr_url, target: "_blank" }, "View PR")
                      : null
                  ),
                )
              )
            ),
          )
    ),

    createElement("section", null,
      createElement("h2", null, "Agent Fleet Health"),
      hosts.length === 0
        ? createElement("p", null, "No agent hosts registered.")
        : createElement("table", null,
            createElement("thead", null,
              createElement("tr", null,
                createElement("th", null, "Host"),
                createElement("th", null, "Containers"),
                createElement("th", null, "CPU %"),
                createElement("th", null, "Memory %"),
                createElement("th", null, "Status"),
              )
            ),
            createElement("tbody", null,
              ...hosts.map((host) =>
                createElement("tr", { key: host.host_id },
                  createElement("td", null, host.host_id),
                  createElement("td", null, String(host.container_count)),
                  createElement("td", null, host.cpu_percent.toFixed(1)),
                  createElement("td", null, host.memory_percent.toFixed(1)),
                  createElement("td", null, host.healthy ? "Healthy" : "Unhealthy"),
                )
              )
            ),
          )
    ),
  );
}
