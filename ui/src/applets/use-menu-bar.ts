/**
 * useMenuBar — registers per-applet menus in the UnityDesktop system tray.
 *
 * Uses the SpecifyJS useWindowManager hook to call setMenuBar/clearMenuBar.
 * The menu bar updates when the applet's window is focused (macOS-style).
 *
 * Usage:
 *   useMenuBar("nmgr", [
 *     { label: "Node", items: [
 *       { label: "Provision Node", shortcut: "Ctrl+N", onClick: handleProvision },
 *       { label: "Refresh", shortcut: "Ctrl+R", onClick: handleRefresh },
 *     ]},
 *   ]);
 */

import { useEffect } from "@asymmetric-effort/specifyjs";
import { useWindowManager } from "@asymmetric-effort/specifyjs/components";

export interface MenuItem {
  label: string;
  shortcut?: string;
  onClick?: () => void;
  divider?: boolean;
  disabled?: boolean;
}

export interface Menu {
  label: string;
  items: MenuItem[];
}

/**
 * Register applet-specific menus in the UnityDesktop system tray.
 * Menus are shown when the applet's window is focused.
 *
 * @param appId — the applet's ID (e.g. "nmgr", "amgr")
 * @param menus — array of menu definitions
 */
export function useMenuBar(appId: string, menus: Menu[]) {
  const wm = useWindowManager();

  useEffect(() => {
    if (!wm || !wm.setMenuBar) return;

    // Find the window ID that matches this applet
    // The internal window system uses IDs like "nmgr-1", so we search
    // for a window whose ID starts with the appId
    const windowId = wm.windows?.find(
      (w: any) => w.id?.startsWith(appId) || w.appId === appId
    )?.id;

    if (windowId) {
      wm.setMenuBar(windowId, { menus });
      return () => {
        wm.clearMenuBar(windowId);
      };
    }

    // Fallback: use the appId directly
    wm.setMenuBar(appId, { menus });
    return () => {
      wm.clearMenuBar(appId);
    };
  }, [appId, wm]);
}
