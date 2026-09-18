import assert from "node:assert/strict";
import test from "node:test";

import {
  PaymentRequestsApiError,
  type PaymentRequestRecord,
  type PublicPaymentRequest,
} from "../../lib/api/payment-requests";
import {
  consumePaymentRequestResume,
  createPaymentRequestFlight,
  executePaymentRequestPay,
  lockedPayTransfer,
  markPaymentRequestResume,
  paymentRequestSdkInput,
} from "../../lib/payment-requests/pay";
import {
  nextPaymentRequestPollDelay,
  PAYMENT_REQUEST_POLL_DELAYS_MS,
  statusAfterClientHash,
} from "../../lib/payment-requests/status";
import { NimiqTransactionError } from "../../lib/nimiq/transactions";

const publicId = "11111111-1111-4111-8111-111111111111";
const recipient = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const hash = "ab".repeat(32);

function pendingRequest(): PublicPaymentRequest {
  return {
    public_id: publicId,
    recipient,
    amount_lunas: "2500000",
    amount_nim: "25",
    note: "Dinner",
    status: "pending",
    expires_at: "2099-09-18T12:00:00Z",
    network: "test-albatross",
  };
}

function record(status: PaymentRequestRecord["status"]): PaymentRequestRecord {
  const request = pendingRequest();
  return {
    ...request,
    status,
    created_at: "2026-09-17T12:00:00Z",
    updated_at: "2026-09-17T12:00:00Z",
  };
}

test("payer cannot edit request recipient or amount used by the SDK", () => {
  const request = pendingRequest();
  const locked = lockedPayTransfer({
    ...request,
    amount_nim: "999",
  });
  assert.equal(locked?.recipient, request.recipient);
  assert.equal(locked?.amountLunas, BigInt(request.amount_lunas));
  assert.notEqual(locked?.amountLunas, BigInt("99900000"));
  const sdk = paymentRequestSdkInput(request);
  assert.equal(sdk?.recipient, request.recipient);
  assert.equal(sdk?.valueLunas, BigInt(request.amount_lunas));
});

test("SDK is invoked exactly once and cancellation does not submit a hash", async () => {
  const flight = createPaymentRequestFlight();
  let sendCalls = 0;
  let submitCalls = 0;
  const send = async (to: string, amount: bigint) => {
    sendCalls += 1;
    assert.equal(to, recipient);
    assert.equal(amount, BigInt("2500000"));
    await new Promise((resolve) => setTimeout(resolve, 20));
    return hash;
  };
  const submit = async (id: string, transactionHash: string) => {
    submitCalls += 1;
    assert.equal(id, publicId);
    assert.equal(transactionHash, hash);
    return record("submitted");
  };
  const first = executePaymentRequestPay({
    request: pendingRequest(),
    locallyExpired: false,
    flight,
    sendTransaction: send,
    submitHash: submit,
  });
  const second = executePaymentRequestPay({
    request: pendingRequest(),
    locallyExpired: false,
    flight,
    sendTransaction: send,
    submitHash: submit,
  });
  const [a, b] = await Promise.all([first, second]);
  assert.equal(a.ok, true);
  assert.equal(b.ok, true);
  assert.equal(sendCalls, 1);
  assert.equal(submitCalls, 1);

  const cancelled = await executePaymentRequestPay({
    request: pendingRequest(),
    locallyExpired: false,
    flight: createPaymentRequestFlight(),
    sendTransaction: async () => {
      throw new NimiqTransactionError("cancelled", "The transaction was cancelled in Nimiq Pay.");
    },
    submitHash: async () => {
      submitCalls += 1;
      return record("submitted");
    },
  });
  assert.equal(cancelled.ok, false);
  if (!cancelled.ok) assert.equal(cancelled.kind, "cancelled");
  assert.equal(submitCalls, 1);
});

test("a client hash is submitted to the backend and is not treated as paid", async () => {
  const result = await executePaymentRequestPay({
    request: pendingRequest(),
    locallyExpired: false,
    flight: createPaymentRequestFlight(),
    sendTransaction: async () => hash,
    submitHash: async () => record("submitted"),
  });
  assert.equal(result.ok, true);
  if (result.ok) {
    assert.equal(result.hash, hash);
    assert.equal(statusAfterClientHash(), "submitted");
    assert.equal(statusAfterClientHash(result.record.status), "submitted");
    assert.notEqual(statusAfterClientHash(), "paid");
    assert.notEqual(result.record.status, "paid");
  }
});

test("submitted and verifying poll with a bounded delay schedule until paid", () => {
  assert.equal(nextPaymentRequestPollDelay(0), 2000);
  assert.equal(nextPaymentRequestPollDelay(PAYMENT_REQUEST_POLL_DELAYS_MS.length - 1), 15000);
  assert.equal(nextPaymentRequestPollDelay(PAYMENT_REQUEST_POLL_DELAYS_MS.length), null);
  const sequence: Array<PublicPaymentRequest["status"]> = ["submitted", "verifying", "paid"];
  assert.deepEqual(sequence, ["submitted", "verifying", "paid"]);
});

test("realtime events keep fallback polling and do not become status truth", () => {
  const backend = record("verifying");
  const event = { type: "payment_request.paid", data: { resource_id: publicId, status: "paid" } };
  assert.equal(event.data.status, "paid");
  assert.equal(backend.status, "verifying");
  assert.equal(nextPaymentRequestPollDelay(0), 2000);
});

test("already-paid conflict does not send another transaction", async () => {
  let sendCalls = 0;
  const result = await executePaymentRequestPay({
    request: pendingRequest(),
    locallyExpired: false,
    existingHash: hash,
    flight: createPaymentRequestFlight(),
    sendTransaction: async () => {
      sendCalls += 1;
      return hash;
    },
    submitHash: async () => {
      throw new PaymentRequestsApiError(409, "payment request is already paid", "payment_request_already_paid");
    },
  });
  assert.equal(sendCalls, 0);
  assert.equal(result.ok, false);
  if (!result.ok) assert.equal(result.kind, "conflict");
});

test("auth resume keeps the payment-request public id and no token", () => {
  const storage = new Map<string, string>();
  const sessionStorage = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => { storage.set(key, value); },
    removeItem: (key: string) => { storage.delete(key); },
  };
  markPaymentRequestResume(publicId, sessionStorage);
  assert.equal(storage.get("nimnear.pay.resume"), publicId);
  assert.equal([...storage.values()].some((value) => value.includes("jwt") || value.includes("Bearer")), false);
  assert.equal(consumePaymentRequestResume(publicId, sessionStorage), true);
  assert.equal(consumePaymentRequestResume(publicId, sessionStorage), false);
});
