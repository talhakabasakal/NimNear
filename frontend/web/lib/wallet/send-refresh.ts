import { historicTransactionStatus } from "./activity";
import { normalizeTransactionHash } from "../nimiq/transactions";
import type { WalletTransaction } from "../api/wallet";

export const WALLET_SEND_REFRESH_DELAYS_MS = [0, 2000, 4000, 8000] as const;

export type SubmittedTransfer = {
  hash: string;
  recipient: string;
  amountNim: string;
  from: string;
};

export type SubmittedTransferStatus = "submitted" | "confirmed" | "failed";

export function nextWalletRefreshDelay(attempt: number): number | null {
  return WALLET_SEND_REFRESH_DELAYS_MS[attempt] ?? null;
}

export function findHistoryTransaction(
  transactions: WalletTransaction[],
  hash: string,
): WalletTransaction | undefined {
  const normalized = normalizeTransactionHash(hash);
  if (!normalized) return undefined;
  return transactions.find((transaction) => normalizeTransactionHash(transaction.hash) === normalized);
}

export function submittedTransferStatus(
  transfer: Pick<SubmittedTransfer, "hash">,
  transactions: WalletTransaction[],
): SubmittedTransferStatus {
  const match = findHistoryTransaction(transactions, transfer.hash);
  if (!match) return "submitted";
  const historic = historicTransactionStatus(match.status);
  if (historic === "confirmed") return "confirmed";
  if (historic === "failed") return "failed";
  return "submitted";
}

export function shouldKeepLocalSubmittedState(
  transfer: Pick<SubmittedTransfer, "hash">,
  transactions: WalletTransaction[],
) {
  return submittedTransferStatus(transfer, transactions) === "submitted" &&
    !findHistoryTransaction(transactions, transfer.hash);
}
