import { test, expect, describe, beforeEach, mock } from "bun:test";
import {
  validateProjectName,
  PROJECT_NAME_PATTERN,
  fetchProjects,
  createProject,
  updateProject,
} from "./shared-projects";

// ---------------------------------------------------------------------
// Mock localStorage (not available in Bun test env)
// ---------------------------------------------------------------------
const storage: Record<string, string> = {};
(globalThis as any).localStorage = {
  getItem: (key: string) => storage[key] ?? null,
  setItem: (key: string, value: string) => { storage[key] = value; },
  removeItem: (key: string) => { delete storage[key]; },
};

// Helper to install a mock fetch for one call
function mockFetch(response: { ok: boolean; status?: number; body?: any }) {
  const fn = mock(() =>
    Promise.resolve({
      ok: response.ok,
      status: response.status ?? (response.ok ? 200 : 500),
      json: () => Promise.resolve(response.body ?? {}),
    } as any),
  );
  globalThis.fetch = fn as any;
  return fn;
}

// ---------------------------------------------------------------------
// validateProjectName
// ---------------------------------------------------------------------
describe("validateProjectName", () => {
  test("returns null for a valid name", () => {
    expect(validateProjectName("my-project")).toBeNull();
    expect(validateProjectName("Ab")).toBeNull();
    expect(validateProjectName("a1")).toBeNull();
    expect(validateProjectName("my_project_2")).toBeNull();
  });

  test("rejects empty string", () => {
    const err = validateProjectName("");
    expect(err).toBe("Project name is required.");
  });

  test("rejects whitespace-only string", () => {
    expect(validateProjectName("   ")).toBe("Project name is required.");
  });

  test("rejects name that is too short (1 char)", () => {
    const err = validateProjectName("a");
    expect(err).not.toBeNull();
  });

  test("rejects name that is too long (66+ chars)", () => {
    const long = "a" + "b".repeat(65); // 66 chars
    const err = validateProjectName(long);
    expect(err).not.toBeNull();
  });

  test("accepts name at max length (65 chars)", () => {
    const maxName = "a" + "b".repeat(64); // 65 chars total
    expect(validateProjectName(maxName)).toBeNull();
  });

  test("rejects name starting with a digit", () => {
    expect(validateProjectName("1project")).not.toBeNull();
  });

  test("rejects name starting with a hyphen", () => {
    expect(validateProjectName("-project")).not.toBeNull();
  });

  test("rejects name starting with an underscore", () => {
    expect(validateProjectName("_project")).not.toBeNull();
  });

  test("rejects name with special characters", () => {
    expect(validateProjectName("my project")).not.toBeNull();
    expect(validateProjectName("my@project")).not.toBeNull();
    expect(validateProjectName("my.project")).not.toBeNull();
    expect(validateProjectName("my/project")).not.toBeNull();
  });

  test("trims leading/trailing whitespace before validation", () => {
    expect(validateProjectName("  validName  ")).toBeNull();
  });
});

// ---------------------------------------------------------------------
// PROJECT_NAME_PATTERN
// ---------------------------------------------------------------------
describe("PROJECT_NAME_PATTERN", () => {
  test("matches valid project names", () => {
    expect(PROJECT_NAME_PATTERN.test("ab")).toBe(true);
    expect(PROJECT_NAME_PATTERN.test("my-project")).toBe(true);
    expect(PROJECT_NAME_PATTERN.test("A_thing_123")).toBe(true);
  });

  test("rejects names starting with non-letter", () => {
    expect(PROJECT_NAME_PATTERN.test("9abc")).toBe(false);
    expect(PROJECT_NAME_PATTERN.test("-abc")).toBe(false);
    expect(PROJECT_NAME_PATTERN.test("_abc")).toBe(false);
  });

  test("rejects single-character names", () => {
    expect(PROJECT_NAME_PATTERN.test("a")).toBe(false);
  });

  test("rejects names with spaces or dots", () => {
    expect(PROJECT_NAME_PATTERN.test("a b")).toBe(false);
    expect(PROJECT_NAME_PATTERN.test("a.b")).toBe(false);
  });
});

