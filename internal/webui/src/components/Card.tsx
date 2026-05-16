import { Component } from "@asymmetric-effort/specifyjs";

interface CardProps {
  title: string;
  children?: unknown;
}

export class Card extends Component<CardProps, Record<string, never>> {
  state = {};

  render() {
    const { title, children } = this.props;
    return (
      <div className="card">
        <div className="card-header">{title}</div>
        <div className="card-body">{children}</div>
      </div>
    );
  }
}
