import { compactNimiqAddress } from "../nimiq/address";
import { WalletApiError, type WalletTransaction } from "../api/wallet";

export type TransactionDirection = "received" | "sent";
export type HistoricTransactionStatus = "confirmed" | "failed";

/**
 * Historic RPC status comes from executionResult:
 * successful → included and executed, failed → included but reverted.
 * getTransactionsByAddress does not return mempool transactions, so
 * "verifying"/"pending" belong to purchase flows, not wallet history.
 */
export function historicTransactionStatus(
  status?: string,
): HistoricTransactionStatus | null {
  const normalized = status?.trim().toLowerCase();
  if (normalized === "successful") return "confirmed";
  if (normalized === "failed") return "failed";
  return null;
}

export function transactionDirection(
  transaction: Pick<WalletTransaction, "sender" | "recipient">,
  walletAddress: string,
): TransactionDirection {
  const wallet = compactNimiqAddress(walletAddress);
  if (wallet && compactNimiqAddress(transaction.sender) === wallet) return "sent";
  if (wallet && compactNimiqAddress(transaction.recipient) === wallet) {
    return "received";
  }
  return "received";
}

export function formatNimAmount(value: string) {
  const normalized = value.trim();
  const match = normalized.match(/^(-?)(\d+)(?:\.(\d+))?$/);
  if (!match) return normalized;
  const [, sign, whole, fraction = ""] = match;
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const trimmedFraction = fraction.replace(/0+$/, "");
  return `${sign}${grouped}${trimmedFraction ? `.${trimmedFraction}` : ""}`;
}

export function signedActivityAmount(
  direction: TransactionDirection,
  valueNim: string,
) {
  const amount = formatNimAmount(valueNim);
  return direction === "received" ? `+${amount}` : `-${amount}`;
}

export function isWalletAuthError(error: unknown) {
  return error instanceof WalletApiError && error.status === 401;
}

export function isWalletIdentityError(error: unknown) {
  if (!(error instanceof WalletApiError)) return false;
  if (error.errorCode === "wallet_identity_forbidden") return true;
  if (error.errorCode === "no_verified_nimiq_identity") return true;
  return error.status === 403;
}

export function isWalletRpcUnavailable(error: unknown) {
  if (error instanceof TypeError) return true;
  if (!(error instanceof WalletApiError)) return false;
  if (error.status >= 500) return true;
  return (
    error.errorCode === "nimiq_rpc_unavailable" ||
    error.errorCode === "nimiq_rpc_timeout" ||
    error.errorCode === "nimiq_rpc_authentication_failed" ||
    error.errorCode === "nimiq_rpc_error"
  );
}
