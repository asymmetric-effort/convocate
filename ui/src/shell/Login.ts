import { createElement, useState } from "@asymmetric-effort/specifyjs";
import { Card, TextField, Button } from "@asymmetric-effort/specifyjs/components";
import { login } from "../lib/auth";

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
      await login({ username, password, mfaToken });
      onSuccess();
    } catch (err: any) {
      setError(err?.message || "User login failed.");
    } finally {
      setLoading(false);
    }
  }

  return h("div", {
    style: {
      width: "100%", height: "100%", display: "flex",
      alignItems: "center", justifyContent: "center",
      background: "#000",
    },
  },
    h("form", { onSubmit: handleSubmit, style: { width: "360px" } },
      h(Card, { style: { padding: "40px", textAlign: "center" } },
        h("h1", { style: { fontSize: "24px", fontWeight: 300, marginBottom: "24px", color: "#fff" } }, "Convocate"),
        h(TextField, {
          placeholder: "Username",
          value: username,
          onChange: (val: string) => setUsername(val),
        }),
        h("div", { style: { height: "12px" } }),
        h(TextField, {
          placeholder: "Password",
          type: "password",
          value: password,
          onChange: (val: string) => setPassword(val),
        }),
        h("div", { style: { height: "12px" } }),
        h(TextField, {
          placeholder: "MFA Token",
          value: mfaToken,
          onChange: (val: string) => setMfaToken(val),
        }),
        h("div", { style: { height: "16px" } }),
        h(Button, {
          type: "submit" as const,
          variant: "primary",
          disabled: loading,
          fullWidth: true,
        }, loading ? "Signing in..." : "Sign In"),
        error ? h("div", { style: { color: "#e55", fontSize: "13px", marginTop: "12px" } }, error) : null
      )
    )
  );
}
