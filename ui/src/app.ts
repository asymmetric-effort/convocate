import { createElement } from "@asymmetric-effort/specifyjs";
import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { Desktop } from "./shell/Desktop";

const root = createRoot(document.getElementById("app")!);
root.render(createElement(Desktop, null));
