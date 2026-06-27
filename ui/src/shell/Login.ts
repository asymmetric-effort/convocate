import { createElement, useState } from "@asymmetric-effort/specifyjs";
import { login } from "../lib/auth";
import type { LoginRequest } from "../types/api";

const h = createElement;

export function Login({ onSuccess }: { onSuccess: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [mfaToken, setMfaToken] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login({ username, password, mfaToken } as LoginRequest);
      onSuccess();
    } catch (err: any) {
      setError(err?.message || "User login failed.");
    } finally {
      setLoading(false);
    }
  }

  return h("div", { className: "login-overlay" },
    h("form", { className: "login-form", onSubmit: handleSubmit },
      h("h1", null, "Convocate"),
      h("input", {
        type: "text",
        placeholder: "Username",
        value: username,
        onInput: (e: any) => setUsername(e.target.value),
        autoComplete: "username",
      }),
      h("input", {
        type: "password",
        placeholder: "Password",
        value: password,
        onInput: (e: any) => setPassword(e.target.value),
        autoComplete: "current-password",
      }),
      h("input", {
        type: "text",
        placeholder: "MFA Token",
        value: mfaToken,
        onInput: (e: any) => setMfaToken(e.target.value),
        maxLength: 6,
        pattern: "[0-9]{6}",
      }),
      h("button", { type: "submit", disabled: loading },
        loading ? "Signing in..." : "Sign In"
      ),
      error ? h("div", { className: "error" }, error) : null
    )
  );
}
