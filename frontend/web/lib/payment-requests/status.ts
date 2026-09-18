import type { PaymentRequestStatus } from "../api/payment-requests";

export const PAYMENT_REQUEST_POLL_DELAYS_MS = [
  2000, 3000, 5000, 8000, 12000, 15000, 15000, 15000,
] as const;

export type PaymentRequestStatusCopy = {
  label: string;
  title: string;
  description: string;
  badge: "default" | "success" | "warning" | "destructive" | "outline";
};

const STATUS_COPY: Record<PaymentRequestStatus, PaymentRequestStatusCopy> = {
  pending: {
    label: "Pending",
    title: "Payable",
    description: "This request is waiting for payment.",
    badge: "default",
  },
  submitted: {
    label: "Submitted",
    title: "Payment submitted",
    description: "Payment submitted. Verification is in progress.",
    badge: "warning",
  },
  verifying: {
    label: "Verifying",
    title: "Verifying on Nimiq",
    description: "The transaction is being verified on Nimiq.",
    badge: "warning",
  },
  paid: {
    label: "Paid",
    title: "Paid",
    description: "This payment request has been paid.",
    badge: "success",
  },
  cancelled: {
    label: "Cancelled",
    title: "Request cancelled",
    description: "This payment request was cancelled.",
    badge: "outline",
  },
  expired: {
    label: "Expired",
    title: "Request expired",
    description: "This payment request has expired.",
    badge: "outline",
  },
  failed: {
    label: "Failed",
    title: "Verification failed",
    description: "Payment verification failed. This request is no longer payable.",
    badge: "destructive",
  },
};

export function isPayableStatus(status: string) {
  return status === "pending";
}

export function isInFlightStatus(status: string) {
  return status === "submitted" || status === "verifying";
}

export function isTerminalStatus(status: string) {
  return (
    status === "paid" ||
    status === "cancelled" ||
    status === "expired" ||
    status === "failed"
  );
}

export function canCancelPaymentRequest(status: string) {
  return status === "pending";
}

export function paymentRequestStatusCopy(status: string): PaymentRequestStatusCopy {
  if (status in STATUS_COPY) return STATUS_COPY[status as PaymentRequestStatus];
  return {
    label: status || "Unknown",
    title: "Unknown request status",
    description: "This payment request could not be classified.",
    badge: "outline",
  };
}

export function isLocallyExpired(expiresAt: string, now = Date.now()) {
  const expires = Date.parse(expiresAt);
  return Number.isFinite(expires) && now >= expires;
}

export function expirationView(expiresAt: string, now = Date.now()) {
  const expires = Date.parse(expiresAt);
  if (!Number.isFinite(expires)) {
    return {
      expired: false,
      label: "Expiry unavailable",
      detail: expiresAt,
    };
  }
  const absolute = new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(expires));
  if (now >= expires) {
    return {
      expired: true,
      label: "Expired",
      detail: `Expired at ${absolute}`,
    };
  }
  const remainingMs = expires - now;
  const minutes = Math.round(remainingMs / 60_000);
  let label: string;
  if (minutes < 1) label = "Expires in less than a minute";
  else if (minutes < 60) label = `Expires in ${minutes} min`;
  else if (minutes < 60 * 48) {
    const hours = Math.max(1, Math.round(minutes / 60));
    label = hours === 1 ? "Expires in 1 hour" : `Expires in ${hours} hours`;
  } else {
    label = `Expires at ${absolute}`;
  }
  return { expired: false, label, detail: absolute };
}

export function nextPaymentRequestPollDelay(attempt: number): number | null {
  return PAYMENT_REQUEST_POLL_DELAYS_MS[attempt] ?? null;
}

export function statusAfterClientHash(
  backendStatus?: PaymentRequestStatus,
): PaymentRequestStatus {
  return backendStatus ?? "submitted";
}
