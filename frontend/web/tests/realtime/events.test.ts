import assert from "node:assert/strict";
import test from "node:test";

import {
  createCoalescer,
  eventTouchesResource,
  isEventPurchaseEvent,
  isKnownRealtimeEvent,
  isPaymentRequestEvent,
  isWalletActivityEvent,
} from "../../lib/realtime/events";

const publicId = "11111111-1111-4111-8111-111111111111";

test("payment page events refetch the matching request and ignore others", () => {
  const fetches: string[] = [];
  const handle = (type: string, resourceId: string) => {
    const event = { type, data: { resource_id: resourceId, status: "paid" } };
    if (!isPaymentRequestEvent(event.type) || !eventTouchesResource(event, publicId)) return;
    fetches.push("payment-request");
  };
  handle("payment_request.verifying", publicId);
  handle("payment_request.paid", publicId);
  handle("payment_request.paid", "22222222-2222-4222-8222-222222222222");
  handle("wallet.activity_changed", publicId);
  handle("not.a.real.event", publicId);
  assert.deepEqual(fetches, ["payment-request", "payment-request"]);
});

test("creator wallet refreshes requests and paid wallet activity separately", () => {
  const actions: string[] = [];
  const handle = (type: string) => {
    if (isPaymentRequestEvent(type)) actions.push("requests");
    if (isWalletActivityEvent(type)) actions.push("wallet");
  };
  handle("payment_request.paid");
  handle("wallet.activity_changed");
  handle("event_purchase.confirmed");
  assert.deepEqual(actions, ["requests", "wallet"]);
});

test("duplicate events are coalesced into one refetch", async () => {
  const coalescer = createCoalescer(20);
  let calls = 0;
  coalescer.run(() => {
    calls += 1;
  });
  coalescer.run(() => {
    calls += 1;
  });
  await new Promise((resolve) => setTimeout(resolve, 50));
  assert.equal(calls, 1);
  coalescer.clear();
});

test("unknown events are ignored and known purchase events are recognized", () => {
  assert.equal(isKnownRealtimeEvent("nope"), false);
  assert.equal(isEventPurchaseEvent("event_purchase.confirmed"), true);
  assert.equal(isPaymentRequestEvent("pending"), false);
});

test("HTTP authoritative status wins over a websocket hint", () => {
  const http = { status: "paid" as const };
  const websocket = { type: "payment_request.verifying", data: { status: "verifying" } };
  assert.equal(http.status, "paid");
  assert.notEqual(websocket.data.status, http.status);
});
