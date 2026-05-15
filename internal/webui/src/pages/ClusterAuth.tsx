import { createElement, useState, useEffect } from "@asymmetric-effort/specifyjs";
import { api } from "../api/client";

export function ClusterAuth() {
  const [mode, setMode] = useState<"anthropic_api_key" | "claude_session">("anthropic_api_key");
  const [apiKey, setApiKey] = useState("");
  const [sessionToken, setSessionToken] = useState("");
  const [currentMode, setCurrentMode] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    api.getClusterAuth()
      .then((resp) => setCurrentMode(resp.mode || "none"))
      .catch((err: Error) => setError(err.message));
  }, []);

  const handleSubmit = async () => {
    setError("");
    setSuccess("");
    setSubmitting(true);
    try {
      await api.setClusterAuth({
        mode,
        api_key: mode === "anthropic_api_key" ? apiKey : undefined,
        session_token: mode === "claude_session" ? sessionToken : undefined,
      });
      setSuccess("Cluster authentication updated.");
      setCurrentMode(mode);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setSubmitting(false);
    }
  };

  return createElement("div", { className: "cluster-auth" },
    createElement("h1", null, "Cluster Authentication"),
    createElement("p", null, `Current mode: ${currentMode || "loading..."}`),

    error ? createElement("div", { className: "error" }, error) : null,
    success ? createElement("div", { className: "success" }, success) : null,

    createElement("div", { className: "form-group" },
      createElement("label", null, "Authentication Mode"),
      createElement("select", {
        value: mode,
        onChange: (e: Event) => setMode((e.target as HTMLSelectElement).value as typeof mode),
      },
        createElement("option", { value: "anthropic_api_key" }, "Anthropic API Key"),
        createElement("option", { value: "claude_session" }, "Claude.ai Session"),
      ),
    ),

    mode === "anthropic_api_key"
      ? createElement("div", { className: "form-group" },
          createElement("label", null, "Anthropic API Key"),
          createElement("input", {
            type: "password",
            value: apiKey,
            placeholder: "sk-ant-api03-...",
            onInput: (e: Event) => setApiKey((e.target as HTMLInputElement).value),
          }),
        )
      : createElement("div", { className: "form-group" },
          createElement("label", null, "Claude.ai Session Token"),
          createElement("input", {
            type: "password",
            value: sessionToken,
            placeholder: "session token",
            onInput: (e: Event) => setSessionToken((e.target as HTMLInputElement).value),
          }),
        ),

    createElement("button", {
      onClick: handleSubmit,
      disabled: submitting,
    }, submitting ? "Updating..." : "Update Authentication"),
  );
}
