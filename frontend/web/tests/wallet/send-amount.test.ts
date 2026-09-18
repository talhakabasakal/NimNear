import assert from "node:assert/strict";
import test from "node:test";

import { nimToLunas } from "../../lib/nimiq/amount";
import {
  amountErrorMessage,
  inspectSendAmount,
  validateWalletSend,
} from "../../lib/wallet/send";

const from = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const recipient = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";

test("NIM to Luna parsing is exact without floating point rounding", () => {
  assert.equal(nimToLunas("1"), BigInt("100000"));
  assert.equal(nimToLunas("1.5"), BigInt("150000"));
  assert.equal(nimToLunas("0.00001"), BigInt("1"));
  assert.equal(inspectSendAmount("0"), "zero");
  assert.equal(inspectSendAmount("-1"), "malformed");
  assert.equal(inspectSendAmount("1.000001"), "too_many_decimals");
  assert.equal(inspectSendAmount("1.2.3"), "malformed");
  assert.equal(inspectSendAmount("abc"), "malformed");
  assert.equal(inspectSendAmount("NaN"), "malformed");
  assert.equal(inspectSendAmount("Infinity"), "malformed");
  assert.equal(inspectSendAmount("90071992547.40992"), "too_large");
});

test("send amount validation rejects zero and amounts above available balance", () => {
  assert.equal(inspectSendAmount(""), "empty");
  assert.equal(amountErrorMessage("zero"), "Enter an amount greater than zero.");
  const over = validateWalletSend({
    recipient,
    amount: "3",
    fromAddress: from,
    balanceLunas: "250000",
  });
  assert.equal(over.ok, false);
  if (!over.ok) {
    assert.equal(over.field, "amount");
    assert.equal(over.message, "Amount exceeds available balance.");
  }
  const exact = validateWalletSend({
    recipient,
    amount: "2.5",
    fromAddress: from,
    balanceLunas: "250000",
  });
  assert.equal(exact.ok, true);
  if (exact.ok) assert.equal(exact.value.amountLunas, BigInt("250000"));
});
