/**
 * useEventStream — custom hook for subscribing to the Convocate events
 * endpoint via Server-Sent Events (SSE).  SSE works reliably through
 * HTTP/2 proxies where WebSocket upgrade may fail.
 *
 * The hook handles authentication via ?token= query param, optional
 * server-side type filtering, automatic reconnection (built into
 * EventSource), and cleanup on unmount.
 *
 * Usage:
 *   const { connected } = useWebSocket(
 *     "/api/v1/events/nmgr/status",
 *     ["node.metrics"],
 *     (evt) => { ... }
 *   );
 */

import { useState, useEffect, useRef, useCallback } from "@asymmetric-effort/specifyjs";

/** Parsed server event */
export interface ServerEvent {
  type: string;
  timestamp: string;
  payload: any;
}

/**
 * Subscribe to a Convocate events channel via SSE.
 *
 * @param path     — event endpoint path (e.g. "/api/v1/events/nmgr/status")
 * @param filter   — optional array of event types to receive (server-side filter)
 * @param onMessage — callback invoked for each received event
 * @returns {{ connected: boolean }} — live connection status
 */
export function useWebSocket(
  path: string,
  filter: string[] | null,
  onMessage: (event: ServerEvent) => void
): { connected: boolean } {
  const [connected, setConnected] = useState(false);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  useEffect(() => {
    // Build the SSE URL with auth token and optional type filter
    const token = localStorage.getItem("accessToken");
    const params: string[] = [];
    if (token) params.push(`token=${encodeURIComponent(token)}`);
    if (filter && filter.length > 0) params.push(`filter=${filter.join(",")}`);
    const url = path + (params.length > 0 ? `?${params.join("&")}` : "");

    const es = new EventSource(url);

    es.onopen = () => {
      setConnected(true);
    };

    es.onmessage = (e: MessageEvent) => {
      try {
        const evt: ServerEvent = JSON.parse(e.data);
        onMessageRef.current(evt);
      } catch {
        // Ignore malformed messages
      }
    };

    es.onerror = () => {
      setConnected(false);
      // EventSource reconnects automatically
    };

    return () => {
      es.close();
      setConnected(false);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, filter && filter.join(",")]);

  return { connected };
}
