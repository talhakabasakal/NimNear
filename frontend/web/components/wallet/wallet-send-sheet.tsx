"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Sheet } from "@/components/ui/sheet";
import { detectNimiqAuthTransport } from "@/lib/auth/nimiq";
import {
  createSingleFlight,
  initializeMiniAppProvider,
  NimiqTransactionError,
  sendBasicNimTransaction,
  userMessageForTransactionFailure,
} from "@/lib/nimiq/transactions";
import { formatNimAmount } from "@/lib/wallet/activity";
import {
  formatWalletNetwork,
  validateWalletSend,
  type ValidatedWalletSend,
} from "@/lib/wallet/send";

type SendStep = "form" | "review" | "approving" | "submitted";

type WalletSendSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  fromAddress: string;
  balanceLunas?: string;
  balanceNim?: string;
  network?: string;
  onSubmitted: (transfer: {
    hash: string;
    recipient: string;
    amountNim: string;
    from: string;
  }) => void;
};

const fieldClass =
  "mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 text-sm text-foreground outline-none transition focus-visible:ring-2 focus-visible:ring-ring";

function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="mt-2 text-xs leading-5 text-red-200">{message}</p>;
}

function ReviewRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-b border-border py-3 last:border-0">
      <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted">{label}</p>
      <p className="mt-1 break-all text-sm leading-6 text-foreground">{value}</p>
    </div>
  );
}

