import { createElement, useState, useEffect, useCallback } from "@asymmetric-effort/specifyjs";
import {
  UnityDesktop,
  UnityApp,
  useMessageBus,
} from "@asymmetric-effort/specifyjs/components";
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
  const [openApps, setOpenApps] = useState<string[]>([]);

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
    setOpenApps([]);
  }

  const handleAppOpen = useCallback((appId: string) => {
    setOpenApps((prev: string[]) => {
      if (prev.includes(appId)) return prev;
      return [...prev, appId];
    });
  }, []);

  function handleAppClose(appId: string) {
    setOpenApps((prev: string[]) => prev.filter((id: string) => id !== appId));
  }

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

  return h(UnityDesktop, {
    apps,
    user: principal ? { name: principal.name } : undefined,
    onAppOpen: handleAppOpen,
    onLogout: handleLogout,
    theme: "dark" as const,
  },
    // Render UnityApp windows for each opened applet.
    // UnityApp registers with the WindowManagerProvider and renders as
    // a draggable/resizable window within the desktop workspace.
    openApps.map((appId: string) => {
      const Component = APPLET_COMPONENTS[appId];
      if (!Component) return null;
      const item = DOCK_ITEMS.find((d) => d.applet === appId);
      return h(UnityApp, {
        key: appId,
        id: appId,
        title: APPLET_TITLES[appId] || appId,
        icon: item ? `/${item.icon}` : "",
        defaultSize: { width: 900, height: 600 },
        onClose: () => handleAppClose(appId),
        resizable: true,
      },
        h(Component, null)
      );
    })
  );
}
