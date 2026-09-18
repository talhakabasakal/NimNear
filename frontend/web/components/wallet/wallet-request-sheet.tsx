"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { PaymentRequestQr } from "@/components/payment-requests/payment-request-qr";
import { CopyShareActions, RequestDetailRow } from "@/components/payment-requests/share-actions";
import { Button } from "@/components/ui/button";
import { Sheet } from "@/components/ui/sheet";
import { userFacingCaughtError } from "@/lib/api/http-error";
import {
  createPaymentRequest,
  type PaymentRequestRecord,
} from "@/lib/api/payment-requests";
import { createSingleFlight } from "@/lib/nimiq/transactions";
import { formatNimAmount } from "@/lib/wallet/activity";
import { validateCreatePaymentRequest } from "@/lib/payment-requests/create";
import { expirationView } from "@/lib/payment-requests/status";

type RequestStep = "form" | "creating" | "created";

const fieldClass =
  "mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 text-sm text-foreground outline-none transition focus-visible:ring-2 focus-visible:ring-ring";

function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return <p className="mt-2 text-xs leading-5 text-red-200" role="alert">{message}</p>;
}

export function WalletRequestSheet({
  open,
  onOpenChange,
  selectedAddress,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedAddress: string;
  onCreated: (request: PaymentRequestRecord) => void;
}) {
  const [step, setStep] = useState<RequestStep>("form");
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const [amountError, setAmountError] = useState<string | null>(null);
  const [noteError, setNoteError] = useState<string | null>(null);
  const [identityError, setIdentityError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [created, setCreated] = useState<PaymentRequestRecord | null>(null);
  const busy = step === "creating";
  const amountRef = useRef<HTMLInputElement>(null);
  const flight = useRef(createSingleFlight<PaymentRequestRecord>());

  useEffect(() => {
    if (!open) return;
    setStep("form");
    setAmount("");
    setNote("");
    setAmountError(null);
    setNoteError(null);
    setIdentityError(null);
    setNotice(null);
    setCreated(null);
  }, [open, selectedAddress]);

  const title = useMemo(() => {
    if (step === "created") return "Payment Request Created";
    return "Request NIM";
  }, [step]);

  function close() {
    if (busy) return;
    onOpenChange(false);
  }

  async function submit() {
    if (flight.current.busy) return;
    const result = validateCreatePaymentRequest({
      amount,
      note,
      selectedAddress,
    });
    if (!result.ok) {
      setAmountError(result.field === "amount" ? result.message : null);
      setNoteError(result.field === "note" ? result.message : null);
      setIdentityError(result.field === "identity" ? result.message : null);
      return;
    }
    setAmountError(null);
    setNoteError(null);
    setIdentityError(null);
    setNotice(null);
    setStep("creating");
    try {
      const record = await flight.current.run(() =>
        createPaymentRequest({
          amount_nim: result.value.amountNim,
          ...(result.value.note ? { note: result.value.note } : {}),
          address: result.value.address,
        }),
      );
      setCreated(record);
      setStep("created");
      onCreated(record);
    } catch (error) {
      setStep("form");
      setNotice(userFacingCaughtError(error, "The payment request could not be created."));
    }
  }

  const expiry = created ? expirationView(created.expires_at) : null;

  return (
    <Sheet
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) close();
      }}
      title={title}
      description={
        step === "created"
          ? "Share this link. Status is confirmed by the NIMNear backend, not by this device."
          : "Create a shareable request for an exact NIM amount."
      }
    >
      {step === "form" || step === "creating" ? (
        <form
          className="flex flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <label className="block text-[11px] font-medium uppercase tracking-[0.12em] text-muted">
            Amount
            <div className="relative">
              <input
                ref={amountRef}
                value={amount}
                onChange={(event) => {
                  setAmount(event.target.value);
                  setAmountError(null);
                }}
                onFocus={(event) => event.currentTarget.scrollIntoView({ block: "center" })}
                inputMode="decimal"
                placeholder="25.00"
                autoComplete="off"
                className={fieldClass + " pr-14"}
                disabled={busy}
              />
              <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted">
                NIM
              </span>
            </div>
            <FieldError message={amountError ?? undefined} />
          </label>
          <label className="block text-[11px] font-medium uppercase tracking-[0.12em] text-muted">
            Note (optional)
            <input
              value={note}
              onChange={(event) => {
                setNote(event.target.value);
                setNoteError(null);
              }}
              onFocus={(event) => event.currentTarget.scrollIntoView({ block: "center" })}
              maxLength={140}
              placeholder="Dinner"
              className={fieldClass}
              disabled={busy}
            />
            <FieldError message={noteError ?? undefined} />
          </label>
          <div>
            <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted">
              Selected receiving identity
            </p>
            <p className="mt-2 break-all font-mono text-sm leading-6 text-foreground">
              {selectedAddress || "No verified identity selected"}
            </p>
            <FieldError message={identityError ?? undefined} />
          </div>
          {notice ? (
            <p className="text-sm leading-6 text-red-200" role="alert">
              {notice}
            </p>
          ) : (
            <p className="text-sm leading-6 text-muted">
              The recipient is your verified Nimiq identity. Amount and destination cannot be changed by the payer.
            </p>
          )}
          <div className="mt-2 grid grid-cols-2 gap-3">
            <Button type="button" variant="outline" disabled={busy} onClick={close}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || !selectedAddress}>
              {busy ? "Creating…" : "Create Request"}
            </Button>
          </div>
        </form>
      ) : null}

      {step === "created" && created ? (
        <div className="flex flex-col gap-5">
          <div className="rounded-xl border border-border bg-surface p-4">
            <p className="text-2xl font-semibold tracking-[-0.03em]">
              {formatNimAmount(created.amount_nim)} <span className="text-base font-medium text-muted">NIM</span>
            </p>
            {created.note ? (
              <p className="mt-2 break-words text-sm leading-6 text-foreground">{created.note}</p>
            ) : null}
          </div>
          <div className="rounded-xl border border-border bg-surface px-4">
            <RequestDetailRow label="Recipient" value={created.recipient} mono />
            <RequestDetailRow label="Expires" value={expiry?.detail ?? created.expires_at} />
          </div>
          <PaymentRequestQr publicId={created.public_id} />
          <div>
            <p className="mb-2 text-[11px] font-medium uppercase tracking-[0.12em] text-muted">Share link</p>
            <CopyShareActions
              publicId={created.public_id}
              amountNim={created.amount_nim}
              note={created.note}
            />
          </div>
          <Button type="button" onClick={() => onOpenChange(false)}>
            Done
          </Button>
        </div>
      ) : null}
    </Sheet>
  );
}
