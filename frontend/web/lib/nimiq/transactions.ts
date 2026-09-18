import { init, type ErrorResponse, type NimiqProvider } from "@nimiq/mini-app-sdk";

export const TRANSACTION_HASH_PATTERN = /^[0-9a-fA-F]{64}$/;

export type NimiqTransactionFailureKind =
  | "cancelled"
  | "unavailable"
  | "insufficient_funds"
  | "invalid"
  | "network"
  | "unexpected";

export class NimiqTransactionError extends Error {
  constructor(
    public readonly kind: NimiqTransactionFailureKind,
    message: string,
  ) {
    super(message);
    this.name = "NimiqTransactionError";
  }
}

export function isNimiqErrorResponse(value: unknown): value is ErrorResponse {
  return Boolean(value && typeof value === "object" && "error" in value);
}

export function normalizeTransactionHash(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const normalized = value.trim().toLowerCase();
  if (!/^[0-9a-f]{64}$/.test(normalized)) return null;
  return normalized;
}

function errorText(error: unknown): string {
  if (isNimiqErrorResponse(error)) {
    return `${error.error.type} ${error.error.message}`;
  }
  if (error instanceof Error) return `${error.name} ${error.message}`;
  return String(error);
}

export function classifyNimiqTransactionFailure(error: unknown): NimiqTransactionFailureKind {
  if (error instanceof NimiqTransactionError) return error.kind;
  const text = errorText(error).toLowerCase();
  const type = isNimiqErrorResponse(error) ? error.error.type.trim().toLowerCase() : "";

  if (type === "permission_denied" || text.includes("permission_denied")) return "cancelled";
  if (
    text.includes("cancel") ||
    text.includes("reject") ||
    text.includes("denied") ||
    text.includes("permission")
  ) {
    return "cancelled";
  }
  if (text.includes("insufficient") || text.includes("not enough")) return "insufficient_funds";
  if (type === "invalid_transaction" || text.includes("invalid")) return "invalid";
  if (type === "network_error" || text.includes("network")) return "network";
  if (
    text.includes("not injected") ||
    text.includes("not found") ||
    text.includes("unavailable") ||
    text.includes("timeout")
  ) {
    return "unavailable";
  }
  return "unexpected";
}

export function userMessageForTransactionFailure(kind: NimiqTransactionFailureKind): string {
  if (kind === "cancelled") return "The transaction was cancelled in Nimiq Pay.";
  if (kind === "unavailable") {
    return "Nimiq Pay is not available. Open the app in Nimiq Pay to continue.";
  }
  if (kind === "insufficient_funds") {
    return "The wallet reported insufficient funds for this transaction.";
  }
  if (kind === "invalid") return "The wallet rejected this transaction request.";
  if (kind === "network") {
    return "The transaction could not be submitted to the network. Please try again.";
  }
  return "The wallet transaction could not be completed.";
}

export async function initializeMiniAppProvider(
  initialize: typeof init = init,
): Promise<NimiqProvider> {
  try {
    return await initialize({ timeout: 10000 });
  } catch {
    throw new NimiqTransactionError(
      "unavailable",
      userMessageForTransactionFailure("unavailable"),
    );
  }
}

export function lunaAmountForSdk(lunas: bigint): number {
  if (lunas <= BigInt(0) || lunas > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new NimiqTransactionError("invalid", "The amount is outside the supported range.");
  }
  return Number(lunas);
}

export type SendBasicTransactionFn = (tx: {
  recipient: string;
  value: number;
}) => Promise<string | ErrorResponse>;

export async function sendBasicNimTransaction(
  input: { recipient: string; valueLunas: bigint },
  send: SendBasicTransactionFn,
): Promise<string> {
  let result: string | ErrorResponse;
  try {
    result = await send({
      recipient: input.recipient,
      value: lunaAmountForSdk(input.valueLunas),
    });
  } catch (error) {
    const kind = classifyNimiqTransactionFailure(error);
    throw new NimiqTransactionError(kind, userMessageForTransactionFailure(kind));
  }
  if (isNimiqErrorResponse(result)) {
    const kind = classifyNimiqTransactionFailure(result);
    throw new NimiqTransactionError(kind, userMessageForTransactionFailure(kind));
  }
  const hash = normalizeTransactionHash(result);
  if (!hash) {
    throw new NimiqTransactionError(
      "unexpected",
      "The wallet transaction ID is not in the expected hash format.",
    );
  }
  return hash;
}

export function createSingleFlight<T>() {
  let pending: Promise<T> | null = null;
  return {
    get busy() {
      return pending != null;
    },
    run(fn: () => Promise<T>) {
      if (pending) return pending;
      pending = fn().finally(() => {
        pending = null;
      });
      return pending;
    },
  };
}