export function WalletSendSheet({
  open,
  onOpenChange,
  fromAddress,
  balanceLunas,
  balanceNim,
  network,
  onSubmitted,
}: WalletSendSheetProps) {
  const [step, setStep] = useState<SendStep>("form");
  const [recipient, setRecipient] = useState("");
  const [amount, setAmount] = useState("");
  const [recipientError, setRecipientError] = useState<string | null>(null);
  const [amountError, setAmountError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [draft, setDraft] = useState<ValidatedWalletSend | null>(null);
  const [submittedHash, setSubmittedHash] = useState<string | null>(null);
  const [miniAppAvailable, setMiniAppAvailable] = useState<boolean | null>(null);
  const flight = useRef(createSingleFlight<string>());
  const recipientRef = useRef<HTMLInputElement>(null);

  const networkLabel = formatWalletNetwork(network);
  const busy = step === "approving";

  useEffect(() => {
    if (!open) return;
    setMiniAppAvailable(detectNimiqAuthTransport() === "mini-app");
    setStep("form");
    setRecipient("");
    setAmount("");
    setRecipientError(null);
    setAmountError(null);
    setNotice(null);
    setDraft(null);
    setSubmittedHash(null);
  }, [open, fromAddress]);

  const title = useMemo(() => {
    if (step === "review" || step === "approving") return "Review send";
    if (step === "submitted") return "Submitted";
    return "Send NIM";
  }, [step]);

  function close() {
    if (busy) return;
    onOpenChange(false);
  }

  function validateForm() {
    const result = validateWalletSend({
      recipient,
      amount,
      fromAddress,
      balanceLunas,
    });
    if (!result.ok) {
      setRecipientError(result.field === "recipient" ? result.message : null);
      setAmountError(result.field === "amount" ? result.message : null);
      return null;
    }
    setRecipientError(null);
    setAmountError(null);
    setDraft(result.value);
    return result.value;
  }

  function continueToReview() {
    const next = validateForm();
    if (!next) return;
    setNotice(null);
    setStep("review");
  }

  async function confirmSend() {
    const next = draft ?? validateForm();
    if (!next || flight.current.busy) return;
    if (miniAppAvailable === false) {
      setNotice("Sending NIM is available in Nimiq Pay. Hub sign-in cannot submit this transfer.");
      return;
    }
    setNotice(null);
    setStep("approving");
    try {
      const hash = await flight.current.run(async () => {
        const provider = await initializeMiniAppProvider();
        return sendBasicNimTransaction(
          { recipient: next.recipient, valueLunas: next.amountLunas },
          (tx) => provider.sendBasicTransaction(tx),
        );
      });
      setSubmittedHash(hash);
      setStep("submitted");
      onSubmitted({
        hash,
        recipient: next.recipient,
        amountNim: next.amountNim,
        from: next.from,
      });
    } catch (error) {
      if (error instanceof NimiqTransactionError && error.kind === "cancelled") {
        setNotice(userMessageForTransactionFailure("cancelled"));
        setStep("review");
        return;
      }
      const kind = error instanceof NimiqTransactionError ? error.kind : "unexpected";
      setNotice(
        error instanceof NimiqTransactionError
          ? error.message
          : userMessageForTransactionFailure(kind),
      );
      setStep("review");
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) close();
      }}
      title={title}
      description={
        step === "submitted"
          ? "The wallet accepted this transfer. Network confirmation can take a moment."
          : "Send NIM from your verified Nimiq address."
      }
    >
      {step === "form" ? (
        <form
          className="flex flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            continueToReview();
          }}
        >
          <label className="block text-[11px] font-medium uppercase tracking-[0.12em] text-muted">
            Recipient
            <input
              ref={recipientRef}
              value={recipient}
              onChange={(event) => {
                setRecipient(event.target.value);
                setRecipientError(null);
              }}
              onFocus={(event) => event.currentTarget.scrollIntoView({ block: "center" })}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              inputMode="text"
              placeholder="NQ…"
              className={fieldClass + " font-mono"}
            />
            <FieldError message={recipientError ?? undefined} />
          </label>
          <label className="block text-[11px] font-medium uppercase tracking-[0.12em] text-muted">
            Amount
            <div className="relative">
              <input
                value={amount}
                onChange={(event) => {
                  setAmount(event.target.value);
                  setAmountError(null);
                }}
                onFocus={(event) => event.currentTarget.scrollIntoView({ block: "center" })}
                inputMode="decimal"
                placeholder="0.00000"
                className={fieldClass + " pr-14"}
              />
              <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted">
                NIM
              </span>
            </div>
            <FieldError message={amountError ?? undefined} />
          </label>
          {balanceNim ? (
            <p className="text-xs leading-5 text-muted">
              Available {formatNimAmount(balanceNim)} NIM
            </p>
          ) : null}
          <div className="mt-2 grid grid-cols-2 gap-3">
            <Button type="button" variant="outline" onClick={close}>
              Cancel
            </Button>
            <Button type="submit">Continue</Button>
          </div>
        </form>
      ) : null}

      {step === "review" || step === "approving" ? (
        <div className="flex flex-col gap-5">
          <div className="rounded-xl border border-border bg-surface px-4">
            <ReviewRow label="From" value={draft?.from ?? fromAddress} />
            <ReviewRow label="To" value={draft?.recipient ?? recipient} />
            <ReviewRow label="Amount" value={`${draft?.amountNim ?? amount} NIM`} />
            <ReviewRow label="Network" value={networkLabel} />
          </div>
          {notice ? (
            <p className="text-sm leading-6 text-muted" role="status">
              {notice}
            </p>
          ) : miniAppAvailable === false ? (
            <p className="text-sm leading-6 text-muted" role="status">
              Sending NIM is available in Nimiq Pay. Hub sign-in cannot submit this transfer.
            </p>
          ) : (
            <p className="text-sm leading-6 text-muted">
              Confirm in Nimiq Pay to sign and submit this transfer. NIMNear never handles private keys.
            </p>
          )}
          <div className="grid grid-cols-2 gap-3">
            <Button type="button" variant="outline" disabled={busy} onClick={() => setStep("form")}>
              Cancel
            </Button>
            <Button type="button" disabled={busy || miniAppAvailable === false} onClick={() => void confirmSend()}>
              {busy ? "Waiting for Nimiq Pay…" : "Confirm in Nimiq Pay"}
            </Button>
          </div>
        </div>
      ) : null}

      {step === "submitted" ? (
        <div className="flex flex-col gap-5">
          <div className="rounded-xl border border-border bg-surface p-4">
            <p className="text-xs font-medium uppercase tracking-[0.12em] text-accent">Submitted</p>
            <p className="mt-2 text-sm leading-6 text-foreground">
              The transfer was submitted. Confirmation appears after the network includes it in history.
            </p>
            {draft ? (
              <p className="mt-3 text-2xl font-semibold tracking-[-0.03em]">
                {draft.amountNim} <span className="text-base font-medium text-muted">NIM</span>
              </p>
            ) : null}
          </div>
          {submittedHash ? (
            <div className="rounded-xl border border-border bg-surface px-4">
              <ReviewRow label="Transaction hash" value={submittedHash} />
              <ReviewRow label="To" value={draft?.recipient ?? recipient} />
            </div>
          ) : null}
          <Button type="button" onClick={() => onOpenChange(false)}>
            Done
          </Button>
        </div>
      ) : null}
    </Sheet>
  );
}
