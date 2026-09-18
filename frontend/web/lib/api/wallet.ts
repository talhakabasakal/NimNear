import { apiBaseUrl } from "./events";
import { userFacingApiMessage } from "./http-error";
import { withAuthSession } from "./session-request";

export type WalletBalance = {
  address: string;
  balance_lunas: string;
  balance_nim: string;
  account_type?: string;
  network?: string;
};

export type WalletTransaction = {
  hash: string;
  sender: string;
  recipient: string;
  value_lunas: string;
  value_nim: string;
  block_number?: number;
  timestamp?: number;
  network_id?: number;
  network?: string;
  status?: string;
};

export type WalletTransactionsQuery = {
  address?: string;
  max?: number;
  start_at?: string;
};

export type WalletTransactionsResult = {
  data: WalletTransaction[];
  next_start_at?: string;
};

type WalletBalanceResponse = { data: WalletBalance };

export class WalletApiError extends Error {
  constructor(
    public readonly status: number,
    message = "Wallet API returned " + status,
    public readonly errorCode?: string,
  ) {
    super(message);
    this.name = "WalletApiError";
  }
}

async function throwWalletApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as {
    message?: string;
    error?: string;
    error_code?: string;
  } | null;
  throw new WalletApiError(
    response.status,
    userFacingApiMessage(
      response.status,
      payload?.message ?? payload?.error,
      "Wallet could not be loaded.",
    ),
    payload?.error_code,
  );
}

export async function getWalletBalance(
  token?: string,
  address?: string,
): Promise<WalletBalance> {
  const url = new URL("/api/v1/wallet/balance", apiBaseUrl);
  if (address) url.searchParams.set("address", address);
  const response = await fetch(
    url,
    withAuthSession(token, { cache: "no-store" }),
  );
  if (!response.ok) await throwWalletApiError(response);
  return ((await response.json()) as WalletBalanceResponse).data;
}

export async function getWalletTransactions(
  query: WalletTransactionsQuery = {},
  token?: string,
): Promise<WalletTransactionsResult> {
  const url = new URL("/api/v1/wallet/transactions", apiBaseUrl);
  if (query.address) url.searchParams.set("address", query.address);
  if (query.max) url.searchParams.set("max", String(query.max));
  if (query.start_at) url.searchParams.set("start_at", query.start_at);
  const response = await fetch(
    url,
    withAuthSession(token, { cache: "no-store" }),
  );
  if (!response.ok) await throwWalletApiError(response);
  return (await response.json()) as WalletTransactionsResult;
}
