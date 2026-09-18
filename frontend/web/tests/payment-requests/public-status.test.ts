import assert from "node:assert/strict";
import test from "node:test";

import type { PublicPaymentRequest } from "../../lib/api/payment-requests";
import {
  expirationView,
  isLocallyExpired,
  isPayableStatus,
  paymentRequestStatusCopy,
} from "../../lib/payment-requests/status";
import { paymentRequestNoteText, publicPaymentRequestHasPrivateFields } from "../../lib/payment-requests/pay";

function publicRequest(status: PublicPaymentRequest["status"]): PublicPaymentRequest {
  return {
    public_id: "11111111-1111-4111-8111-111111111111",
    recipient: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
    amount_lunas: "2500000",
    amount_nim: "25",
    note: "Dinner",
    status,
    expires_at: "2026-09-18T12:00:00Z",
    network: "test-albatross",
  };
}

test("public statuses have explicit copy and only pending is payable", () => {
  assert.equal(isPayableStatus("pending"), true);
  assert.equal(paymentRequestStatusCopy("pending").title, "Payable");
  assert.equal(paymentRequestStatusCopy("submitted").title, "Payment submitted");
  assert.equal(paymentRequestStatusCopy("verifying").title, "Verifying on Nimiq");
  assert.equal(paymentRequestStatusCopy("paid").label, "Paid");
  assert.equal(paymentRequestStatusCopy("cancelled").title, "Request cancelled");
  assert.equal(paymentRequestStatusCopy("expired").title, "Request expired");
  assert.equal(paymentRequestStatusCopy("failed").title, "Verification failed");
  for (const status of ["submitted", "verifying", "paid", "cancelled", "expired", "failed"] as const) {
    assert.equal(isPayableStatus(status), false);
    assert.equal(isPayableStatus(publicRequest(status).status), false);
  }
});

test("unknown request copy does not invent a payable state", () => {
  const unknown = paymentRequestStatusCopy("not-a-status");
  assert.equal(unknown.title, "Unknown request status");
  assert.equal(isPayableStatus("not-a-status"), false);
});

test("expiration uses expires_at and does not locally mark backend status", () => {
  const now = Date.parse("2026-09-18T11:00:00Z");
  const later = Date.parse("2026-09-18T13:00:00Z");
  assert.equal(isLocallyExpired("2026-09-18T12:00:00Z", now), false);
  assert.equal(isLocallyExpired("2026-09-18T12:00:00Z", later), true);
  assert.equal(expirationView("2026-09-18T12:00:00Z", later).expired, true);
  assert.equal(publicRequest("pending").status, "pending");
});

test("public DTO helper treats note as plain text and rejects private fields", () => {
  const request = publicRequest("pending");
  assert.equal(paymentRequestNoteText(request.note), "Dinner");
  assert.equal(paymentRequestNoteText("<script>alert(1)</script>"), "<script>alert(1)</script>");
  assert.equal(publicPaymentRequestHasPrivateFields(request), false);
  assert.equal(
    publicPaymentRequestHasPrivateFields({
      ...request,
      creator_user_id: "secret",
    }),
    true,
  );
});
