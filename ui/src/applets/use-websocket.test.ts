import { test, expect, describe, beforeEach, mock } from "bun:test";
import type { ServerEvent } from "./use-websocket";

/**
 * useWebSocket uses SpecifyJS hooks (useState, useEffect, useRef, useCallback)
 * and the browser EventSource API. Since Bun tests have no DOM or component
 * lifecycle, we test:
 *   1. The ServerEvent type structure
 *   2. The URL-building logic (extracted/simulated)
 *   3. The event-parsing logic (extracted/simulated)
 *   4. The reconnection/error handling logic
 */

// Mock localStorage
const storage: Record<string, string> = {};
(globalThis as any).localStorage = {
  getItem: (key: string) => storage[key] ?? null,
  setItem: (key: string, value: string) => { storage[key] = value; },
  removeItem: (key: string) => { delete storage[key]; },
};

// ---------- Helper: replicate the URL-building logic from useWebSocket ----------
function buildSSEUrl(path: string, filter: string[] | null, token: string | null): string {
  const params: string[] = [];
  if (token) params.push(`token=${encodeURIComponent(token)}`);
  if (filter && filter.length > 0) params.push(`filter=${filter.join(",")}`);
  return path + (params.length > 0 ? `?${params.join("&")}` : "");
}

// ---------- Helper: replicate event-parsing logic ----------
function parseEvent(data: string): ServerEvent | null {
  try {
    return JSON.parse(data) as ServerEvent;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------
// ServerEvent type
// ---------------------------------------------------------------------
describe("ServerEvent type structure", () => {
  test("accepts a well-formed event object", () => {
    const evt: ServerEvent = {
      type: "node.metrics",
      timestamp: "2026-07-01T12:00:00Z",
      payload: { cpu: 42, mem: 1024 },
    };
    expect(evt.type).toBe("node.metrics");
    expect(evt.timestamp).toBe("2026-07-01T12:00:00Z");
    expect(evt.payload.cpu).toBe(42);
  });

  test("accepts null payload", () => {
    const evt: ServerEvent = { type: "ping", timestamp: "2026-07-01T12:00:00Z", payload: null };
    expect(evt.payload).toBeNull();
  });
});

// ---------------------------------------------------------------------
// URL building logic
// ---------------------------------------------------------------------
describe("SSE URL building", () => {
  test("path only when no token and no filter", () => {
    expect(buildSSEUrl("/api/v1/events/nmgr/status", null, null))
      .toBe("/api/v1/events/nmgr/status");
  });

  test("appends token query param", () => {
    expect(buildSSEUrl("/api/v1/events/nmgr/status", null, "abc123"))
      .toBe("/api/v1/events/nmgr/status?token=abc123");
  });

  test("appends filter query param", () => {
    expect(buildSSEUrl("/api/v1/events/nmgr/status", ["node.metrics"], null))
      .toBe("/api/v1/events/nmgr/status?filter=node.metrics");
  });

  test("appends both token and filter", () => {
    const url = buildSSEUrl("/api/v1/events/nmgr/status", ["a", "b"], "tok");
    expect(url).toBe("/api/v1/events/nmgr/status?token=tok&filter=a,b");
  });

  test("encodes token with special characters", () => {
    const url = buildSSEUrl("/events", null, "a b+c");
    expect(url).toContain("token=a%20b%2Bc");
  });

  test("empty filter array treated as no filter", () => {
    expect(buildSSEUrl("/events", [], "tok")).toBe("/events?token=tok");
  });
});

// ---------------------------------------------------------------------
// Event parsing logic
// ---------------------------------------------------------------------
describe("event parsing", () => {
  test("parses valid JSON event", () => {
    const evt = parseEvent('{"type":"node.up","timestamp":"2026-07-01T00:00:00Z","payload":{}}');
    expect(evt).not.toBeNull();
    expect(evt!.type).toBe("node.up");
  });

  test("returns null for malformed JSON", () => {
    expect(parseEvent("not json")).toBeNull();
  });

  test("returns null for empty string", () => {
    expect(parseEvent("")).toBeNull();
  });

  test("parses event with nested payload", () => {
    const data = JSON.stringify({
      type: "agent.status",
      timestamp: "2026-07-01T00:00:00Z",
      payload: { agentId: "a1", status: "running", metrics: { cpu: 50 } },
    });
    const evt = parseEvent(data);
    expect(evt!.payload.metrics.cpu).toBe(50);
  });
});

// ---------------------------------------------------------------------
// EventSource mock — simulate the lifecycle
// ---------------------------------------------------------------------
describe("EventSource lifecycle (simulated)", () => {
  let onopen: (() => void) | null;
  let onmessage: ((e: { data: string }) => void) | null;
  let onerror: (() => void) | null;
  let closeFn: ReturnType<typeof mock>;

  beforeEach(() => {
    onopen = null;
    onmessage = null;
    onerror = null;
    closeFn = mock(() => {});

    // Simulate what useWebSocket does with EventSource
    (globalThis as any).EventSource = class {
      url: string;
      constructor(url: string) {
        this.url = url;
      }
      set onopen(fn: any) { onopen = fn; }
      set onmessage(fn: any) { onmessage = fn; }
      set onerror(fn: any) { onerror = fn; }
      close() { closeFn(); }
    };
  });

  test("onopen sets connected state", () => {
    let connected = false;
    const setConnected = (v: boolean) => { connected = v; };

    storage["accessToken"] = "tok";
    const es = new (globalThis as any).EventSource("/api/v1/events/nmgr/status?token=tok");
    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);

    // Simulate open
    onopen!();
    expect(connected).toBe(true);
  });

  test("onmessage parses and dispatches event", () => {
    const received: ServerEvent[] = [];
    const handler = (evt: ServerEvent) => received.push(evt);

    const es = new (globalThis as any).EventSource("/events");
    es.onmessage = (e: { data: string }) => {
      try {
        const evt: ServerEvent = JSON.parse(e.data);
        handler(evt);
      } catch { /* ignore */ }
    };

    onmessage!({ data: '{"type":"test","timestamp":"2026-07-01T00:00:00Z","payload":null}' });
    expect(received).toHaveLength(1);
    expect(received[0].type).toBe("test");
  });

  test("onmessage ignores malformed data", () => {
    const received: ServerEvent[] = [];

    const es = new (globalThis as any).EventSource("/events");
    es.onmessage = (e: { data: string }) => {
      try {
        received.push(JSON.parse(e.data));
      } catch { /* ignore malformed */ }
    };

    onmessage!({ data: "not-json" });
    expect(received).toHaveLength(0);
  });

  test("onerror sets connected to false", () => {
    let connected = true;
    const setConnected = (v: boolean) => { connected = v; };

    const es = new (globalThis as any).EventSource("/events");
    es.onerror = () => setConnected(false);

    onerror!();
    expect(connected).toBe(false);
  });

  test("cleanup calls close", () => {
    const es = new (globalThis as any).EventSource("/events");
    es.close();
    expect(closeFn).toHaveBeenCalledTimes(1);
  });
});
