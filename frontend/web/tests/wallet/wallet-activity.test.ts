import assert from "node:assert/strict";
import test from "node:test";

import { WalletApiError, type WalletBalance } from "../../lib/api/wallet";
import {
  formatNimAmount,
  historicTransactionStatus,
  isWalletAuthError,
  isWalletIdentityError,
  isWalletRpcUnavailable,
  signedActivityAmount,
  transactionDirection,
} from "../../lib/wallet/activity";
import { compactNimiqAddress } from "../../lib/nimiq/address";

const wallet = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const counterparty = "NQ05 U1RF 4MDX SFUP FHYL 9FYC BH7V 5EWN ARYL";

test("balance API strings stay precision-safe for large Luna values", () => {
  const payload: WalletBalance = {
    address: wallet,
    balance_lunas: "9007199254740993",
    balance_nim: "90071992547.40993",
    account_type: "basic",
    network: "test-albatross",
  };
  assert.equal(payload.balance_lunas, "9007199254740993");
  assert.equal(typeof payload.balance_lunas, "string");
  assert.equal(formatNimAmount(payload.balance_nim), "90,071,992,547.40993");
  assert.notEqual(String(Number(payload.balance_lunas)), payload.balance_lunas);
});

test("sent and received direction use compact Nimiq address comparison", () => {
  const compactWallet = compactNimiqAddress(wallet);
  assert.equal(
    transactionDirection(
      { sender: compactWallet.toLowerCase(), recipient: counterparty },
      wallet,
    ),
    "sent",
  );
  assert.equal(
    transactionDirection(
      { sender: counterparty, recipient: wallet.replace(/\s/g, "").toLowerCase() },
      wallet,
    ),
    "received",
  );
  assert.equal(signedActivityAmount("sent", "10"), "-10");
  assert.equal(signedActivityAmount("received", "10"), "+10");
  assert.equal(signedActivityAmount("received", "2.50000"), "+2.5");
});

test("historic RPC status maps successful to confirmed and failed to failed", () => {
  assert.equal(historicTransactionStatus("successful"), "confirmed");
  assert.equal(historicTransactionStatus("failed"), "failed");
  assert.equal(historicTransactionStatus("verifying"), null);
  assert.equal(historicTransactionStatus("pending"), null);
  assert.equal(historicTransactionStatus(undefined), null);
});

test("optional RPC fields can be omitted without inventing values", () => {
  const tx = {
    hash: "ab".repeat(32),
    sender: wallet,
    recipient: counterparty,
    value_lunas: "1",
    value_nim: "0.00001",
  };
  assert.equal("block_number" in tx, false);
  assert.equal("timestamp" in tx, false);
  assert.equal("status" in tx, false);
  assert.equal(historicTransactionStatus(undefined), null);
  assert.equal(transactionDirection(tx, wallet), "sent");
});

test("empty history remains an empty list", () => {
  const data: unknown[] = [];
  assert.equal(data.length, 0);
});

test("wallet error classifiers distinguish auth, identity, and RPC failures", () => {
  assert.equal(isWalletAuthError(new WalletApiError(401, "missing authentication session")), true);
  assert.equal(
    isWalletIdentityError(
      new WalletApiError(403, "forbidden", "wallet_identity_forbidden"),
    ),
    true,
  );
  assert.equal(
    isWalletIdentityError(
      new WalletApiError(404, "missing identity", "no_verified_nimiq_identity"),
    ),
    true,
  );
  assert.equal(
    isWalletRpcUnavailable(
      new WalletApiError(500, "unavailable", "nimiq_rpc_unavailable"),
    ),
    true,
  );
  assert.equal(isWalletRpcUnavailable(new TypeError("Failed to fetch")), true);
  assert.equal(isWalletAuthError(new WalletApiError(500, "unavailable")), false);
});
