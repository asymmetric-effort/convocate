import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { getPrincipal, logout } from "../lib/auth";

const h = createElement;

interface MenuBarProps {
  activeApplet: string | null;
  onLock: () => void;
}

export function MenuBar({ activeApplet, onLock }: MenuBarProps) {
  const [time, setTime] = useState(formatTime());
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const principal = getPrincipal();

  useEffect(() => {
    const interval = setInterval(() => setTime(formatTime()), 1000);
    return () => clearInterval(interval);
  }, []);

  function handleLogout() {
    setUserMenuOpen(false);
    logout();
    onLock();
  }

  function handleLock() {
    setUserMenuOpen(false);
    onLock();
  }

  const appletLabel = activeApplet
    ? APPLET_LABELS[activeApplet] || activeApplet
    : "Convocate";

  return h("div", { className: "menu-bar" },
    h("div", { className: "menu-bar-left" },
      h("span", { className: "menu-bar-applet" }, appletLabel)
    ),
    h("div", { className: "menu-bar-center" },
      h("span", { className: "menu-bar-time" }, time)
    ),
    h("div", { className: "menu-bar-right" },
      h("div", { className: "user-menu-container" },
        h("span", {
          className: "user-menu-trigger",
          onClick: () => setUserMenuOpen(!userMenuOpen),
        }, principal?.name || "User"),
        userMenuOpen ? h("div", { className: "user-menu-dropdown" },
          h("div", { className: "user-menu-item", onClick: handleLock }, "Lock Screen"),
          h("div", { className: "user-menu-item", onClick: handleLogout }, "Log Out")
        ) : null
      )
    )
  );
}

function formatTime(): string {
  return new Date().toLocaleString("en-US", {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });
}

const APPLET_LABELS: Record<string, string> = {
  nmgr: "Node Manager",
  amgr: "Agent Manager",
  pb: "Project Board",
  ide: "Code IDE",
  ac: "Access Control",
  repo: "Repo Manager",
  sup: "Support Tool",
};
