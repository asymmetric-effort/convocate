import { createElement, useState } from "@asymmetric-effort/specifyjs";
import type { AppWindow } from "../types/desktop";

const h = createElement;

interface WindowManagerProps {
  windows: AppWindow[];
  onClose: (id: string) => void;
  onFocus: (id: string) => void;
  onMinimize: (id: string) => void;
  onMaximize: (id: string) => void;
  renderApplet: (applet: string) => any;
}

export function WindowManager({
  windows,
  onClose,
  onFocus,
  onMinimize,
  onMaximize,
  renderApplet,
}: WindowManagerProps) {
  return h("div", { className: "window-manager" },
    windows
      .filter((w) => !w.minimized)
      .sort((a, b) => a.zIndex - b.zIndex)
      .map((win) =>
        h("div", {
          key: win.id,
          className: `app-window ${win.focused ? "focused" : ""} ${win.maximized ? "maximized" : ""}`,
          style: win.maximized
            ? {}
            : {
                left: `${win.x}px`,
                top: `${win.y}px`,
                width: `${win.width}px`,
                height: `${win.height}px`,
                zIndex: win.zIndex,
              },
          onMouseDown: () => onFocus(win.id),
        },
          h("div", { className: "window-titlebar" },
            h("span", { className: "window-title" }, win.title),
            h("div", { className: "window-controls" },
              h("button", {
                className: "window-btn minimize",
                onClick: (e: Event) => { e.stopPropagation(); onMinimize(win.id); },
              }, "\u2013"),
              h("button", {
                className: "window-btn maximize",
                onClick: (e: Event) => { e.stopPropagation(); onMaximize(win.id); },
              }, "\u25A1"),
              h("button", {
                className: "window-btn close",
                onClick: (e: Event) => { e.stopPropagation(); onClose(win.id); },
              }, "\u00D7")
            )
          ),
          h("div", { className: "window-content" },
            renderApplet(win.applet)
          )
        )
      )
  );
}
