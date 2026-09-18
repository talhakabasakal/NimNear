import assert from "node:assert/strict";
import test from "node:test";

import {
  cancelPaymentRequest,
  listPaymentRequests,
  PaymentRequestsApiError,
  type PaymentRequestRecord,
} from "../../lib/api/payment-requests";
import { canCancelPaymentRequest } from "../../lib/payment-requests/status";

function record(status: PaymentRequestRecord["status"]): PaymentRequestRecord {
  return {
    public_id: "11111111-1111-4111-8111-111111111111",
    recipient: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
    amount_lunas: "2500000",
    amount_nim: "25",
    note: "Dinner",
    status,
    expires_at: "2026-09-18T12:00:00Z",
    network: "test-albatross",
    created_at: "2026-09-17T12:00:00Z",
    updated_at: "2026-09-17T12:00:00Z",
  };
}

test("creator list returns recent requests with public statuses", async () => {
  const previous = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(
      JSON.stringify({
        data: [record("pending"), { ...record("paid"), amount_nim: "10" }, { ...record("expired"), amount_nim: "5" }],
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    )) as typeof fetch;
  try {
    const listed = await listPaymentRequests(undefined, 20);
    assert.equal(listed.length, 3);
    assert.deepEqual(listed.map((item) => item.status), ["pending", "paid", "expired"]);
    assert.equal("id" in listed[0], false);
    assert.equal("creator_user_id" in listed[0], false);
  } finally {
    globalThis.fetch = previous;
  }
});

test("pending requests can be cancelled through the API", async () => {
  assert.equal(canCancelPaymentRequest("pending"), true);
  const previous = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(JSON.stringify({ data: record("cancelled") }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    })) as typeof fetch;
  try {
    const cancelled = await cancelPaymentRequest(record("pending").public_id);
    assert.equal(cancelled.status, "cancelled");
  } finally {
    globalThis.fetch = previous;
  }
});

test("terminal requests cannot be cancelled locally and follow backend conflict", async () => {
  assert.equal(canCancelPaymentRequest("paid"), false);
  assert.equal(canCancelPaymentRequest("submitted"), false);
  assert.equal(canCancelPaymentRequest("verifying"), false);
  const previous = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(
      JSON.stringify({
        error_code: "payment_request_not_cancellable",
        message: "payment request can no longer be cancelled",
      }),
      { status: 409, headers: { "Content-Type": "application/json" } },
    )) as typeof fetch;
  try {
    await assert.rejects(
      () => cancelPaymentRequest(record("paid").public_id),
      (error: unknown) =>
        error instanceof PaymentRequestsApiError &&
        error.status === 409 &&
        error.errorCode === "payment_request_not_cancellable",
    );
  } finally {
    globalThis.fetch = previous;
  }
});
