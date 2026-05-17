import { createRoot } from "@asymmetric-effort/specifyjs/dom";
import { App, setAppRoot } from "./app";
import { api, UnauthorizedError } from "./api/client";
import type { MeResponse } from "./api/client";
import "./styles.css";

// Check auth BEFORE rendering — specifyjs setState doesn't trigger
// re-renders, so we determine state upfront and re-render the tree
// on navigation changes.
async function bootstrap() {
  const rootEl = document.getElementById("root");
  if (!rootEl) return;

  let authenticated = false;
  let user: MeResponse | null = null;
  let startupError = "";

  try {
    user = await api.getMe();
    authenticated = true;
  } catch (err: unknown) {
    if (err instanceof UnauthorizedError) {
      authenticated = false;
    } else {
      startupError = err instanceof Error ? err.message : "Unknown error";
    }
  }

  const root = createRoot(rootEl);
  setAppRoot(root);

  root.render(
    <App
      initialAuthenticated={authenticated}
      initialUser={user}
      initialError={startupError}
    />
  );
}

// Show loading state immediately via plain HTML, then bootstrap.
const rootEl = document.getElementById("root");
if (rootEl) {
  rootEl.innerHTML = `
    <div class="startup-screen">
      <div class="startup-spinner">
        <div class="nav-brand">convocate</div>
        <p>Connecting to services...</p>
      </div>
    </div>
  `;
}

bootstrap();
