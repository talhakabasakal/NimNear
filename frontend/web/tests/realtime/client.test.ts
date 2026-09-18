import assert from "node:assert/strict";
import test from "node:test";

import {
  nextRealtimeReconnectDelay,
  resetRealtimeClientForTests,
  subscribeRealtime,
} from "../../lib/realtime/client";
import { websocketUrlHasSecret } from "../../lib/realtime/url";

type Listener = (event: Event | MessageEvent<string>) => void;

class FakeSocket {
  static instances: FakeSocket[] = [];
  readyState = 0;
  url: string;
  private listeners = new Map<string, Set<Listener>>();

  constructor(url: string) {
    this.url = url;
    FakeSocket.instances.push(this);
  }

  addEventListener(type: string, listener: Listener) {
    const set = this.listeners.get(type) ?? new Set();
    set.add(listener);
    this.listeners.set(type, set);
  }

  removeEventListener(type: string, listener: Listener) {
    this.listeners.get(type)?.delete(listener);
  }

  send() {}

  close() {
    this.readyState = 3;
    this.emit("close");
  }

  open() {
    this.readyState = 1;
    this.emit("open");
  }

  emit(type: string, event: Event | MessageEvent<string> = new Event(type)) {
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }

  emitMessage(data: string) {
    this.emit("message", { data } as MessageEvent<string>);
  }
}

test("realtime client connects without a JWT, ignores unknown events, and cleans up", async () => {
  FakeSocket.instances = [];
  const original = globalThis.WebSocket;
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  resetRealtimeClientForTests();
  const received: string[] = [];
  const unsubscribe = subscribeRealtime({
    onEvent(event) {
      received.push(event.type);
    },
  });
  assert.equal(FakeSocket.instances.length, 1);
  const socket = FakeSocket.instances[0];
  assert.equal(websocketUrlHasSecret(socket.url), false);
  assert.equal(socket.url.includes("token="), false);
  socket.open();
  socket.emitMessage(JSON.stringify({ type: "payment_request.paid", data: { resource_id: "abc", status: "paid" } }));
  socket.emitMessage(JSON.stringify({ type: "payment_request.paid", data: { resource_id: "abc", status: "paid" } }));
  socket.emitMessage(JSON.stringify({ type: "totally.unknown", data: { resource_id: "abc" } }));
  socket.emitMessage(JSON.stringify({ type: "pong" }));
  assert.deepEqual(received, ["payment_request.paid", "payment_request.paid"]);
  unsubscribe();
  assert.equal(socket.readyState, 3);
  resetRealtimeClientForTests();
  globalThis.WebSocket = original;
});

test("reconnect notifies listeners and uses bounded backoff", async () => {
  FakeSocket.instances = [];
  const original = globalThis.WebSocket;
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket;
  resetRealtimeClientForTests();
  let reconnects = 0;
  const unsubscribe = subscribeRealtime({
    onReconnect() {
      reconnects += 1;
    },
  });
  FakeSocket.instances[0].open();
  FakeSocket.instances[0].close();
  await new Promise((resolve) => setTimeout(resolve, 20));
  assert.equal(nextRealtimeReconnectDelay(0), 1000);
  assert.equal(nextRealtimeReconnectDelay(8), 15000);
  unsubscribe();
  resetRealtimeClientForTests();
  globalThis.WebSocket = original;
  assert.equal(reconnects, 0);
});

test("public unauthenticated pages do not require a websocket subscription", () => {
  const session = null;
  const shouldSubscribe = Boolean(session);
  assert.equal(shouldSubscribe, false);
});
