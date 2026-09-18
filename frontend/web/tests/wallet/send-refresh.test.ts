import assert from "node:assert/strict";
import test from "node:test";

import type { WalletTransaction } from "../../lib/api/wallet";
import {
  findHistoryTransaction,
  nextWalletRefreshDelay,
  shouldKeepLocalSubmittedState,
  submittedTransferStatus,
  WALLET_SEND_REFRESH_DELAYS_MS,
} from "../../lib/wallet/send-refresh";

const hash = "ab".repeat(32);
const wallet = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const recipient = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";

function tx(overrides: Partial<WalletTransaction> = {}): WalletTransaction {
  return {
    hash,
    sender: wallet,
    recipient,
    value_lunas: "100000",
    value_nim: "1",
    ...overrides,
  };
}

test("a submitted hash is not confirmed before it appears in historic history", () => {
  const transfer = { hash, recipient, amountNim: "1", from: wallet };
  assert.equal(submittedTransferStatus(transfer, []), "submitted");
  assert.equal(shouldKeepLocalSubmittedState(transfer, []), true);
  assert.equal(findHistoryTransaction([], hash), undefined);
});

test("bounded refresh delays are finite and stop after the last attempt", () => {
  assert.deepEqual([...WALLET_SEND_REFRESH_DELAYS_MS], [0, 2000, 4000, 8000]);
  assert.equal(nextWalletRefreshDelay(0), 0);
  assert.equal(nextWalletRefreshDelay(3), 8000);
  assert.equal(nextWalletRefreshDelay(4), null);
});

test("eventual history appearance uses the historic status and allows manual refresh", () => {
  const transfer = { hash, recipient, amountNim: "1", from: wallet };
  const missing: WalletTransaction[] = [];
  assert.equal(submittedTransferStatus(transfer, missing), "submitted");

  const later = [tx({ status: "successful" })];
  assert.equal(submittedTransferStatus(transfer, later), "confirmed");
  assert.equal(shouldKeepLocalSubmittedState(transfer, later), false);
  assert.equal(findHistoryTransaction(later, hash.toUpperCase())?.hash, hash);
});
