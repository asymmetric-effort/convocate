import type { LoginRequest, Session, Principal } from "../types/api";
import { apiPost, apiGet, setAccessToken, getAccessToken } from "./api";

let currentPrincipal: Principal | null = null;
let onAuthChange: ((p: Principal | null) => void) | null = null;

export function setAuthChangeListener(fn: (p: Principal | null) => void) { onAuthChange = fn; }
export function getPrincipal(): Principal | null { return currentPrincipal; }

export async function login(req: LoginRequest): Promise<Session> {
  const session = await apiPost<Session>("/auth/login", req);
  setAccessToken(session.accessToken);
  currentPrincipal = session.principal;
  onAuthChange?.(currentPrincipal);
  return session;
}

export async function logout(): Promise<void> {
  try { await apiPost("/auth/logout"); } catch {}
  setAccessToken(null);
  currentPrincipal = null;
  onAuthChange?.(null);
}

export async function fetchMe(): Promise<Principal | null> {
  if (!getAccessToken()) return null;
  try {
    currentPrincipal = await apiGet<Principal>("/auth/me");
    onAuthChange?.(currentPrincipal);
    return currentPrincipal;
  } catch {
    setAccessToken(null);
    currentPrincipal = null;
    onAuthChange?.(null);
    return null;
  }
}

export function hasRole(role: string): boolean {
  if (!currentPrincipal) return false;
  if (currentPrincipal.roles.includes("admin")) return true;
  return currentPrincipal.roles.includes(role);
}

export function hasApplet(applet: string): boolean {
  if (!currentPrincipal) return false;
  return currentPrincipal.authorizedApplets.includes(applet);
}
