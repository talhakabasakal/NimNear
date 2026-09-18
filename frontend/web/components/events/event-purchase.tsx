"use client";

import { useEffect, useRef, useState } from "react";

import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { Button } from "@/components/ui/button";
import {
  AuthApiError,
  clearAuthSession,
  fetchCurrentUser,
  isLostSessionStatus,
  readAuthSession,
  type AuthSession,
  writeAuthSession,
} from "@/lib/api/auth";
import {
  createOrGetPurchase,
  fetchCurrentPurchase,
  fetchPaymentInstructions,
  fetchPurchase,
  PurchasesApiError,
  submitPurchaseTransaction,
  type PurchaseRecord,
} from "@/lib/api/purchases";
import { subscribeRealtime } from "@/lib/realtime/client";
import { createCoalescer, eventTouchesResource, isEventPurchaseEvent } from "@/lib/realtime/events";
import {
  initializeMiniAppProvider,
  NimiqTransactionError,
  sendBasicNimTransaction,
} from "@/lib/nimiq/transactions";

type EventPurchaseProps = {
  eventId: string;
  isPast: boolean;
  isSoldOut: boolean;
};

type ViewState =
  | "checking"
  | "load-error"
  | "anonymous"
  | "ready"
  | "preparing"
  | "wallet"
  | "rejected"
  | "submitted"
  | "verifying"
  | "confirmed"
  | "failed"
  | "expired";

function stateForPurchase(purchase: PurchaseRecord | null): ViewState {
  if (!purchase) return "ready";
  if (purchase.status === "submitted") return "submitted";
  if (purchase.status === "verifying") return "verifying";
  if (purchase.status === "confirmed") return "confirmed";
  if (purchase.status === "failed") return "failed";
  if (purchase.status === "expired") return "expired";
  return "ready";
}

function isWalletRejection(error: unknown) {
  if (error instanceof NimiqTransactionError) return error.kind === "cancelled";
  if (!(error instanceof Error)) return false;
  const text = (error.name + " " + error.message).toLowerCase();
  return text.includes("reject") || text.includes("cancel") || text.includes("denied") || text.includes("permission");
}

function parseSafeLuna(value: string) {
  if (!/^[0-9]+$/.test(value)) throw new Error("The payment amount is not a safe integer.");
  const lunas = BigInt(value);
  if (lunas <= BigInt(0) || lunas > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new Error("The payment amount is outside the supported safe number range.");
  }
  return lunas;
}

function verificationMessage(state: ViewState) {
  if (state === "submitted") return "Transaction submitted to the network. Verification pending.";
  return "Payment is being verified. This status will update when the transaction is finalized.";
}

