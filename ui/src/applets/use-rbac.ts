/**
 * useRBAC — helper for checking user roles in applet components.
 *
 * Extracts roles from the principal object and provides role-check functions.
 * The admin role implies all permissions.
 */

/** Check if the principal has a specific role */
export function hasRole(principal: any, role: string): boolean {
  if (!principal || !principal.roles) return false;
  return principal.roles.includes(role) || principal.roles.includes("admin") || principal.roles.includes("admin-policy");
}

/** Check if the principal has any of the given roles */
export function hasAnyRole(principal: any, ...roles: string[]): boolean {
  if (!principal || !principal.roles) return false;
  if (principal.roles.includes("admin") || principal.roles.includes("admin-policy")) return true;
  return roles.some((r) => principal.roles.includes(r));
}

/** Role definitions per applet (view role is minimum to see the applet) */
export const APPLET_ROLES = {
  nmgr: { view: "node-view", create: "node-create", update: "node-update", delete: "node-delete" },
  amgr: { view: "agent-view", update: "agent-update" },
  pb: { view: "pb-view", update: "pb-update", execute: "pb-execute" },
  ide: { view: "ide-view", update: "ide-update" },
  ac: { view: "access-view", update: "access-update" },
  repo: { view: "repo-view", update: "repo-update", merge: "repo-merge" },
  sup: { view: "support-view" },
} as const;
