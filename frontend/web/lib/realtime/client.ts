import {
  isKnownRealtimeEvent,
  REALTIME_RECONNECTED,
  type RealtimeEvent,
} from "./events";
import { websocketUrl, websocketUrlHasSecret } from "./url";

export const REALTIME_RECONNECT_DELAYS_MS = [1000, 2000, 4000, 8000, 15000] as const;

export type RealtimeListener = {
  onEvent?: (event: RealtimeEvent) => void;
  onReconnect?: () => void;
  onDisconnect?: () => void;
};

type SocketLike = {
  readyState: number;
  close(): void;
  send(data: string): void;
  addEventListener(type: string, listener: (event: MessageEvent<string> | Event) => void): void;
  removeEventListener(type: string, listener: (event: MessageEvent<string> | Event) => void): void;
};

export function nextRealtimeReconnectDelay(attempt: number): number {
  return REALTIME_RECONNECT_DELAYS_MS[Math.min(attempt, REALTIME_RECONNECT_DELAYS_MS.length - 1)] ?? 15000;
}

class RealtimeClient {
  private socket: SocketLike | null = null;
  private listeners = new Set<RealtimeListener>();
  private reconnectAttempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private opened = false;
  private closed = false;
  private connecting = false;

  addListener(listener: RealtimeListener) {
    this.listeners.add(listener);
    this.closed = false;
    this.connect();
    return () => {
      this.listeners.delete(listener);
      if (this.listeners.size === 0) this.disconnect();
    };
  }

  connect() {
    if (this.closed || this.connecting) return;
    if (this.socket && (this.socket.readyState === 0 || this.socket.readyState === 1)) return;
    if (typeof WebSocket === "undefined") return;
    const url = websocketUrl();
    if (websocketUrlHasSecret(url)) {
      throw new Error("WebSocket URL must not include authentication secrets");
    }
    this.connecting = true;
    const socket = new WebSocket(url) as unknown as SocketLike;
    this.socket = socket;
    const onOpen = () => {
      this.connecting = false;
      const reconnected = this.opened;
      this.opened = true;
      this.reconnectAttempt = 0;
      if (reconnected) {
        for (const listener of this.listeners) listener.onReconnect?.();
      }
    };
    const onMessage = (event: MessageEvent<string> | Event) => {
      const data = "data" in event ? event.data : "";
      if (typeof data !== "string") return;
      let parsed: RealtimeEvent;
      try {
        parsed = JSON.parse(data) as RealtimeEvent;
      } catch {
        return;
      }
      if (!parsed?.type || parsed.type === "pong" || parsed.type === "subscribed" || parsed.type === "error") {
        return;
      }
      if (!isKnownRealtimeEvent(parsed.type)) return;
      for (const listener of this.listeners) listener.onEvent?.(parsed);
    };
    const onClose = () => {
      this.connecting = false;
      socket.removeEventListener("open", onOpen);
      socket.removeEventListener("message", onMessage);
      socket.removeEventListener("close", onClose);
      socket.removeEventListener("error", onClose);
      if (this.socket === socket) this.socket = null;
      for (const listener of this.listeners) listener.onDisconnect?.();
      this.scheduleReconnect();
    };
    socket.addEventListener("open", onOpen);
    socket.addEventListener("message", onMessage);
    socket.addEventListener("close", onClose);
    socket.addEventListener("error", onClose);
  }

  disconnect() {
    this.closed = true;
    this.connecting = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    const socket = this.socket;
    this.socket = null;
    socket?.close();
  }

  private scheduleReconnect() {
    if (this.closed || this.listeners.size === 0) return;
    if (this.reconnectTimer) return;
    const delay = nextRealtimeReconnectDelay(this.reconnectAttempt);
    this.reconnectAttempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }
}

let shared: RealtimeClient | null = null;

export function subscribeRealtime(listener: RealtimeListener): () => void {
  if (!shared) shared = new RealtimeClient();
  return shared.addListener(listener);
}

export function resetRealtimeClientForTests() {
  shared?.disconnect();
  shared = null;
}

export function reconnectEvent(): RealtimeEvent {
  return { type: REALTIME_RECONNECTED };
}
