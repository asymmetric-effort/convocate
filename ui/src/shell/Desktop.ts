import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import type { AppWindow } from "../types/desktop";
import type { Principal } from "../types/api";
import { fetchMe, hasRole } from "../lib/auth";
import { getAccessToken } from "../lib/api";
import { Login } from "./Login";
import { Dock } from "./Dock";
import { MenuBar } from "./MenuBar";
import { WindowManager } from "./WindowManager";
import { NodeManager } from "../applets/nmgr/NodeManager";
import { AgentManager } from "../applets/amgr/AgentManager";
import { ProjectBoard } from "../applets/pb/ProjectBoard";
import { CodeIDE } from "../applets/ide/CodeIDE";
import { AccessControl } from "../applets/ac/AccessControl";
import { RepoManager } from "../applets/repo/RepoManager";
import { SupportTool } from "../applets/sup/SupportTool";

const h = createElement;

let nextZIndex = 1;
let windowId = 0;

export function Desktop() {
  const [state, setState] = useState<"locked" | "unlocked">(
    getAccessToken() ? "unlocked" : "locked"
  );
  const [windows, setWindows] = useState<AppWindow[]>([]);
  const [principal, setPrincipal] = useState<Principal | null>(null);

  useEffect(() => {
    if (getAccessToken()) {
      fetchMe().then((p) => {
        if (p) {
          setPrincipal(p);
          setState("unlocked");
        } else {
          setState("locked");
        }
      });
    }
  }, []);

  function handleLoginSuccess() {
    fetchMe().then((p) => {
      setPrincipal(p);
      setState("unlocked");
    });
  }

  function handleLock() {
    setState("locked");
    setWindows([]);
  }

  function openApplet(applet: string) {
    const existing = windows.find((w) => w.applet === applet);
    if (existing) {
      focusWindow(existing.id);
      return;
    }
    const id = `win-${++windowId}`;
    const win: AppWindow = {
      id,
      applet,
      title: APPLET_TITLES[applet] || applet,
      x: 100 + (windowId % 5) * 30,
      y: 60 + (windowId % 5) * 30,
      width: 900,
      height: 600,
      minimized: false,
      maximized: false,
      focused: true,
      zIndex: ++nextZIndex,
    };
    setWindows((prev: AppWindow[]) => [
      ...prev.map((w: AppWindow) => ({ ...w, focused: false })),
      win,
    ]);
  }

  function focusWindow(id: string) {
    setWindows((prev: AppWindow[]) =>
      prev.map((w: AppWindow) => ({
        ...w,
        focused: w.id === id,
        minimized: w.id === id ? false : w.minimized,
        zIndex: w.id === id ? ++nextZIndex : w.zIndex,
      }))
    );
  }

  function closeWindow(id: string) {
    setWindows((prev: AppWindow[]) => prev.filter((w: AppWindow) => w.id !== id));
  }

  function minimizeWindow(id: string) {
    setWindows((prev: AppWindow[]) =>
      prev.map((w: AppWindow) => w.id === id ? { ...w, minimized: true, focused: false } : w)
    );
  }

  function maximizeWindow(id: string) {
    setWindows((prev: AppWindow[]) =>
      prev.map((w: AppWindow) => w.id === id ? { ...w, maximized: !w.maximized } : w)
    );
  }

  function renderApplet(applet: string) {
    switch (applet) {
      case "nmgr":
        return h(NodeManager, null);
      case "amgr":
        return h(AgentManager, null);
      case "pb":
        return h(ProjectBoard, null);
      case "ide":
        return h(CodeIDE, null);
      case "ac":
        return h(AccessControl, null);
      case "repo":
        return h(RepoManager, null);
      case "sup":
        return h(SupportTool, null);
      default:
        return h("div", null, "Unknown applet");
    }
  }

  if (state === "locked") {
    return h(Login, { onSuccess: handleLoginSuccess });
  }

  const activeApplet = windows.find((w: AppWindow) => w.focused)?.applet || null;

  return h("div", { className: "desktop" },
    h(MenuBar, { activeApplet, onLock: handleLock }),
    h(Dock, { onAppletClick: openApplet, activeApplet }),
    h(WindowManager, {
      windows,
      onClose: closeWindow,
      onFocus: focusWindow,
      onMinimize: minimizeWindow,
      onMaximize: maximizeWindow,
      renderApplet,
    })
  );
}

const APPLET_TITLES: Record<string, string> = {
  nmgr: "Node Manager",
  amgr: "Agent Manager",
  pb: "Project Board",
  ide: "Code IDE",
  ac: "Access Control",
  repo: "Repo Manager",
  sup: "Support Tool",
};
