import {
  PaymentRequestsApiError,
  type PaymentRequestRecord,
  type PaymentRequestStatus,
  type PublicPaymentRequest,
} from "../api/payment-requests";
import { parseBalanceLunas } from "../nimiq/amount";
import {
  createSingleFlight,
  NimiqTransactionError,
} from "../nimiq/transactions";
import { isInFlightStatus, isPayableStatus, isTerminalStatus } from "./status";

const PAY_RESUME_KEY = "nimnear.pay.resume";

const CONFLICT_CODES = new Set([
  "payment_request_already_paid",
  "payment_request_not_submittable",
  "payment_request_cancelled",
  "payment_request_expired",
  "payment_request_state_terminal",
  "payment_request_state_changed",
]);

export type LockedPayTransfer = {
  recipient: string;
  amountNim: string;
  amountLunas: bigint;
  network: string;
};

export function paymentRequestSdkInput(
  request: Pick<PublicPaymentRequest, "recipient" | "amount_lunas">,
) {
  const amountLunas = parseBalanceLunas(request.amount_lunas);
  if (amountLunas == null || amountLunas <= BigInt(0)) return null;
  return {
    recipient: request.recipient,
    valueLunas: amountLunas,
  };
}

export function lockedPayTransfer(
  request: Pick<PublicPaymentRequest, "recipient" | "amount_lunas" | "amount_nim" | "network">,
): LockedPayTransfer | null {
  const sdk = paymentRequestSdkInput(request);
  if (!sdk) return null;
  return {
    recipient: sdk.recipient,
    amountNim: request.amount_nim,
    amountLunas: sdk.valueLunas,
    network: request.network,
  };
}

export function canInvokePaymentSend(status: string, locallyExpired: boolean) {
  return isPayableStatus(status) && !locallyExpired && !isTerminalStatus(status) && !isInFlightStatus(status);
}

export function isPaymentRequestConflict(error: unknown) {
  if (!(error instanceof PaymentRequestsApiError)) return false;
  if (error.status === 409) return true;
  return Boolean(error.errorCode && CONFLICT_CODES.has(error.errorCode));
}

export function publicPaymentRequestHasPrivateFields(value: object) {
  const privateKeys = [
    "creator_user_id",
    "payer_user_id",
    "id",
    "email",
    "first_name",
    "last_name",
  ];
  return privateKeys.some((key) => key in value);
}

export function paymentRequestNoteText(note: string | null | undefined) {
  return note ?? "";
}

export function markPaymentRequestResume(
  publicId: string,
  storage: Pick<Storage, "setItem"> | undefined = typeof window !== "undefined"
    ? window.sessionStorage
    : undefined,
) {
  storage?.setItem(PAY_RESUME_KEY, publicId);
}

export function consumePaymentRequestResume(
  expectedPublicId: string,
  storage: Pick<Storage, "getItem" | "removeItem"> | undefined = typeof window !== "undefined"
    ? window.sessionStorage
    : undefined,
) {
  if (!storage) return false;
  const stored = storage.getItem(PAY_RESUME_KEY);
  storage.removeItem(PAY_RESUME_KEY);
  return stored === expectedPublicId;
}

export function createPaymentRequestFlight() {
  return createSingleFlight<{
    hash: string;
    record: PaymentRequestRecord;
    localStatus: PaymentRequestStatus;
  }>();
}

export async function executePaymentRequestPay(input: {
  request: Pick<PublicPaymentRequest, "public_id" | "recipient" | "amount_lunas" | "status">;
  locallyExpired: boolean;
  existingHash?: string | null;
  sendTransaction: (recipient: string, valueLunas: bigint) => Promise<string>;
  submitHash: (publicId: string, hash: string) => Promise<PaymentRequestRecord>;
  flight: ReturnType<typeof createPaymentRequestFlight>;
}): Promise<
  | { ok: true; hash: string; record: PaymentRequestRecord; localStatus: PaymentRequestStatus }
  | { ok: false; kind: "cancelled" | "conflict" | "blocked" | "error"; hash?: string; error?: unknown }
> {
  if (!canInvokePaymentSend(input.request.status, input.locallyExpired) && !input.existingHash) {
    return { ok: false, kind: "blocked" };
  }
  const sdk = paymentRequestSdkInput(input.request);
  if (!sdk) return { ok: false, kind: "blocked" };

  let hash = input.existingHash?.trim() || "";
  try {
    const result = await input.flight.run(async () => {
      if (!hash) {
        hash = await input.sendTransaction(sdk.recipient, sdk.valueLunas);
      }
      const record = await input.submitHash(input.request.public_id, hash);
      return { hash, record, localStatus: record.status };
    });
    return { ok: true, ...result };
  } catch (error) {
    if (error instanceof NimiqTransactionError && error.kind === "cancelled") {
      return { ok: false, kind: "cancelled", error };
    }
    if (isPaymentRequestConflict(error)) {
      return { ok: false, kind: "conflict", hash: hash || undefined, error };
    }
    return { ok: false, kind: "error", hash: hash || undefined, error };
  }
}
