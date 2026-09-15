import { apiBaseUrl } from "./events";

export type PurchaseStatus = "pending" | "submitted" | "verifying" | "confirmed" | "failed" | "expired" | "cancelled";

export type PurchaseRecord = {
  id: string;
  event_id: string;
  amount_lunas: string;
  amount_nim: string;
  status: PurchaseStatus;
  transaction_hash?: string;
  capacity_hold_expires_at?: string;
  confirmed_at?: string;
  created_at: string;
  updated_at: string;
};

export type PaymentInstructions = {
  purchase_id: string;
  recipient: string;
  amount_lunas: string;
  network: string;
  capacity_hold_expires_at?: string;
};

type PurchaseResponse = { data: PurchaseRecord };
type PaymentInstructionsResponse = { data: PaymentInstructions };

export class PurchasesApiError extends Error {
  constructor(public readonly status: number, message = "Payment request failed", public readonly errorCode?: string) {
    super(message);
    this.name = "PurchasesApiError";
  }
}

async function throwPurchasesApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as { message?: string; error?: string; error_code?: string } | null;
  throw new PurchasesApiError(response.status, payload?.message ?? payload?.error ?? "Payment request failed", payload?.error_code);
}

export async function createOrGetPurchase(eventId: string, token: string): Promise<PurchaseRecord> {
  const response = await fetch(new URL("/api/v1/events/" + encodeURIComponent(eventId) + "/purchases", apiBaseUrl), {
    method: "POST",
    headers: { Authorization: "Bearer " + token },
  });
  if (!response.ok) await throwPurchasesApiError(response);
  return ((await response.json()) as PurchaseResponse).data;
}

export async function fetchPurchase(purchaseId: string, token: string): Promise<PurchaseRecord> {
  const response = await fetch(new URL("/api/v1/purchases/" + encodeURIComponent(purchaseId), apiBaseUrl), {
    headers: { Authorization: "Bearer " + token },
    cache: "no-store",
  });
  if (!response.ok) await throwPurchasesApiError(response);
  return ((await response.json()) as PurchaseResponse).data;
}

export async function fetchCurrentPurchase(eventId: string, token: string): Promise<PurchaseRecord | null> {
  const response = await fetch(new URL("/api/v1/events/" + encodeURIComponent(eventId) + "/purchases/current", apiBaseUrl), {
    headers: { Authorization: "Bearer " + token },
    cache: "no-store",
  });
  if (response.status === 404) return null;
  if (!response.ok) await throwPurchasesApiError(response);
  return ((await response.json()) as PurchaseResponse).data;
}

export async function fetchPaymentInstructions(purchaseId: string, token: string): Promise<PaymentInstructions> {
  const response = await fetch(new URL("/api/v1/purchases/" + encodeURIComponent(purchaseId) + "/payment-instructions", apiBaseUrl), {
    headers: { Authorization: "Bearer " + token },
    cache: "no-store",
  });
  if (!response.ok) await throwPurchasesApiError(response);
  return ((await response.json()) as PaymentInstructionsResponse).data;
}

export async function submitPurchaseTransaction(purchaseId: string, transactionHash: string, token: string): Promise<PurchaseRecord> {
  const response = await fetch(new URL("/api/v1/purchases/" + encodeURIComponent(purchaseId) + "/transaction", apiBaseUrl), {
    method: "POST",
    headers: {
      Authorization: "Bearer " + token,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ transaction_hash: transactionHash }),
  });
  if (!response.ok) await throwPurchasesApiError(response);
  return ((await response.json()) as PurchaseResponse).data;
}
