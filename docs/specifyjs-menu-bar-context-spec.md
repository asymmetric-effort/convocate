# SpecifyJS Menu Bar Context Switching — Integration Spec

## Problem

Convocate applets need to register per-applet menus in the `UnityDesktop` system tray (top menu bar). When a user focuses an applet window, the menu bar should display that applet's menus — like macOS, where the menu bar reflects the active application.

SpecifyJS already has the infrastructure for this:

1. `WindowManagerProvider` maintains a `menuBars` map keyed by window ID.
2. `UnityApp` accepts a `menuBar` prop and calls `windowManager.setMenuBar(id, menuBar)`.
3. `SystemTray` renders the `activeMenuBar` (resolved from the focused window's ID).

**The issue**: Convocate cannot use `UnityApp` to render real applet content because `UnityDesktop` also creates internal mock windows for each dock click. Rendering both `UnityApp` children and dock-triggered mock windows results in duplicate windows. Convocate therefore uses `AppletPortal` (DOM injection) to replace mock window content — but this means `UnityApp` is not in the render tree, so `menuBar` props cannot be passed.

## Proposed Solution

### Option A: Export `useWindowManager` (preferred)

Export the `useWindowManager()` hook from `@asymmetric-effort/specifyjs/components` so that applet code rendered inside `UnityDesktop` children (via portals or otherwise) can call `windowManager.setMenuBar(appId, menuBar)` directly.

This is the minimal change — the hook and the context already exist internally. Only the barrel export is missing.

**Required export:**
```typescript
// from @asymmetric-effort/specifyjs/components
export { useWindowManager } from './layout/window-manager/src/WindowManagerProvider';
```

**Convocate usage:**
```typescript
import { useWindowManager } from '@asymmetric-effort/specifyjs/components';

function NodeManagerApplet() {
  const wm = useWindowManager();

  useEffect(() => {
    wm.setMenuBar('nmgr', {
      menus: [
        {
          label: 'Node',
          items: [
            { label: 'Provision Node', shortcut: 'Ctrl+N', onClick: () => setShowProvision(true) },
            { label: 'Refresh', shortcut: 'Ctrl+R', onClick: loadNodes },
            { label: '', divider: true },
            { label: 'Close', onClick: onClose },
          ],
        },
        {
          label: 'View',
          items: [
            { label: 'Show All Nodes', onClick: () => setOffset(0) },
          ],
        },
      ],
    });
    return () => wm.clearMenuBar('nmgr');
  }, []);

  // ... applet content
}
```

### Option B: Export `WindowManagerProvider` context

Export the `WindowManagerContext` so consumers can create their own `useContext(WindowManagerContext)` call. This is essentially the same as Option A but less ergonomic.

### Option C: Add a `contentRenderer` callback to the `apps` array

Add an optional `render` or `component` field to the objects in the `UnityDesktop` `apps` array. When present, `UnityDesktopInner` would call this function instead of `getMockContent()` to render the window body content. This would also receive the window ID so the rendered component can register menus.

```typescript
<UnityDesktop
  apps={[
    {
      id: 'nmgr',
      label: 'Node Manager',
      icon: '/img/icons/node-manager.png',
      render: (windowId) => <NodeManagerApplet windowId={windowId} />,
    },
  ]}
/>
```

This option eliminates the need for `AppletPortal` entirely and solves the dual-window problem. It would require changes to `UnityDesktopInner` to check for `app.render` before falling back to `getMockContent()`.

## Menu Bar Format

The `menuBar` object passed to `setMenuBar()` has this shape:

```typescript
interface AppMenuBar {
  menus: AppMenu[];
}

interface AppMenu {
  label: string;              // e.g. "File", "Edit", "Node"
  items: AppMenuItem[];
}

interface AppMenuItem {
  label: string;              // e.g. "Save", "Provision Node"
  shortcut?: string;          // e.g. "Ctrl+S", "Ctrl+N"
  onClick: () => void;
  divider?: boolean;          // renders a separator line instead of a clickable item
  disabled?: boolean;
  icon?: string;
}
```

## Per-Applet Menus (Convocate)

Each Convocate applet registers its own menus when focused:

### Node Manager
| Menu | Items |
|------|-------|
| Node | Provision Node (Ctrl+N), Refresh (Ctrl+R), —, Close |
| View | Show All Nodes, Filter by Status |

### Agent Manager
| Menu | Items |
|------|-------|
| Agent | Create Agent (Ctrl+N), Refresh (Ctrl+R), —, Close |
| View | Group by Node, Show All |

### Code Monkey IDE
| Menu | Items |
|------|-------|
| File | New File (Ctrl+N), Open, Save (Ctrl+S), Save As, —, Close |
| Edit | Undo (Ctrl+Z), Redo (Ctrl+Shift+Z), —, Cut, Copy, Paste |
| View | Explorer, Command Palette (Ctrl+Shift+P) |

### Project Board
| Menu | Items |
|------|-------|
| Graph | New Card, New Container, —, Save to Repository, Open from Repository, —, Rename Project, Close |
| View | Status View, Canvas View, —, Zoom In, Zoom Out, Fit All |
| Edit | Undo (Ctrl+Z), Redo (Ctrl+Shift+Z), —, Delete Selected |

### Access Control
| Menu | Items |
|------|-------|
| Admin | Add User, Add Group, —, Close |
| View | Users, Groups, Settings |

### Repo Manager
| Menu | Items |
|------|-------|
| Repository | New Repository (Ctrl+N), —, Close |
| View | Files, Pull Requests |

### Support Tool
| Menu | Items |
|------|-------|
| Support | New Ticket (Ctrl+N), —, Close |
| View | Tickets, Documentation |

## Recommendation

**Option C** is the cleanest long-term solution as it eliminates the `AppletPortal` hack entirely. It requires a small change to `UnityDesktopInner` (check for `app.render` before `getMockContent`) but produces a proper integration point for real applications.

**Option A** is the quickest fix if the `apps` array architecture shouldn't change. One line added to the barrel export.

Either way, the `menuBar` format and `setMenuBar`/`clearMenuBar` API are already correct and don't need changes.
