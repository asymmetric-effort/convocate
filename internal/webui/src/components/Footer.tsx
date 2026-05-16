import { Component } from "@asymmetric-effort/specifyjs";

export class Footer extends Component<Record<string, never>, Record<string, never>> {
  state = {};

  render() {
    const year = new Date().getFullYear();
    return (
      <footer className="app-footer">
        <span>convocate v2.2.0</span>
        <span>© {String(year)} Asymmetric Effort, LLC</span>
        <span>MIT License</span>
      </footer>
    );
  }
}