// ---------------------------------------------------------------------
// fetchProjects
// ---------------------------------------------------------------------
describe("fetchProjects", () => {
  beforeEach(() => {
    storage["accessToken"] = "test-token";
  });

  test("returns mapped project list on success", async () => {
    mockFetch({
      ok: true,
      body: {
        items: [
          { id: "p1", name: "alpha", repoId: "r1", boardId: "b1", agentId: "a1" },
          { id: "p2", name: "beta" },
        ],
      },
    });

    const projects = await fetchProjects();
    expect(projects).toHaveLength(2);
    expect(projects[0]).toEqual({ id: "p1", name: "alpha", repoId: "r1", boardId: "b1", agentId: "a1" });
    expect(projects[1]).toEqual({ id: "p2", name: "beta", repoId: "", boardId: "", agentId: "" });
  });

  test("returns empty array when items is missing", async () => {
    mockFetch({ ok: true, body: {} });
    const projects = await fetchProjects();
    expect(projects).toEqual([]);
  });

  test("throws on non-ok response", async () => {
    mockFetch({ ok: false, status: 403 });
    await expect(fetchProjects()).rejects.toThrow("Failed to fetch projects: 403");
  });

  test("includes Authorization header when token exists", async () => {
    const fn = mockFetch({ ok: true, body: { items: [] } });
    await fetchProjects();
    const call = fn.mock.calls[0];
    expect(call[1].headers.Authorization).toBe("Bearer test-token");
  });

  test("omits Authorization header when no token", async () => {
    delete storage["accessToken"];
    const fn = mockFetch({ ok: true, body: { items: [] } });
    await fetchProjects();
    const call = fn.mock.calls[0];
    expect(call[1].headers.Authorization).toBeUndefined();
  });
});

// ---------------------------------------------------------------------
// createProject
// ---------------------------------------------------------------------
describe("createProject", () => {
  beforeEach(() => {
    storage["accessToken"] = "test-token";
  });

  test("returns created project on success", async () => {
    mockFetch({
      ok: true,
      body: { id: "p1", name: "alpha", repoId: "r1", boardId: "b1", agentId: "a1" },
    });
    const project = await createProject("alpha");
    expect(project).toEqual({ id: "p1", name: "alpha", repoId: "r1", boardId: "b1", agentId: "a1" });
  });

  test("fills empty optional fields", async () => {
    mockFetch({ ok: true, body: { id: "p2", name: "beta" } });
    const project = await createProject("beta");
    expect(project.repoId).toBe("");
    expect(project.boardId).toBe("");
    expect(project.agentId).toBe("");
  });

  test("sends POST with name in body", async () => {
    const fn = mockFetch({ ok: true, body: { id: "p1", name: "test" } });
    await createProject("test");
    const call = fn.mock.calls[0];
    expect(call[0]).toBe("/api/v1/projects");
    expect(call[1].method).toBe("POST");
    expect(JSON.parse(call[1].body)).toEqual({ name: "test" });
  });

  test("throws with server error message on failure", async () => {
    mockFetch({ ok: false, status: 409, body: { message: "already exists" } });
    await expect(createProject("dup")).rejects.toThrow("already exists");
  });

  test("throws with status code when no error message in body", async () => {
    const fn = mock(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error("parse error")),
      } as any),
    );
    globalThis.fetch = fn as any;
    await expect(createProject("fail")).rejects.toThrow("Failed to create project: 500");
  });
});

// ---------------------------------------------------------------------
// updateProject
// ---------------------------------------------------------------------
describe("updateProject", () => {
  beforeEach(() => {
    storage["accessToken"] = "test-token";
  });

  test("returns updated project on success", async () => {
    mockFetch({
      ok: true,
      body: { id: "p1", name: "renamed", repoId: "r1", boardId: "b1", agentId: "a1" },
    });
    const project = await updateProject("p1", { name: "renamed" });
    expect(project.name).toBe("renamed");
  });

  test("sends PATCH to correct URL with updates", async () => {
    const fn = mockFetch({ ok: true, body: { id: "p1", name: "test" } });
    await updateProject("p1", { boardId: "b2" });
    const call = fn.mock.calls[0];
    expect(call[0]).toBe("/api/v1/projects/p1");
    expect(call[1].method).toBe("PATCH");
    expect(JSON.parse(call[1].body)).toEqual({ boardId: "b2" });
  });

  test("throws on non-ok response", async () => {
    mockFetch({ ok: false, status: 404 });
    await expect(updateProject("missing", {})).rejects.toThrow("Failed to update project: 404");
  });

  test("fills missing optional fields with empty strings", async () => {
    mockFetch({ ok: true, body: { id: "p1", name: "x" } });
    const project = await updateProject("p1", { name: "x" });
    expect(project.repoId).toBe("");
    expect(project.boardId).toBe("");
    expect(project.agentId).toBe("");
  });
});
