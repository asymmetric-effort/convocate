declare module "@asymmetric-effort/specifyjs" {
  export class Component<TProps = Record<string, never>, TState = Record<string, unknown>> {
    props: TProps;
    state: TState;
    setState(partial: Partial<TState>): void;
    componentDidMount?(): void;
    componentWillUnmount?(): void;
    render(): unknown;
  }
}

declare module "@asymmetric-effort/specifyjs/dom" {
  export function createRoot(container: HTMLElement): {
    render(element: unknown): void;
  };
}

declare module "@asymmetric-effort/specifyjs/jsx-runtime" {
  export function jsx(type: unknown, props: unknown, key?: unknown): unknown;
  export function jsxs(type: unknown, props: unknown, key?: unknown): unknown;
  export const Fragment: unique symbol;
}

declare namespace JSX {
  type Element = unknown;
  interface IntrinsicElements {
    [elemName: string]: Record<string, unknown>;
  }
}
