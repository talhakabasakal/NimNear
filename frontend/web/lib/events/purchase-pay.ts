import type { PurchaseRecord } from "@/lib/api/purchases";
import {
  markHubPaymentSubmitted,
  readPendingHubPayment,
  recoverHubPaymentHash,
  runHubPaymentSubmitOnce,
  sendNimiqPayment,
  type HubPaymentKind,
  type SendNimiqPaymentDeps,
} from "@/lib/nimiq/payment-transport";
import { createSingleFlight, NimiqTransactionError } from "@/lib/nimiq/transactions";

export type EventPurchasePayResult =
  | { redirected: false; record: PurchaseRecord; hash: string }
  | { redirected: true };

export function parsePurchaseLunas(value: string) {
  if (!/^[0-9]+$/.test(value)) throw new NimiqTransactionError("invalid", "The payment amount is not a safe integer.");
  const lunas = BigInt(value);
  if (lunas <= BigInt(0) || lunas > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new NimiqTransactionError("invalid", "The payment amount is outside the supported safe number range.");
  }
  return lunas;
}

export function createEventPurchaseFlight() {
  return createSingleFlight<EventPurchasePayResult>();
}

async function submitPurchaseOnce(
  purchaseId: string,
  hash: string,
  submitHash: (purchaseId: string, hash: string) => Promise<PurchaseRecord>,
): Promise<EventPurchasePayResult> {
  return runHubPaymentSubmitOnce(purchaseId, hash, async () => {
    const record = await submitHash(purchaseId, hash);
    markHubPaymentSubmitted(hash);
    return { redirected: false as const, record, hash };
  });
}

export async function executeEventPurchasePay(input: {
  purchaseId: string;
  instructions: { recipient: string; amount_lunas: string; network: string };
  submitHash: (purchaseId: string, hash: string) => Promise<PurchaseRecord>;
  flight: ReturnType<typeof createEventPurchaseFlight>;
  existingHash?: string | null;
  payment?: SendNimiqPaymentDeps;
}): Promise<EventPurchasePayResult> {
  const amountLunas = parsePurchaseLunas(input.instructions.amount_lunas);
  return input.flight.run(async () => {
    const pending = readPendingHubPayment();
    if (pending?.kind === "event-purchase" && pending.id === input.purchaseId && pending.txHash) {
      return submitPurchaseOnce(input.purchaseId, pending.txHash, input.submitHash);
    }

    let hash = input.existingHash?.trim() || "";
    if (!hash) {
      const sent = await sendNimiqPayment(
        {
          recipient: input.instructions.recipient,
          amountLunas,
          network: input.instructions.network,
          resume: { kind: "event-purchase", id: input.purchaseId },
        },
        input.payment,
      );
      if (!sent) return { redirected: true as const };
      hash = sent;
    }
    return submitPurchaseOnce(input.purchaseId, hash, input.submitHash);
  });
}

export async function resumeHubPaidResource<T>(input: {
  kind: HubPaymentKind;
  id: string;
  submitHash: (id: string, hash: string) => Promise<T>;
  alreadySubmitted?: boolean;
  takeRedirect?: Parameters<typeof recoverHubPaymentHash>[1];
}): Promise<{ record: T; hash: string } | null> {
  const pending = readPendingHubPayment();
  if (!pending || pending.kind !== input.kind || pending.id !== input.id) return null;
  if (pending.submitted || input.alreadySubmitted) return null;
  const hash = await recoverHubPaymentHash({ kind: input.kind, id: input.id }, input.takeRedirect);
  if (!hash) return null;
  return runHubPaymentSubmitOnce(input.id, hash, async () => {
    const record = await input.submitHash(input.id, hash);
    markHubPaymentSubmitted(hash);
    return { record, hash };
  });
}
