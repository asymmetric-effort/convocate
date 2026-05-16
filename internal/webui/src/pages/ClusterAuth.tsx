import { useState, useEffect } from "@asymmetric-effort/specifyjs";
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

  return (
    <div className="cluster-auth">
      <h1>Cluster Authentication</h1>
      <p>Current mode: {currentMode || "loading..."}</p>

      {error ? <div className="error">{error}</div> : null}
      {success ? <div className="success">{success}</div> : null}

      <div className="form-group">
        <label>Authentication Mode</label>
        <select
          value={mode}
          onChange={(e: Event) => setMode((e.target as HTMLSelectElement).value as typeof mode)}
        >
          <option value="anthropic_api_key">Anthropic API Key</option>
          <option value="claude_session">Claude.ai Session</option>
        </select>
      </div>

      {mode === "anthropic_api_key" ? (
        <div className="form-group">
          <label>Anthropic API Key</label>
          <input
            type="password"
            value={apiKey}
            placeholder="sk-ant-api03-..."
            onInput={(e: Event) => setApiKey((e.target as HTMLInputElement).value)}
          />
        </div>
      ) : (
        <div className="form-group">
          <label>Claude.ai Session Token</label>
          <input
            type="password"
            value={sessionToken}
            placeholder="session token"
            onInput={(e: Event) => setSessionToken((e.target as HTMLInputElement).value)}
          />
        </div>
      )}

      <button onClick={handleSubmit} disabled={submitting}>
        {submitting ? "Updating..." : "Update Authentication"}
      </button>
    </div>
  );
}
