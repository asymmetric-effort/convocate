import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { UnityDesktop, UnityApp } from "@asymmetric-effort/specifyjs/components";
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

  // Filter dock items by user permissions
  const authorizedApplets = DOCK_ITEMS.filter((item) => hasApplet(item.applet));

  const apps: UnityDesktopApp[] = authorizedApplets.map((item) => ({
    id: item.applet,
    icon: `/${item.icon}`,
    label: item.label,
  }));

  return h(UnityDesktop, {
    apps,
    user: principal ? { name: principal.name } : undefined,
    onLogout: handleLogout,
    theme: "dark" as const,
  },
    // Always render all authorized UnityApp children — the window manager
    // handles open/close/focus visibility internally via dock clicks
    authorizedApplets.map((item) => {
      const Component = APPLET_COMPONENTS[item.applet];
      if (!Component) return null;
      return h(UnityApp, {
        key: item.applet,
        id: item.applet,
        title: APPLET_TITLES[item.applet] || item.label,
        icon: `/${item.icon}`,
        defaultSize: { width: 900, height: 600 },
      },
        h(Component, null)
      );
    })
  );
}
