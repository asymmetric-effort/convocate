import { test, expect, describe } from "bun:test";
import { hasRole, hasAnyRole, APPLET_ROLES } from "./use-rbac";

// ---------------------------------------------------------------------
// hasRole
// ---------------------------------------------------------------------
describe("hasRole", () => {
  test("returns true when principal has the exact role", () => {
    const principal = { roles: ["node-view", "node-create"] };
    expect(hasRole(principal, "node-view")).toBe(true);
    expect(hasRole(principal, "node-create")).toBe(true);
  });

  test("returns false when principal lacks the role", () => {
    const principal = { roles: ["node-view"] };
    expect(hasRole(principal, "node-delete")).toBe(false);
  });

  test("admin role implies every role", () => {
    const principal = { roles: ["admin"] };
    expect(hasRole(principal, "node-view")).toBe(true);
    expect(hasRole(principal, "agent-update")).toBe(true);
    expect(hasRole(principal, "any-arbitrary-role")).toBe(true);
  });

  test("returns false for null principal", () => {
    expect(hasRole(null, "node-view")).toBe(false);
  });

  test("returns false for undefined principal", () => {
    expect(hasRole(undefined, "node-view")).toBe(false);
  });

  test("returns false when principal has no roles property", () => {
    expect(hasRole({}, "node-view")).toBe(false);
    expect(hasRole({ name: "alice" }, "node-view")).toBe(false);
  });

  test("returns false when roles array is empty", () => {
    expect(hasRole({ roles: [] }, "node-view")).toBe(false);
  });

  test("admin in roles array makes hasRole true even for unlisted roles", () => {
    const principal = { roles: ["admin", "node-view"] };
    expect(hasRole(principal, "something-else")).toBe(true);
  });
});

// ---------------------------------------------------------------------
// hasAnyRole
// ---------------------------------------------------------------------
describe("hasAnyRole", () => {
  test("returns true when principal has one of the listed roles", () => {
    const principal = { roles: ["node-view"] };
    expect(hasAnyRole(principal, "node-view", "node-delete")).toBe(true);
  });

  test("returns false when principal has none of the listed roles", () => {
    const principal = { roles: ["node-view"] };
    expect(hasAnyRole(principal, "agent-update", "pb-execute")).toBe(false);
  });

  test("admin role implies all roles", () => {
    const principal = { roles: ["admin"] };
    expect(hasAnyRole(principal, "node-view", "agent-update")).toBe(true);
  });

  test("returns false for null principal", () => {
    expect(hasAnyRole(null, "node-view")).toBe(false);
  });

  test("returns false for undefined principal", () => {
    expect(hasAnyRole(undefined, "node-view")).toBe(false);
  });

  test("returns false when principal has no roles property", () => {
    expect(hasAnyRole({}, "node-view")).toBe(false);
  });

  test("returns true when at least one role matches", () => {
    const principal = { roles: ["pb-execute", "ide-view"] };
    expect(hasAnyRole(principal, "node-view", "ide-view")).toBe(true);
  });

  test("returns false for empty roles list on principal", () => {
    expect(hasAnyRole({ roles: [] }, "node-view")).toBe(false);
  });
});

// ---------------------------------------------------------------------
// APPLET_ROLES
// ---------------------------------------------------------------------
describe("APPLET_ROLES", () => {
  test("has exactly 7 applets", () => {
    expect(Object.keys(APPLET_ROLES)).toHaveLength(7);
  });

  test("contains the expected applet keys", () => {
    const keys = Object.keys(APPLET_ROLES);
    expect(keys).toContain("nmgr");
    expect(keys).toContain("amgr");
    expect(keys).toContain("pb");
    expect(keys).toContain("ide");
    expect(keys).toContain("ac");
    expect(keys).toContain("repo");
    expect(keys).toContain("sup");
  });

  test("every applet has a view role", () => {
    for (const [, roles] of Object.entries(APPLET_ROLES)) {
      expect((roles as Record<string, string>).view).toBeDefined();
    }
  });

  test("nmgr has view, create, update, delete roles", () => {
    expect(APPLET_ROLES.nmgr).toEqual({
      view: "node-view",
      create: "node-create",
      update: "node-update",
      delete: "node-delete",
    });
  });

  test("amgr has view and update roles", () => {
    expect(APPLET_ROLES.amgr).toEqual({
      view: "agent-view",
      update: "agent-update",
    });
  });

  test("pb has view, update, execute roles", () => {
    expect(APPLET_ROLES.pb).toEqual({
      view: "pb-view",
      update: "pb-update",
      execute: "pb-execute",
    });
  });

  test("ide has view and update roles", () => {
    expect(APPLET_ROLES.ide).toEqual({
      view: "ide-view",
      update: "ide-update",
    });
  });

  test("ac has view and update roles", () => {
    expect(APPLET_ROLES.ac).toEqual({
      view: "access-view",
      update: "access-update",
    });
  });

  test("repo has view, update, merge roles", () => {
    expect(APPLET_ROLES.repo).toEqual({
      view: "repo-view",
      update: "repo-update",
      merge: "repo-merge",
    });
  });

  test("sup has only view role", () => {
    expect(APPLET_ROLES.sup).toEqual({
      view: "support-view",
    });
  });
});
