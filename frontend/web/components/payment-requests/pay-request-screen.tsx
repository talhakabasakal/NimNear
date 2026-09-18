"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { PaymentRequestStatusBadge } from "@/components/wallet/wallet-requests-list";
import { RequestDetailRow } from "@/components/payment-requests/share-actions";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  fetchPublicPaymentRequest,
  PaymentRequestsApiError,
  submitPaymentRequestTransaction,
  type PublicPaymentRequest,
} from "@/lib/api/payment-requests";
import {
  clearAuthSession,
  readAuthSession,
  restoreAuthSession,
  type AuthSession,
} from "@/lib/api/auth";
import { detectNimiqAuthTransport } from "@/lib/auth/nimiq";
import { paymentNetworkMatchesDeployment } from "@/lib/auth/nimiq-network";
import {
  canInvokePaymentSend,
  consumePaymentRequestResume,
  createPaymentRequestFlight,
  executePaymentRequestPay,
  isPaymentRequestConflict,
  lockedPayTransfer,
  markPaymentRequestResume,
} from "@/lib/payment-requests/pay";
import {
  expirationView,
  isInFlightStatus,
  isLocallyExpired,
  isPayableStatus,
  nextPaymentRequestPollDelay,
  paymentRequestStatusCopy,
  statusAfterClientHash,
} from "@/lib/payment-requests/status";
import {
  createCoalescer,
  eventTouchesResource,
  isPaymentRequestEvent,
} from "@/lib/realtime/events";
import { subscribeRealtime } from "@/lib/realtime/client";
import {
  initializeMiniAppProvider,
  NimiqTransactionError,
  sendBasicNimTransaction,
  userMessageForTransactionFailure,
} from "@/lib/nimiq/transactions";
import { formatWalletNetwork } from "@/lib/wallet/send";
import { formatNimAmount } from "@/lib/wallet/activity";

type PayStep = "view" | "review" | "approving" | "submitted";
type LoadState = "loading" | "ready" | "missing" | "error";

function payerIdentity(session: AuthSession | null) {
  return session?.user.wallet_address?.trim() || "";
}

