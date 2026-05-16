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
