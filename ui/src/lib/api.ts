import type { ApiError } from "../types/api";

let accessToken: string | null = localStorage.getItem("accessToken");

export function setAccessToken(token: string | null): void {
  accessToken = token;
  if (token) localStorage.setItem("accessToken", token);
  else localStorage.removeItem("accessToken");
}

export function getAccessToken(): string | null {
  return accessToken;
}

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;

  const res = await fetch(`/api/v1${path}`, { ...options, headers });
  if (!res.ok) {
    let error: ApiError;
    try { error = await res.json(); } catch { error = { code: "unknown", message: res.statusText }; }
    throw { status: res.status, ...error };
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const apiGet = <T>(path: string) => apiFetch<T>(path);
export const apiPost = <T>(path: string, body?: unknown) => apiFetch<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined });
export const apiPut = <T>(path: string, body: unknown) => apiFetch<T>(path, { method: "PUT", body: JSON.stringify(body) });
export const apiPatch = <T>(path: string, body: unknown) => apiFetch<T>(path, { method: "PATCH", body: JSON.stringify(body) });
export const apiDelete = (path: string) => apiFetch<void>(path, { method: "DELETE" });
