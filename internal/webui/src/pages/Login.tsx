import { Component } from "@asymmetric-effort/specifyjs";

export class Login extends Component<Record<string, never>, Record<string, never>> {
  state = {};

  render() {
    return (
      <div className="login-page">
        <div className="login-card">
          <h1>convocate</h1>
          <p>Sign in to access the management console.</p>
          <a href="/auth/login" className="login-button">
            Sign in with GitHub
          </a>
        </div>
      </div>
    );
  }
}
