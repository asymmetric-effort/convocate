import { createElement } from "@asymmetric-effort/specifyjs";
import { hasApplet } from "../lib/auth";
import { DOCK_ITEMS } from "../types/desktop";
import type { DockItem } from "../types/desktop";

const h = createElement;

interface DockProps {
  onAppletClick: (applet: string) => void;
  activeApplet: string | null;
}

export function Dock({ onAppletClick, activeApplet }: DockProps) {
  const visibleItems = DOCK_ITEMS.filter((item) => hasApplet(item.applet));

  return h("div", { className: "dock" },
    visibleItems.map((item: DockItem) =>
      h("div", {
        key: item.id,
        className: `dock-item ${activeApplet === item.applet ? "active" : ""}`,
        onClick: () => onAppletClick(item.applet),
        title: item.label,
      },
        h("img", { src: `/${item.icon}`, alt: item.label, className: "dock-icon" })
      )
    )
  );
}
