import assert from "node:assert/strict";
import test from "node:test";

import {
  classifyNimiqTransactionFailure,
  createSingleFlight,
  NimiqTransactionError,
  normalizeTransactionHash,
  sendBasicNimTransaction,
} from "../../lib/nimiq/transactions";

const recipient = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";
const hash = "ab".repeat(32);

test("successful sendBasicTransaction hash is normalized and is not treated as confirmed", async () => {
  let calls = 0;
  const result = await sendBasicNimTransaction(
    { recipient, valueLunas: BigInt("100000") },
    async () => {
      calls += 1;
      return hash.toUpperCase();
    },
  );
  assert.equal(result, hash);
  assert.equal(normalizeTransactionHash(result), hash);
  assert.equal(calls, 1);
  assert.notEqual(result, "confirmed");
});

test("confirm invokes the SDK exactly once and double submit is prevented", async () => {
  let calls = 0;
  const flight = createSingleFlight<string>();
  const send = async () => {
    calls += 1;
    await new Promise((resolve) => setTimeout(resolve, 20));
    return hash;
  };
  const first = flight.run(() => sendBasicNimTransaction({ recipient, valueLunas: BigInt("1") }, send));
  const second = flight.run(() => sendBasicNimTransaction({ recipient, valueLunas: BigInt("1") }, send));
  const [a, b] = await Promise.all([first, second]);
  assert.equal(a, hash);
  assert.equal(b, hash);
  assert.equal(calls, 1);
});

test("cancelled wallet approval is classified separately from SDK failure", async () => {
  await assert.rejects(
    () => sendBasicNimTransaction(
      { recipient, valueLunas: BigInt("1") },
      async () => {
        throw new Error("PERMISSION_DENIED: user cancelled");
      },
    ),
    (error: unknown) => error instanceof NimiqTransactionError && error.kind === "cancelled",
  );
  await assert.rejects(
    () => sendBasicNimTransaction(
      { recipient, valueLunas: BigInt("1") },
      async () => ({ error: { type: "NETWORK_ERROR", message: "broadcast failed" } }),
    ),
    (error: unknown) => error instanceof NimiqTransactionError && error.kind === "network",
  );
  assert.equal(classifyNimiqTransactionFailure({ error: { type: "INVALID_TRANSACTION", message: "bad tx" } }), "invalid");
});

test("retry is allowed after a cancelled or failed SDK call", async () => {
  const flight = createSingleFlight<string>();
  await assert.rejects(
    () => flight.run(() => sendBasicNimTransaction(
      { recipient, valueLunas: BigInt("1") },
      async () => {
        throw new Error("user rejected");
      },
    )),
    (error: unknown) => error instanceof NimiqTransactionError && error.kind === "cancelled",
  );
  const result = await flight.run(() => sendBasicNimTransaction(
    { recipient, valueLunas: BigInt("1") },
    async () => hash,
  ));
  assert.equal(result, hash);
});
