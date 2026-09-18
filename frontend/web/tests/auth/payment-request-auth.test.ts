import assert from "node:assert/strict";
import test from "node:test";

import { fetchPublicPaymentRequest } from "../../lib/api/payment-requests";
import { nimiqHubReturnUrl } from "../../lib/auth/nimiq";
import { consumePaymentRequestResume, markPaymentRequestResume } from "../../lib/payment-requests/pay";

test("public payment request fetch does not send credentials-required Authorization", async () => {
  const previous = globalThis.fetch;
  let init: RequestInit | undefined;
  globalThis.fetch = (async (_input: URL | RequestInfo, requestInit?: RequestInit) => {
    init = requestInit;
    return new Response(
      JSON.stringify({
        data: {
          public_id: "abc",
          recipient: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
          amount_lunas: "1",
          amount_nim: "0.00001",
          note: null,
          status: "pending",
          expires_at: "2099-01-01T00:00:00Z",
          network: "test-albatross",
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;
  try {
    const request = await fetchPublicPaymentRequest("abc");
    assert.equal(request.public_id, "abc");
    assert.equal(new Headers(init?.headers).has("Authorization"), false);
  } finally {
    globalThis.fetch = previous;
  }
});

test("Hub return URL preserves /pay/{public_id} and does not add secrets", () => {
  const previous = globalThis.window;
  globalThis.window = {
    location: {
      origin: "https://nimnear.example",
      pathname: "/pay/abc",
      search: "",
    },
  } as never;
  try {
    const url = nimiqHubReturnUrl();
    assert.equal(url, "https://nimnear.example/pay/abc");
    assert.equal(url.includes("token="), false);
    assert.equal(url.includes("/wallet"), false);
  } finally {
    globalThis.window = previous;
  }
});

test("pay resume after authentication stays on the same public id", () => {
  const storage = new Map<string, string>();
  const sessionStorage = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => { storage.set(key, value); },
    removeItem: (key: string) => { storage.delete(key); },
  };
  markPaymentRequestResume("abc", sessionStorage);
  assert.equal(consumePaymentRequestResume("abc", sessionStorage), true);
});
