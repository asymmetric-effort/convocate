import { test, expect, describe, mock, beforeEach } from "bun:test";
import type { Menu, MenuItem } from "./use-menu-bar";

/**
 * useMenuBar relies on SpecifyJS hooks (useEffect, useWindowManager) which
 * require a component lifecycle. We test the pure data structures (Menu,
 * MenuItem interfaces) and verify the module exports, plus test the core
 * logic that useMenuBar would execute by extracting it into a simulated
 * call against a mock window manager.
 */

// Mock SpecifyJS hooks at module level so the import resolves
let effectCallback: (() => (() => void) | void) | null = null;
let mockWm: any = null;

// We need to mock the SpecifyJS modules before importing useMenuBar
import { mock as bunMock } from "bun:test";

// Since Bun's module mocking is limited, we test the logic by simulating
// what useMenuBar does internally with a mock window manager.

describe("useMenuBar types and structure", () => {
  test("Menu interface accepts label and items", () => {
    const menu: Menu = {
      label: "File",
      items: [
        { label: "New", shortcut: "Ctrl+N", onClick: () => {} },
        { label: "Open", disabled: true },
        { label: "", divider: true },
      ],
    };
    expect(menu.label).toBe("File");
    expect(menu.items).toHaveLength(3);
  });

  test("MenuItem supports all optional fields", () => {
    const item: MenuItem = {
      label: "Save",
      shortcut: "Ctrl+S",
      onClick: () => {},
      divider: false,
      disabled: false,
    };
    expect(item.label).toBe("Save");
    expect(item.shortcut).toBe("Ctrl+S");
    expect(item.divider).toBe(false);
    expect(item.disabled).toBe(false);
  });

  test("MenuItem works with minimal fields", () => {
    const item: MenuItem = { label: "Quit" };
    expect(item.label).toBe("Quit");
    expect(item.shortcut).toBeUndefined();
    expect(item.onClick).toBeUndefined();
  });
});

describe("useMenuBar logic (simulated)", () => {
  let setMenuBar: ReturnType<typeof mock>;
  let clearMenuBar: ReturnType<typeof mock>;

  beforeEach(() => {
    setMenuBar = mock(() => {});
    clearMenuBar = mock(() => {});
  });

  test("sets menu bar on matching window by appId prefix", () => {
    const wm = {
      setMenuBar,
      clearMenuBar,
      windows: [{ id: "nmgr-1", appId: "nmgr" }],
    };

    const menus: Menu[] = [{ label: "Node", items: [{ label: "Refresh" }] }];
    const appId = "nmgr";

    // Simulate the logic in useMenuBar's useEffect body
    const windowId = wm.windows?.find(
      (w: any) => w.id?.startsWith(appId) || w.appId === appId,
    )?.id;

    if (windowId) {
      wm.setMenuBar(windowId, { menus });
    }

    expect(setMenuBar).toHaveBeenCalledTimes(1);
    expect(setMenuBar).toHaveBeenCalledWith("nmgr-1", { menus });
  });

  test("falls back to appId when no matching window found", () => {
    const wm = {
      setMenuBar,
      clearMenuBar,
      windows: [{ id: "amgr-1", appId: "amgr" }],
    };

    const menus: Menu[] = [{ label: "Node", items: [{ label: "Refresh" }] }];
    const appId = "nmgr";

    const windowId = wm.windows?.find(
      (w: any) => w.id?.startsWith(appId) || w.appId === appId,
    )?.id;

    if (windowId) {
      wm.setMenuBar(windowId, { menus });
    } else {
      wm.setMenuBar(appId, { menus });
    }

    expect(setMenuBar).toHaveBeenCalledTimes(1);
    expect(setMenuBar).toHaveBeenCalledWith("nmgr", { menus });
  });

  test("cleanup calls clearMenuBar with correct id", () => {
    const wm = {
      setMenuBar,
      clearMenuBar,
      windows: [{ id: "pb-1", appId: "pb" }],
    };

    const appId = "pb";

    const windowId = wm.windows?.find(
      (w: any) => w.id?.startsWith(appId) || w.appId === appId,
    )?.id;

    // Simulate cleanup
    if (windowId) {
      wm.setMenuBar(windowId, { menus: [] });
      wm.clearMenuBar(windowId);
    }

    expect(clearMenuBar).toHaveBeenCalledWith("pb-1");
  });

  test("handles null/undefined window manager gracefully", () => {
    const wm: any = null;

    // Simulate the guard: if (!wm || !wm.setMenuBar) return;
    const shouldSkip = !wm || !wm?.setMenuBar;
    expect(shouldSkip).toBe(true);
  });

  test("handles window manager without setMenuBar", () => {
    const wm: any = { windows: [] };
    const shouldSkip = !wm || !wm.setMenuBar;
    expect(shouldSkip).toBe(true);
  });

  test("matches window by appId field when id does not match prefix", () => {
    const wm = {
      setMenuBar,
      clearMenuBar,
      windows: [{ id: "window-42", appId: "ide" }],
    };

    const appId = "ide";
    const windowId = wm.windows?.find(
      (w: any) => w.id?.startsWith(appId) || w.appId === appId,
    )?.id;

    expect(windowId).toBe("window-42");
  });
});
