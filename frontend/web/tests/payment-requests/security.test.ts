import assert from "node:assert/strict";
import test from "node:test";

import { fetchPublicPaymentRequest } from "../../lib/api/payment-requests";
import { paymentRequestNoteText, publicPaymentRequestHasPrivateFields } from "../../lib/payment-requests/pay";
import { paymentRequestShareUrl, shareUrlContainsSecrets } from "../../lib/payment-requests/share";
import { withAuthSession } from "../../lib/api/session-request";

test("share URL never includes an auth token", () => {
  const url = paymentRequestShareUrl("abc", "https://nimnear.example");
  assert.equal(url.includes("token"), false);
  assert.equal(shareUrlContainsSecrets(url), false);
  assert.equal(new Headers(withAuthSession(undefined).headers).has("Authorization"), false);
});

test("public payment request DTO omits private identifiers", async () => {
  const previous = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(
      JSON.stringify({
        data: {
          public_id: "11111111-1111-4111-8111-111111111111",
          recipient: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
          amount_lunas: "2500000",
          amount_nim: "25",
          note: "<b>Dinner</b>",
          status: "pending",
          expires_at: "2026-09-18T12:00:00Z",
          network: "test-albatross",
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    )) as typeof fetch;
  try {
    const request = await fetchPublicPaymentRequest("11111111-1111-4111-8111-111111111111");
    assert.equal(publicPaymentRequestHasPrivateFields(request), false);
    assert.equal("creator_user_id" in request, false);
    assert.equal("payer_user_id" in request, false);
    assert.equal("email" in request, false);
    assert.equal(paymentRequestNoteText(request.note), "<b>Dinner</b>");
  } finally {
    globalThis.fetch = previous;
  }
});
