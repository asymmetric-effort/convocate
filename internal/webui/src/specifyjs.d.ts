declare module "@asymmetric-effort/specifyjs" {
  export function useState<T>(
    initial: T | (() => T)
  ): [T, (value: T | ((prev: T) => T)) => void];
  export function useEffect(
    effect: () => void | (() => void),
    deps?: unknown[]
  ): void;
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
