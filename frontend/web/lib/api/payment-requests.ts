import { apiBaseUrl } from "./events";
import { withAuthSession } from "./session-request";
import { userFacingApiMessage } from "./http-error";

export type PaymentRequestStatus =
  | "pending"
  | "submitted"
  | "verifying"
  | "paid"
  | "failed"
  | "expired"
  | "cancelled";

export type PaymentRequestRecord = {
  public_id: string;
  recipient: string;
  amount_lunas: string;
  amount_nim: string;
  note: string | null;
  status: PaymentRequestStatus;
  expires_at: string;
  cancelled_at?: string | null;
  paid_at?: string | null;
  transaction_hash?: string;
  network: string;
  created_at: string;
  updated_at: string;
};

export type PublicPaymentRequest = {
  public_id: string;
  recipient: string;
  amount_lunas: string;
  amount_nim: string;
  note: string | null;
  status: PaymentRequestStatus;
  expires_at: string;
  network: string;
};

export type CreatePaymentRequestInput = {
  amount_nim: string;
  note?: string;
  address?: string;
};

type PaymentRequestResponse = { data: PaymentRequestRecord };
type PaymentRequestsResponse = { data: PaymentRequestRecord[] };
type PublicPaymentRequestResponse = { data: PublicPaymentRequest };

export class PaymentRequestsApiError extends Error {
  constructor(
    public readonly status: number,
    message = "Payment request failed",
    public readonly errorCode?: string,
  ) {
    super(message);
    this.name = "PaymentRequestsApiError";
  }
}

async function throwPaymentRequestsApiError(
  response: Response,
): Promise<never> {
  const payload = (await response.json().catch(() => null)) as {
    message?: string;
    error?: string;
    error_code?: string;
  } | null;
  throw new PaymentRequestsApiError(
    response.status,
    userFacingApiMessage(
      response.status,
      payload?.message ?? payload?.error,
      "Payment request failed",
    ),
    payload?.error_code,
  );
}

export async function createPaymentRequest(
  input: CreatePaymentRequestInput,
  token?: string,
): Promise<PaymentRequestRecord> {
  const response = await fetch(
    new URL("/api/v1/payment-requests", apiBaseUrl),
    withAuthSession(token, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        amount_nim: input.amount_nim,
        ...(input.note !== undefined ? { note: input.note } : {}),
        ...(input.address !== undefined ? { address: input.address } : {}),
      }),
    }),
  );
  if (!response.ok) await throwPaymentRequestsApiError(response);
  return ((await response.json()) as PaymentRequestResponse).data;
}

export async function listPaymentRequests(
  token?: string,
  limit?: number,
): Promise<PaymentRequestRecord[]> {
  const url = new URL("/api/v1/payment-requests", apiBaseUrl);
  if (limit !== undefined) url.searchParams.set("limit", String(limit));
  const response = await fetch(
    url,
    withAuthSession(token, {
      cache: "no-store",
    }),
  );
  if (!response.ok) await throwPaymentRequestsApiError(response);
  return ((await response.json()) as PaymentRequestsResponse).data;
}

export async function fetchPaymentRequest(
  publicId: string,
  token?: string,
): Promise<PaymentRequestRecord> {
  const response = await fetch(
    new URL(
      "/api/v1/payment-requests/" + encodeURIComponent(publicId),
      apiBaseUrl,
    ),
    withAuthSession(token, {
      cache: "no-store",
    }),
  );
  if (!response.ok) await throwPaymentRequestsApiError(response);
  return ((await response.json()) as PaymentRequestResponse).data;
}

export async function fetchPublicPaymentRequest(
  publicId: string,
): Promise<PublicPaymentRequest> {
  const response = await fetch(
    new URL(
      "/api/v1/public/payment-requests/" + encodeURIComponent(publicId),
      apiBaseUrl,
    ),
    {
      cache: "no-store",
    },
  );
  if (!response.ok) await throwPaymentRequestsApiError(response);
  return ((await response.json()) as PublicPaymentRequestResponse).data;
}

export async function cancelPaymentRequest(
  publicId: string,
  token?: string,
): Promise<PaymentRequestRecord> {
  const response = await fetch(
    new URL(
      "/api/v1/payment-requests/" +
        encodeURIComponent(publicId) +
        "/cancel",
      apiBaseUrl,
    ),
    withAuthSession(token, {
      method: "POST",
    }),
  );
  if (!response.ok) await throwPaymentRequestsApiError(response);
  return ((await response.json()) as PaymentRequestResponse).data;
}

export async function submitPaymentRequestTransaction(
  publicId: string,
  transactionHash: string,
  token?: string,
): Promise<PaymentRequestRecord> {
  const response = await fetch(
    new URL(
      "/api/v1/payment-requests/" +
        encodeURIComponent(publicId) +
        "/transaction",
      apiBaseUrl,
    ),
    withAuthSession(token, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ transaction_hash: transactionHash }),
    }),
  );
  if (!response.ok) await throwPaymentRequestsApiError(response);
  return ((await response.json()) as PaymentRequestResponse).data;
}
