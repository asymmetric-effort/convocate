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
  nmgr: NodeManager, amgr: AgentManager, pb: ProjectBoard,
  ide: CodeIDE, ac: AccessControl, repo: RepoManager, sup: SupportTool,
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

  const AppletComponent = activeApplet ? APPLET_COMPONENTS[activeApplet] : null;

  return h(UnityDesktop, {
    apps,
    user: principal ? { name: principal.name } : undefined,
    onAppOpen: handleAppOpen,
    onLogout: handleLogout,
    theme: "dark" as const,
  },
    AppletComponent ? h(AppletComponent, null) : null
  );
}