export function EventPurchase({ eventId, isPast, isSoldOut }: EventPurchaseProps) {
  const [state, setState] = useState<ViewState>("checking");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [purchase, setPurchase] = useState<PurchaseRecord | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);
  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (isPast) {
      setState("ready");
      return;
    }

    const stored = readAuthSession();
    if (!stored) {
      setState("anonymous");
      return;
    }
    const storedSession = stored;

    let active = true;
    async function restore() {
      try {
        const user = await fetchCurrentUser();
        const current = await fetchCurrentPurchase(eventId);
        if (!active) return;
        const refreshed: AuthSession = { ...storedSession, user };
        writeAuthSession(refreshed);
        setSession(refreshed);
        setPurchase(current);
        setError(null);
        setState(stateForPurchase(current));
      } catch (requestError) {
        if (!active) return;
        const isUnauthorized =
          (requestError instanceof AuthApiError && isLostSessionStatus(requestError.status))
          || (requestError instanceof PurchasesApiError && requestError.status === 401);
        if (isUnauthorized) {
          clearAuthSession();
          setSession(null);
          setState("anonymous");
        } else {
          setError(requestError instanceof Error ? requestError.message : "Payment status could not be retrieved.");
          setState("load-error");
        }
      }
    }

    void restore();
    return () => {
      active = false;
    };
  }, [eventId, isPast, retryKey]);

  useEffect(() => {
    const purchaseId = purchase?.id;
    const purchaseStatus = purchase?.status;
    if (!purchaseId || !session || (purchaseStatus !== "submitted" && purchaseStatus !== "verifying")) return;

    const verifiedPurchaseId = purchaseId;
    let active = true;
    let attempts = 0;

    async function poll() {
      try {
        const next = await fetchPurchase(verifiedPurchaseId);
        if (!active) return;
        setPurchase(next);
        setError(null);
        setState(stateForPurchase(next));
        if (next.status === "submitted" || next.status === "verifying") {
          attempts += 1;
          if (attempts < 40) {
            pollTimer.current = setTimeout(() => { void poll(); }, 3000);
          }
        }
      } catch (requestError) {
        if (!active) return;
        setError(requestError instanceof PurchasesApiError ? requestError.message : "Payment status could not be retrieved.");
        attempts += 1;
        if (attempts < 40) {
          pollTimer.current = setTimeout(() => { void poll(); }, 5000);
        }
      }
    }

    void poll();
    return () => {
      active = false;
      if (pollTimer.current) clearTimeout(pollTimer.current);
    };
  }, [purchase?.id, purchase?.status, session]);

  useEffect(() => {
    const purchaseId = purchase?.id;
    if (!purchaseId || !session) return;
    const coalesce = createCoalescer(300);
    const unsubscribe = subscribeRealtime({
      onEvent(event) {
        if (!isEventPurchaseEvent(event.type) || !eventTouchesResource(event, purchaseId)) return;
        coalesce.run(() => {
          void fetchPurchase(purchaseId).then((next) => {
            setPurchase(next);
            setError(null);
            setState(stateForPurchase(next));
          }).catch((requestError) => {
            setError(requestError instanceof PurchasesApiError ? requestError.message : "Payment status could not be retrieved.");
          });
        });
      },
      onReconnect() {
        coalesce.flush(() => {
          void fetchPurchase(purchaseId).then((next) => {
            setPurchase(next);
            setState(stateForPurchase(next));
          }).catch(() => undefined);
        });
      },
    });
    return () => {
      coalesce.clear();
      unsubscribe();
    };
  }, [purchase?.id, session]);

  if (isPast) return null;

  if (state === "checking") {
    return <div className="h-32 animate-pulse rounded-xl border border-border bg-surface" aria-label="Checking payment status" />;
  }

  if (state === "load-error") {
    return (
      <section className="rounded-xl border border-red-300/20 bg-red-400/10 p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-red-200">Payment</p>
        <p className="mt-2 text-sm font-medium text-foreground">Payment status could not be loaded.</p>
        <p className="mt-1 text-xs leading-5 text-muted">{error ?? "The payment service is currently unavailable."}</p>
        <Button type="button" variant="outline" className="mt-4 w-full" onClick={() => { setError(null); setState("checking"); setRetryKey((value) => value + 1); }}>Try again</Button>
      </section>
    );
  }

  if (state === "confirmed") {
    return (
      <section className="rounded-xl border border-accent/30 bg-accent/10 p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Payment</p>
        <p className="mt-2 text-sm font-medium text-foreground">Payment verified.</p>
        <p className="mt-1 text-xs leading-5 text-muted">Your attendance was confirmed in the event record. Ticket and QR flows are not supported yet.</p>
      </section>
    );
  }

  if (state === "submitted" || state === "verifying") {
    return (
      <section className="rounded-xl border border-border bg-surface p-5" aria-live="polite">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Payment</p>
        <p className="mt-2 text-sm font-medium text-foreground">{verificationMessage(state)}</p>
        <p className="mt-1 text-xs leading-5 text-muted">Your payment status is preserved in your account even if you close the page.</p>
        {error ? <p className="mt-3 text-xs leading-5 text-red-200">{error}</p> : null}
      </section>
    );
  }

  if (state === "anonymous") {
    if (isSoldOut) {
      return <Button type="button" variant="outline" disabled className="w-full">Sold out</Button>;
    }
    return (
      <NimiqConnect description="Sign in with your Nimiq wallet to start the payment." />
    );
  }

  async function handlePurchase() {
    if (!session || state === "preparing" || state === "wallet") return;
    setState("preparing");
    setError(null);

    try {
      const current = purchase && purchase.status === "pending"
        ? purchase
        : await createOrGetPurchase(eventId);
      setPurchase(current);

      if (current.status === "confirmed") {
        setState("confirmed");
        return;
      }
      if (current.status === "submitted" || current.status === "verifying") {
        setState(stateForPurchase(current));
        return;
      }

      const instructions = await fetchPaymentInstructions(current.id);
      const amount = parseSafeLuna(instructions.amount_lunas);
      const provider = await initializeMiniAppProvider();

      setState("wallet");
      const hash = await sendBasicNimTransaction(
        { recipient: instructions.recipient, valueLunas: amount },
        (tx) => provider.sendBasicTransaction(tx),
      );

      const submitted = await submitPurchaseTransaction(current.id, hash);
      setPurchase(submitted);
      setState(stateForPurchase(submitted));
    } catch (requestError) {
      if (isWalletRejection(requestError)) {
        setState("rejected");
        return;
      }
      if (requestError instanceof PurchasesApiError && requestError.status === 401) {
        clearAuthSession();
        setSession(null);
        setState("anonymous");
        setError("Your session has expired. Sign in again with your Nimiq wallet.");
        return;
      }
      setState("ready");
      setError(requestError instanceof Error ? requestError.message : "Payment could not be started.");
    }
  }

  if (state === "failed") {
    return (
      <section className="rounded-xl border border-red-300/20 bg-red-400/10 p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-red-200">Payment</p>
        <p className="mt-2 text-sm font-medium text-foreground">Payment could not be verified.</p>
        <p className="mt-1 text-xs leading-5 text-muted">The transaction did not match the event amount, recipient, sender, or network.</p>
      </section>
    );
  }

  if (state === "expired") {
    return (
      <section className="rounded-xl border border-border bg-surface p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Payment</p>
        <p className="mt-2 text-sm font-medium text-foreground">Payment expired.</p>
        <p className="mt-1 text-xs leading-5 text-muted">You can start a new payment attempt.</p>
        <Button type="button" className="mt-4 w-full" onClick={() => { setPurchase(null); setState("ready"); }}>Try again</Button>
      </section>
    );
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Payment</p>
      <p className="mt-2 text-sm leading-6 text-muted">
        {state === "rejected" ? "The transaction was cancelled in the wallet. You can try again." : "Pay securely with Nimiq Pay."}
      </p>
      {isSoldOut ? (
        <Button type="button" variant="outline" disabled className="mt-4 w-full">Sold out</Button>
      ) : (
        <Button type="button" disabled={state === "preparing" || state === "wallet"} onClick={() => { void handlePurchase(); }} className="mt-4 w-full">
          {state === "preparing" ? "Preparing…" : state === "wallet" ? "Waiting for Nimiq Pay approval…" : "Purchase"}
        </Button>
      )}
      {error ? <p className="mt-3 text-xs leading-5 text-red-200">{error}</p> : null}
    </section>
  );
}
