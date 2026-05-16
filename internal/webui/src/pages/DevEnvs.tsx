import { Component } from "@asymmetric-effort/specifyjs";

interface DevEnvsProps {
  activeSideNav: string;
}

export class DevEnvs extends Component<DevEnvsProps, Record<string, never>> {
  state = {};

  render() {
    return (
      <div>
        <h1>Dev Environments</h1>
        <p>Development environment management coming soon.</p>
      </div>
    );
  }
}
