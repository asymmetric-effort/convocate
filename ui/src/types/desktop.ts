// Desktop shell types for the Unity/GNOME-style UI

export interface DockItem {
  id: string;
  applet: string;
  label: string;
  icon: string;
  requiredRole: string;
}

export interface AppWindow {
  id: string;
  applet: string;
  title: string;
  x: number;
  y: number;
  width: number;
  height: number;
  minimized: boolean;
  maximized: boolean;
  focused: boolean;
  zIndex: number;
}

export interface MenuItem {
  label: string;
  action?: () => void;
  separator?: boolean;
  children?: MenuItem[];
  disabled?: boolean;
}

export interface MenuBarContext {
  applet: string;
  menus: MenuItem[];
}

export type DesktopState = "locked" | "unlocked";

export const DOCK_ITEMS: DockItem[] = [
  { id: "nmgr", applet: "nmgr", label: "Node Manager", icon: "img/icons/node-manager.png", requiredRole: "node-view" },
  { id: "amgr", applet: "amgr", label: "Agent Manager", icon: "img/icons/agent-manager.png", requiredRole: "agent-view" },
  { id: "pb", applet: "pb", label: "Project Board", icon: "img/icons/productboard.png", requiredRole: "pb-view" },
  { id: "ide", applet: "ide", label: "Code IDE", icon: "img/icons/ide-monkey.png", requiredRole: "ide-view" },
  { id: "ac", applet: "ac", label: "Access Control", icon: "img/icons/access-control.png", requiredRole: "access-view" },
  { id: "repo", applet: "repo", label: "Repo Manager", icon: "img/icons/repo-man.png", requiredRole: "repo-view" },
  { id: "sup", applet: "sup", label: "Support Tool", icon: "img/icons/support-tool.png", requiredRole: "support-view" },
];
