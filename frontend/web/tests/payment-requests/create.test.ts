import assert from "node:assert/strict";
import test from "node:test";

import {
  inspectPaymentRequestNote,
  PAYMENT_REQUEST_MAX_NOTE_LENGTH,
  validateCreatePaymentRequest,
} from "../../lib/payment-requests/create";
import { createPaymentRequest, PaymentRequestsApiError } from "../../lib/api/payment-requests";

const identity = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";

test("valid payment request uses exact NIM parsing and selected identity", () => {
  const result = validateCreatePaymentRequest({
    amount: "25.00",
    note: "Dinner",
    selectedAddress: identity,
  });
  assert.equal(result.ok, true);
  if (result.ok) {
    assert.equal(result.value.amountNim, "25");
    assert.equal(result.value.note, "Dinner");
    assert.equal(result.value.address, identity);
  }
});

test("invalid amounts are rejected without float arithmetic", () => {
  assert.equal(validateCreatePaymentRequest({ amount: "", selectedAddress: identity }).ok, false);
  assert.equal(validateCreatePaymentRequest({ amount: "0", selectedAddress: identity }).ok, false);
  assert.equal(validateCreatePaymentRequest({ amount: "1.000001", selectedAddress: identity }).ok, false);
  assert.equal(validateCreatePaymentRequest({ amount: "-2", selectedAddress: identity }).ok, false);
  const valid = validateCreatePaymentRequest({ amount: "0.00001", selectedAddress: identity });
  assert.equal(valid.ok, true);
  if (valid.ok) assert.equal(valid.value.amountNim, "0.00001");
});

test("note is limited to 140 plain-text characters", () => {
  assert.equal(inspectPaymentRequestNote("Dinner"), null);
  assert.equal(inspectPaymentRequestNote("a".repeat(PAYMENT_REQUEST_MAX_NOTE_LENGTH + 1)), "too_long");
  assert.equal(inspectPaymentRequestNote("hello\nworld"), "control");
  const tooLong = validateCreatePaymentRequest({
    amount: "1",
    note: "n".repeat(141),
    selectedAddress: identity,
  });
  assert.equal(tooLong.ok, false);
  if (!tooLong.ok) assert.equal(tooLong.field, "note");
});

test("create requires the selected verified identity and does not accept an empty recipient", () => {
  const missing = validateCreatePaymentRequest({ amount: "1", selectedAddress: "" });
  assert.equal(missing.ok, false);
  if (!missing.ok) assert.equal(missing.field, "identity");
});

test("create API failure surfaces as PaymentRequestsApiError", async () => {
  const previous = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(JSON.stringify({ message: "amount_nim is required", error_code: "invalid_amount" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    })) as typeof fetch;
  try {
    await assert.rejects(
      () => createPaymentRequest({ amount_nim: "25" }),
      (error: unknown) =>
        error instanceof PaymentRequestsApiError &&
        error.status === 400 &&
        error.errorCode === "invalid_amount",
    );
  } finally {
    globalThis.fetch = previous;
  }
});
