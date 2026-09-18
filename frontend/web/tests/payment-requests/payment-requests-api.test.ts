import assert from "node:assert/strict";
import test from "node:test";

import { withAuthSession } from "../../lib/api/session-request";
import {
  cancelPaymentRequest,
  createPaymentRequest,
  fetchPublicPaymentRequest,
  listPaymentRequests,
  PaymentRequestsApiError,
  submitPaymentRequestTransaction,
} from "../../lib/api/payment-requests";

test("payment request API client sends cookies and typed payloads", async () => {
  const previousFetch = globalThis.fetch;
  const calls: Array<{ url: string; init?: RequestInit }> = [];

  globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(
      JSON.stringify({
        data: {
          public_id: "11111111-1111-4111-8111-111111111111",
          recipient: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
          amount_lunas: "2500000",
          amount_nim: "25",
          note: "Dinner",
          status: "pending",
          expires_at: "2026-09-18T12:00:00Z",
          network: "test-albatross",
          created_at: "2026-09-17T12:00:00Z",
          updated_at: "2026-09-17T12:00:00Z",
        },
      }),
      { status: 201, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const created = await createPaymentRequest({
      amount_nim: "25",
      note: "Dinner",
    });
    assert.equal(created.amount_lunas, "2500000");
    assert.equal(created.status, "pending");
    assert.equal(calls[0]?.init?.credentials, "include");
    assert.equal(
      JSON.parse(String(calls[0]?.init?.body)).amount_nim,
      "25",
    );
    assert.equal(
      JSON.parse(String(calls[0]?.init?.body)).creator_user_id,
      undefined,
    );
    assert.equal(
      JSON.parse(String(calls[0]?.init?.body)).recipient,
      undefined,
    );

    globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      return new Response(
        JSON.stringify({
          data: [
            {
              public_id: created.public_id,
              recipient: created.recipient,
              amount_lunas: created.amount_lunas,
              amount_nim: created.amount_nim,
              note: created.note,
              status: created.status,
              expires_at: created.expires_at,
              network: created.network,
              created_at: created.created_at,
              updated_at: created.updated_at,
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const listed = await listPaymentRequests(undefined, 20);
    assert.equal(listed[0]?.public_id, created.public_id);
    assert.ok(String(calls[1]?.url).includes("/api/v1/payment-requests"));
    assert.ok(String(calls[1]?.url).includes("limit=20"));

    globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      return new Response(
        JSON.stringify({
          data: {
            public_id: created.public_id,
            recipient: created.recipient,
            amount_lunas: created.amount_lunas,
            amount_nim: created.amount_nim,
            note: created.note,
            status: created.status,
            expires_at: created.expires_at,
            network: created.network,
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const publicRequest = await fetchPublicPaymentRequest(created.public_id);
    assert.equal(publicRequest.public_id, created.public_id);
    assert.equal(
      "creator_user_id" in (publicRequest as object),
      false,
    );
    assert.ok(
      String(calls[2]?.url).includes("/api/v1/public/payment-requests/"),
    );

    globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      return new Response(
        JSON.stringify({
          data: { ...created, status: "cancelled" },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const cancelled = await cancelPaymentRequest(created.public_id);
    assert.equal(cancelled.status, "cancelled");
    assert.equal(calls[3]?.init?.method, "POST");

    globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      return new Response(
        JSON.stringify({
          error: "Conflict",
          error_code: "payment_request_already_paid",
          message: "payment request is already paid",
          code: 409,
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    await assert.rejects(
      () =>
        submitPaymentRequestTransaction(
          created.public_id,
          "ab".repeat(32),
        ),
      (error: unknown) => {
        assert.ok(error instanceof PaymentRequestsApiError);
        assert.equal(error.status, 409);
        assert.equal(error.errorCode, "payment_request_already_paid");
        return true;
      },
    );
    assert.deepEqual(JSON.parse(String(calls[4]?.init?.body)), {
      transaction_hash: "ab".repeat(32),
    });
    assert.equal(calls[4]?.init?.credentials, "include");
  } finally {
    globalThis.fetch = previousFetch;
  }
});

test("payment request API helper uses authenticated session cookies", () => {
  const init = withAuthSession(undefined, { method: "POST" });
  assert.equal(init.credentials, "include");
  assert.equal(new Headers(init.headers).has("Authorization"), false);
});
