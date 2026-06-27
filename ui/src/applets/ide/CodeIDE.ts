import { createElement } from "@asymmetric-effort/specifyjs";
import { IDE } from "@asymmetric-effort/specifyjs/components";

const h = createElement;

export function CodeIDE() {
  return h(IDE, { className: "convocate-ide" });
}