export function PayRequestScreen({ publicId }: { publicId: string }) {
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [request, setRequest] = useState<PublicPaymentRequest | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [session, setSession] = useState<AuthSession | null>(null);
  const [step, setStep] = useState<PayStep>("view");
  const [needsAuth, setNeedsAuth] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [submittedHash, setSubmittedHash] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [miniAppAvailable, setMiniAppAvailable] = useState<boolean | null>(null);
  const [pollExhausted, setPollExhausted] = useState(false);
  const flight = useRef(createPaymentRequestFlight());
  const pollTimers = useRef<number[]>([]);

  const clearPollTimers = useCallback(() => {
    for (const timer of pollTimers.current) window.clearTimeout(timer);
    pollTimers.current = [];
  }, []);

  const refreshRequest = useCallback(async () => {
    try {
      const next = await fetchPublicPaymentRequest(publicId);
      setRequest(next);
      setLoadState("ready");
      setLoadError(null);
      if (!isPayableStatus(next.status) && step === "review") setStep("view");
      if (isInFlightStatus(next.status) || next.status === "paid") {
        setStep((current) => (current === "approving" ? current : "submitted"));
      }
      return next;
    } catch (error) {
      if (error instanceof PaymentRequestsApiError && error.status === 404) {
        setLoadState("missing");
        setRequest(null);
        return null;
      }
      setLoadError(error instanceof Error ? error.message : "The payment request could not be loaded.");
      setLoadState((current) => (current === "ready" ? "ready" : "error"));
      return null;
    }
  }, [publicId, step]);

  useEffect(() => {
    setMiniAppAvailable(detectNimiqAuthTransport() === "mini-app");
    let active = true;
    async function boot() {
      const stored = readAuthSession();
      const restored = await restoreAuthSession();
      if (!active) return;
      setSession(restored ?? stored);
      try {
        const next = await fetchPublicPaymentRequest(publicId);
        if (!active) return;
        setRequest(next);
        setLoadState("ready");
        const resume = consumePaymentRequestResume(publicId);
        if (resume && (restored ?? stored)?.user.wallet_address && isPayableStatus(next.status)) {
          setStep("review");
        }
      } catch (error) {
        if (!active) return;
        if (error instanceof PaymentRequestsApiError && error.status === 404) {
          setLoadState("missing");
        } else {
          setLoadError(error instanceof Error ? error.message : "The payment request could not be loaded.");
          setLoadState("error");
        }
      }
    }
    void boot();
    return () => {
      active = false;
    };
  }, [publicId]);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!request || request.status !== "pending") return;
    const expires = Date.parse(request.expires_at);
    if (!Number.isFinite(expires)) return;
    const delay = Math.max(0, Math.min(expires - Date.now() + 50, 30_000));
    const timer = window.setTimeout(() => {
      setNow(Date.now());
      void refreshRequest();
    }, delay);
    return () => window.clearTimeout(timer);
  }, [refreshRequest, request]);

  const startStatusPoll = useCallback(() => {
    clearPollTimers();
    setPollExhausted(false);
    let attempt = 0;
    const run = async () => {
      const next = await refreshRequest();
      if (next && !isInFlightStatus(next.status)) return;
      const delay = nextPaymentRequestPollDelay(attempt);
      if (delay == null) {
        setPollExhausted(true);
        return;
      }
      attempt += 1;
      const timer = window.setTimeout(() => {
        void run();
      }, delay);
      pollTimers.current.push(timer);
    };
    void run();
  }, [clearPollTimers, refreshRequest]);

  useEffect(() => () => clearPollTimers(), [clearPollTimers]);

  useEffect(() => {
    if (!session) return;
    const coalesce = createCoalescer(300);
    const unsubscribe = subscribeRealtime({
      onEvent(event) {
        if (!isPaymentRequestEvent(event.type) || !eventTouchesResource(event, publicId)) return;
        coalesce.run(() => {
          void refreshRequest();
        });
      },
      onReconnect() {
        coalesce.flush(() => {
          void refreshRequest();
        });
      },
    });
    return () => {
      coalesce.clear();
      unsubscribe();
    };
  }, [publicId, refreshRequest, session]);

  const locallyExpired = request ? isLocallyExpired(request.expires_at, now) : false;
  const expiry = request ? expirationView(request.expires_at, now) : null;
  const locked = request ? lockedPayTransfer(request) : null;
  const fromAddress = payerIdentity(session);
  const payable = request
    ? canInvokePaymentSend(request.status, locallyExpired) && !submittedHash
    : false;
  const statusCopy = paymentRequestStatusCopy(request?.status ?? "");
  const busy = step === "approving";

  const payLabel = useMemo(() => {
    if (!request) return "Pay";
    return `Pay ${formatNimAmount(request.amount_nim)} NIM`;
  }, [request]);

  function beginPay() {
    if (!request || !payable) {
      void refreshRequest();
      return;
    }
    setNotice(null);
    if (!session) {
      markPaymentRequestResume(publicId);
      setNeedsAuth(true);
      return;
    }
    if (!fromAddress) {
      setNotice("A verified Nimiq identity is required before paying.");
      setNeedsAuth(true);
      return;
    }
    setStep("review");
  }

  async function confirmPay() {
    if (!request || !locked || busy) return;
    if (!paymentNetworkMatchesDeployment(request.network)) {
      setNotice("Payment network does not match this Nimiq deployment.");
      return;
    }
    if (miniAppAvailable === false) {
      setNotice("Open this payment request in Nimiq Pay to complete the payment.");
      return;
    }
    setNotice(null);
    setStep("approving");
    const result = await executePaymentRequestPay({
      request,
      locallyExpired,
      existingHash: submittedHash,
      flight: flight.current,
      sendTransaction: async (recipient, valueLunas) => {
        const provider = await initializeMiniAppProvider();
        return sendBasicNimTransaction(
          { recipient, valueLunas },
          (tx) => provider.sendBasicTransaction(tx),
        );
      },
      submitHash: submitPaymentRequestTransaction,
    });
    if (result.hash) setSubmittedHash(result.hash);
    if (result.ok) {
      setSubmittedHash(result.hash);
      setRequest({
        public_id: result.record.public_id,
        recipient: result.record.recipient,
        amount_lunas: result.record.amount_lunas,
        amount_nim: result.record.amount_nim,
        note: result.record.note,
        status: statusAfterClientHash(result.record.status),
        expires_at: result.record.expires_at,
        network: result.record.network,
      });
      setStep("submitted");
      startStatusPoll();
      return;
    }
    if (result.kind === "cancelled") {
      setNotice(userMessageForTransactionFailure("cancelled"));
      setStep("review");
      return;
    }
    if (result.kind === "conflict") {
      setNotice("This request is no longer waiting for a new payment.");
      setStep("view");
      void refreshRequest();
      return;
    }
    if (result.kind === "blocked") {
      setStep("view");
      void refreshRequest();
      return;
    }
    const error = result.error;
    if (error instanceof PaymentRequestsApiError && error.status === 401) {
      clearAuthSession();
      setSession(null);
      markPaymentRequestResume(publicId);
      setNeedsAuth(true);
      setStep("view");
      setNotice("Your session has expired. Sign in again to continue.");
      return;
    }
    if (isPaymentRequestConflict(error)) {
      setStep("view");
      void refreshRequest();
      return;
    }
    if (result.hash) {
      setStep("submitted");
      setNotice(
        error instanceof Error
          ? error.message
          : "Payment submitted. Verification could not be recorded yet.",
      );
      return;
    }
    const kind = error instanceof NimiqTransactionError ? error.kind : "unexpected";
    setNotice(
      error instanceof NimiqTransactionError
        ? error.message
        : error instanceof Error
          ? error.message
          : userMessageForTransactionFailure(kind),
    );
    setStep("review");
  }

  if (loadState === "loading") {
    return (
      <main className="mx-auto w-full max-w-[560px] px-4 py-6 pb-[max(2rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-10">
        <NimiqConnect
          description="Sign in with your verified Nimiq wallet to pay this request."
          restoreOnly
          hideWhenAuthenticated
        />
        <div className="space-y-4" aria-label="Loading payment request">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-10 w-40" />
          <Skeleton className="h-48 w-full" />
        </div>
      </main>
    );
  }

  if (loadState === "missing") {
    return (
      <main className="mx-auto w-full max-w-[560px] px-4 py-6 pb-[max(2rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-10">
        <NimiqConnect
          description="Sign in with your verified Nimiq wallet to pay this request."
          restoreOnly
          hideWhenAuthenticated
        />
        <Card className="border-dashed">
          <CardContent className="px-6 py-10 text-center">
            <h1 className="text-lg font-semibold">Payment request not found</h1>
            <p className="mt-2 text-sm leading-6 text-muted">
              This request does not exist or is no longer available.
            </p>
          </CardContent>
        </Card>
      </main>
    );
  }

  if (loadState === "error" || !request) {
    return (
      <main className="mx-auto w-full max-w-[560px] px-4 py-6 pb-[max(2rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-10">
        <NimiqConnect
          description="Sign in with your verified Nimiq wallet to pay this request."
          restoreOnly
          hideWhenAuthenticated
        />
        <Card>
          <CardContent className="px-6 py-8 text-center">
            <h1 className="text-lg font-semibold">Payment request could not be loaded</h1>
            <p className="mt-2 text-sm leading-6 text-muted">{loadError}</p>
            <Button type="button" variant="outline" className="mt-4" onClick={() => void refreshRequest()}>
              Try again
            </Button>
          </CardContent>
        </Card>
      </main>
    );
  }

  const showReview = step === "review" || step === "approving";
  const showSubmitted = step === "submitted" || isInFlightStatus(request.status) || request.status === "paid";

  return (
    <main className="mx-auto w-full max-w-[560px] px-4 py-6 pb-[max(2rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-10">
      <NimiqConnect
        description="Sign in with your verified Nimiq wallet to pay this request."
        autoOpen={needsAuth}
        restoreOnly={!needsAuth}
        hideWhenAuthenticated
      />
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Payment Request</p>
      <div className="mt-2 flex items-start justify-between gap-3">
        <h1 className="text-[28px] font-semibold tracking-[-0.03em] text-foreground">
          {formatNimAmount(request.amount_nim)}{" "}
          <span className="text-lg font-medium tracking-normal text-muted">NIM</span>
        </h1>
        <PaymentRequestStatusBadge status={request.status} />
      </div>
      {request.note ? (
        <p className="mt-3 break-words text-sm leading-6 text-foreground">{request.note}</p>
      ) : null}

      {showSubmitted && (isInFlightStatus(request.status) || request.status === "paid" || submittedHash) ? (
        <Card className="mt-6" aria-live="polite">
          <CardContent className="p-5">
            <p className="text-xs font-medium uppercase tracking-[0.12em] text-accent">
              {request.status === "paid" ? "Paid" : statusCopy.title}
            </p>
            <p className="mt-2 text-sm leading-6 text-foreground">
              {request.status === "paid"
                ? "This payment was verified on Nimiq."
                : pollExhausted
                  ? "Payment submitted. Verification is still in progress."
                  : statusCopy.description}
            </p>
            {submittedHash ? (
              <p className="mt-3 break-all font-mono text-xs leading-5 text-muted">{submittedHash}</p>
            ) : null}
            {request.status !== "paid" ? (
              <div className="mt-4 flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={() => void refreshRequest()}>
                  Refresh status
                </Button>
                {submittedHash && isPayableStatus(request.status) ? (
                  <Button type="button" onClick={() => void confirmPay()}>
                    Retry verification
                  </Button>
                ) : null}
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {showReview && payable && fromAddress ? (
        <section className="mt-6 space-y-5" aria-labelledby="pay-review-title">
          <h2 id="pay-review-title" className="text-lg font-semibold">
            Pay Request
          </h2>
          <div className="rounded-xl border border-border bg-surface px-4">
            <RequestDetailRow label="Amount" value={`${formatNimAmount(request.amount_nim)} NIM`} />
            <RequestDetailRow label="From" value={fromAddress} mono />
            <RequestDetailRow label="To" value={request.recipient} mono />
            {request.note ? <RequestDetailRow label="Note" value={request.note} /> : null}
            <RequestDetailRow label="Network" value={formatWalletNetwork(request.network)} />
          </div>
          {notice ? (
            <p className="text-sm leading-6 text-muted" role="status">
              {notice}
            </p>
          ) : miniAppAvailable === false ? (
            <p className="text-sm leading-6 text-muted" role="status">
              Open this payment request in Nimiq Pay to complete the payment.
            </p>
          ) : (
            <p className="text-sm leading-6 text-muted">
              Confirm in Nimiq Pay to sign this exact amount. NIMNear never handles private keys.
            </p>
          )}
          <div className="grid grid-cols-2 gap-3">
            <Button type="button" variant="outline" disabled={busy} onClick={() => setStep("view")}>
              Cancel
            </Button>
            <Button
              type="button"
              disabled={busy || miniAppAvailable === false}
              aria-disabled={busy || miniAppAvailable === false}
              onClick={() => void confirmPay()}
            >
              {busy ? "Waiting for Nimiq Pay…" : "Confirm in Nimiq Pay"}
            </Button>
          </div>
        </section>
      ) : (
        <section className="mt-6 space-y-5">
          <div className="rounded-xl border border-border bg-surface px-4">
            <RequestDetailRow label="Recipient" value={request.recipient} mono />
            <RequestDetailRow label={expiry?.expired ? "Expired" : "Expires"} value={expiry?.label ?? request.expires_at} />
            <RequestDetailRow label="Network" value={formatWalletNetwork(request.network)} />
          </div>
          {session && !fromAddress ? (
            <p className="text-sm leading-6 text-muted" role="status">
              A verified Nimiq identity is required before paying.
            </p>
          ) : null}
          {notice && step === "view" ? (
            <p className="text-sm leading-6 text-muted" role="status">
              {notice}
            </p>
          ) : null}
          {request.status === "pending" && locallyExpired ? (
            <p className="text-sm leading-6 text-muted" role="status">
              This request looks expired on this device. Refreshing status from the server.
            </p>
          ) : null}
          {request.status !== "pending" && !showSubmitted ? (
            <p className="text-sm leading-6 text-muted" role="status">
              {statusCopy.description}
            </p>
          ) : null}
          {payable ? (
            <Button
              type="button"
              className="h-11 w-full"
              disabled={busy}
              aria-disabled={busy}
              onClick={beginPay}
            >
              {payLabel}
            </Button>
          ) : null}
          {request.status === "pending" && locallyExpired ? (
            <Button type="button" variant="outline" className="w-full" onClick={() => void refreshRequest()}>
              Refresh request
            </Button>
          ) : null}
        </section>
      )}
    </main>
  );
}
