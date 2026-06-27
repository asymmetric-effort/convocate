import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import { UnityDesktop } from "@asymmetric-effort/specifyjs/components";
import type { UnityDesktopApp } from "@asymmetric-effort/specifyjs/components";
import type { Principal } from "../types/api";
import { fetchMe, hasApplet, logout as doLogout } from "../lib/auth";
import { getAccessToken } from "../lib/api";
import { Login } from "./Login";
import { DOCK_ITEMS } from "../types/desktop";
import { NodeManager } from "../applets/nmgr/NodeManager";
import { AgentManager } from "../applets/amgr/AgentManager";
import { ProjectBoard } from "../applets/pb/ProjectBoard";
import { CodeIDE } from "../applets/ide/CodeIDE";
import { AccessControl } from "../applets/ac/AccessControl";
import { RepoManager } from "../applets/repo/RepoManager";
import { SupportTool } from "../applets/sup/SupportTool";

const h = createElement;

const APPLET_COMPONENTS: Record<string, any> = {
  nmgr: NodeManager,
  amgr: AgentManager,
  pb: ProjectBoard,
  ide: CodeIDE,
  ac: AccessControl,
  repo: RepoManager,
  sup: SupportTool,
};

const APPLET_TITLES: Record<string, string> = {
  nmgr: "Node Manager",
  amgr: "Agent Manager",
  pb: "Project Board",
  ide: "Code IDE",
  ac: "Access Control",
  repo: "Repo Manager",
  sup: "Support Tool",
};

export function Desktop() {
  const [state, setState] = useState<"locked" | "unlocked">(
    getAccessToken() ? "unlocked" : "locked"
  );
  const [principal, setPrincipal] = useState<Principal | null>(null);
  const [activeApplet, setActiveApplet] = useState<string | null>(null);

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
    setActiveApplet(null);
  }

  const handleAppOpen = useCallback((appId: string) => {
    setActiveApplet(appId);
  }, []);

  if (state === "locked") {
    return h(Login, { onSuccess: handleLoginSuccess });
  }

  const apps: UnityDesktopApp[] = DOCK_ITEMS
    .filter((item) => hasApplet(item.applet))
    .map((item) => ({
      id: item.applet,
      icon: `/${item.icon}`,
      label: item.label,
    }));

  // Render applet content as a full-screen overlay on the desktop workspace
  // when UnityDesktop opens a window. The framework's built-in window
  // provides the chrome; we overlay our real applet content.
  const appletOverlay = activeApplet && APPLET_COMPONENTS[activeApplet]
    ? h("div", {
        style: {
          position: "absolute",
          top: 0, left: 0, right: 0, bottom: 0,
          zIndex: 50,
          background: "#1e1e1e",
          overflow: "auto",
          padding: "12px",
        },
      },
        h("div", {
          style: {
            display: "flex", justifyContent: "space-between", alignItems: "center",
            padding: "8px 12px", marginBottom: "12px",
            background: "#2c2c2c", borderRadius: "6px",
          },
        },
          h("span", { style: { fontWeight: 600, fontSize: "14px" } },
            APPLET_TITLES[activeApplet] || activeApplet),
          h("button", {
            onClick: () => setActiveApplet(null),
            style: {
              background: "none", border: "1px solid #555", borderRadius: "4px",
              color: "#fff", padding: "4px 12px", cursor: "pointer", fontSize: "12px",
            },
          }, "Close")
        ),
        h(APPLET_COMPONENTS[activeApplet], null)
      )
    : null;

  return h(UnityDesktop, {
    apps,
    user: principal ? { name: principal.name } : undefined,
    onAppOpen: handleAppOpen,
    onLogout: handleLogout,
    theme: "dark" as const,
  }, appletOverlay);
}
