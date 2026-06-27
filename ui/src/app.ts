import { createElement } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { Desktop } from "./shell/Desktop";

const container = document.getElementById("app");
if (container) {
  const root = createRoot(container);
  root.render(createElement(Desktop, null));
}
