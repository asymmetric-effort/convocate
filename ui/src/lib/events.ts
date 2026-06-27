import { useState, useEffect, useRef } from "@asymmetric-effort/specifyjs";
import type { WSEvent } from "../types/api";
import { getAccessToken } from "./api";

export function useEventChannel(
  applet: string,
  channel: string,
  onEvent?: (event: WSEvent) => void
): { connected: boolean; lastEvent: WSEvent | null } {
  const [connected, setConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<WSEvent | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const token = getAccessToken();
    if (!token) return;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${protocol}//${window.location.host}/api/v1/events/${applet}/${channel}`;

    function connect() {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => setConnected(true);
      ws.onclose = () => {
        setConnected(false);
        setTimeout(connect, 3000);
      };
      ws.onerror = () => ws.close();
      ws.onmessage = (e) => {
        try {
          const event: WSEvent = JSON.parse(e.data);
          setLastEvent(event);
          onEvent?.(event);
        } catch {}
      };
    }

    connect();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [applet, channel]);

  return { connected, lastEvent };
}
