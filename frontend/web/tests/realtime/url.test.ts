import assert from "node:assert/strict";
import test from "node:test";

import { apiBaseUrl } from "../../lib/api/events";
import { websocketUrl, websocketUrlHasSecret } from "../../lib/realtime/url";

test("websocket URL uses the API host and never includes a JWT", () => {
  const url = websocketUrl("http://localhost:8080");
  assert.equal(url, "ws://localhost:8080/api/v1/ws");
  assert.equal(websocketUrlHasSecret(url), false);
  assert.equal(websocketUrl("https://api.example/").startsWith("wss://"), true);
  assert.equal(new URL(url).search, "");
  assert.equal(url.includes(apiBaseUrl) || url.includes("localhost:8080"), true);
});

test("secret query strings are rejected", () => {
  assert.equal(websocketUrlHasSecret("ws://localhost:8080/api/v1/ws?token=jwt-secret"), true);
  assert.equal(websocketUrlHasSecret("ws://localhost:8080/api/v1/ws"), false);
});
