import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { UnityDesktop } from "@asymmetric-effort/specifyjs/components";
import type { UnityDesktopApp } from "@asymmetric-effort/specifyjs/components";
import type { Principal } from "../types/api";
import { fetchMe, hasApplet, logout as doLogout } from "../lib/auth";
import { getAccessToken } from "../lib/api";
import { Login } from "./Login";
import { DOCK_ITEMS } from "../types/desktop";

const h = createElement;

export function Desktop() {
  const [state, setState] = useState<"locked" | "unlocked">(
    getAccessToken() ? "unlocked" : "locked"
  );
  const [principal, setPrincipal] = useState<Principal | null>(null);

  useEffect(() => {
    if (getAccessToken()) {
      fetchMe().then((p) => {
        if (p) { setPrincipal(p); setState("unlocked"); }
        else { setState("locked"); }
      });
    }
  }, []);

  function handleLoginSuccess() {
    fetchMe().then((p) => {
      setPrincipal(p);
      setState("unlocked");
    });
  }

  function handleLogout() {
    doLogout();
    setState("locked");
    setPrincipal(null);
  }

  if (state === "locked") {
    return h(Login, { onSuccess: handleLoginSuccess });
  }

  // Build dock apps from authorized applets — following the demo pattern:
  // UnityDesktop with apps array, no children, no onAppOpen.
  // The framework handles dock clicks, window creation, and content
  // rendering via its built-in getMockContent based on app labels.
  const apps: UnityDesktopApp[] = DOCK_ITEMS
    .filter((item) => hasApplet(item.applet))
    .map((item) => ({
      id: item.applet,
      icon: `/${item.icon}`,
      label: item.label,
    }));

  return h(UnityDesktop, {
    apps,
    user: principal ? { name: principal.name } : undefined,
    onLogout: handleLogout,
    theme: "dark" as const,
  });
}
