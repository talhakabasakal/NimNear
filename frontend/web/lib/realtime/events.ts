export const REALTIME_RECONNECTED = "realtime.reconnected";

export type RealtimeEvent = {
  type: string;
  topic?: string;
  data?: {
    resource_id?: string;
    status?: string;
  };
  timestamp?: string;
};

const PAYMENT_REQUEST_TYPES = new Set([
  "payment_request.submitted",
  "payment_request.verifying",
  "payment_request.paid",
  "payment_request.failed",
  "payment_request.expired",
  "payment_request.cancelled",
]);

const EVENT_PURCHASE_TYPES = new Set([
  "event_purchase.submitted",
  "event_purchase.verifying",
  "event_purchase.confirmed",
  "event_purchase.failed",
  "event_purchase.expired",
  "event_purchase.cancelled",
]);

export function isPaymentRequestEvent(type: string) {
  return PAYMENT_REQUEST_TYPES.has(type);
}

export function isEventPurchaseEvent(type: string) {
  return EVENT_PURCHASE_TYPES.has(type);
}

export function isWalletActivityEvent(type: string) {
  return type === "wallet.activity_changed";
}

export function isKnownRealtimeEvent(type: string) {
  return isPaymentRequestEvent(type) || isEventPurchaseEvent(type) || isWalletActivityEvent(type);
}

export function eventResourceId(event: RealtimeEvent): string {
  return event.data?.resource_id?.trim() ?? "";
}

export function eventTouchesResource(event: RealtimeEvent, resourceId: string) {
  return eventResourceId(event) === resourceId;
}

export function createCoalescer(delayMs = 300) {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let pending: (() => void) | null = null;
  const run = (fn: () => void) => {
    pending = fn;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      const next = pending;
      pending = null;
      next?.();
    }, delayMs);
  };
  const flush = (fn: () => void) => {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
    pending = null;
    fn();
  };
  const clear = () => {
    if (timer) clearTimeout(timer);
    timer = null;
    pending = null;
  };
  return { run, flush, clear };
}
