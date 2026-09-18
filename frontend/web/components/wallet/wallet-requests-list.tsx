"use client";

import { useState } from "react";
import Link from "next/link";

import { CopyShareActions } from "@/components/payment-requests/share-actions";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  cancelPaymentRequest,
  PaymentRequestsApiError,
  type PaymentRequestRecord,
} from "@/lib/api/payment-requests";
import { paymentRequestPath } from "@/lib/payment-requests/share";
import {
  canCancelPaymentRequest,
  paymentRequestStatusCopy,
} from "@/lib/payment-requests/status";
import { formatNimAmount } from "@/lib/wallet/activity";

export function PaymentRequestStatusBadge({ status }: { status: string }) {
  const copy = paymentRequestStatusCopy(status);
  return (
    <Badge variant={copy.badge}>
      <span className="sr-only">Status: </span>
      {copy.label}
    </Badge>
  );
}

export function WalletRequestsList({
  requests,
  error,
  onRefresh,
  onUpdated,
}: {
  requests: PaymentRequestRecord[];
  error?: string | null;
  onRefresh: () => void;
  onUpdated: (request: PaymentRequestRecord) => void;
}) {
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [cancelError, setCancelError] = useState<string | null>(null);

  async function confirmCancel(request: PaymentRequestRecord) {
    if (!canCancelPaymentRequest(request.status) || busyId) return;
    setBusyId(request.public_id);
    setCancelError(null);
    try {
      const updated = await cancelPaymentRequest(request.public_id);
      onUpdated(updated);
      setConfirmId(null);
    } catch (requestError) {
      setCancelError(
        requestError instanceof PaymentRequestsApiError
          ? requestError.message
          : "The request could not be cancelled.",
      );
      onRefresh();
    } finally {
      setBusyId(null);
    }
  }

  if (error) {
    return (
      <Card>
        <CardContent className="p-4">
          <p className="text-sm font-medium">Payment requests could not be loaded</p>
          <p className="mt-1 text-xs leading-5 text-muted">{error}</p>
          <Button type="button" variant="ghost" size="sm" className="mt-3" onClick={onRefresh}>
            Try again
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (requests.length === 0) {
    return (
      <Card className="border-dashed border-border-faint bg-surface/60">
        <CardContent className="px-6 py-8 text-center">
          <p className="text-sm font-medium">No payment requests yet</p>
          <p className="mt-1 text-xs leading-5 text-muted">
            Create a request to share an exact NIM amount.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="overflow-hidden">
      <div className="divide-y divide-border">
        {requests.map((request) => {
          const confirming = confirmId === request.public_id;
          const cancellable = canCancelPaymentRequest(request.status);
          return (
            <div key={request.public_id} className="p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-foreground">
                    {formatNimAmount(request.amount_nim)} NIM
                  </p>
                  {request.note ? (
                    <p className="mt-1 break-words text-xs leading-5 text-muted">{request.note}</p>
                  ) : null}
                </div>
                <PaymentRequestStatusBadge status={request.status} />
              </div>
              {confirming ? (
                <div className="mt-4 rounded-xl border border-border bg-background p-3">
                  <p className="text-sm font-medium">Cancel this pending request?</p>
                  <p className="mt-1 text-xs leading-5 text-muted">
                    This cannot be undone from this screen. Paid or in-progress payments cannot be cancelled.
                  </p>
                  <div className="mt-3 grid grid-cols-2 gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={busyId === request.public_id}
                      onClick={() => setConfirmId(null)}
                    >
                      Keep
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      disabled={busyId === request.public_id}
                      onClick={() => void confirmCancel(request)}
                    >
                      {busyId === request.public_id ? "Cancelling…" : "Cancel request"}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="mt-3 flex flex-wrap gap-2">
                  <Link
                    href={paymentRequestPath(request.public_id)}
                    className="inline-flex h-8 items-center rounded-lg border border-border px-3 text-xs font-medium text-foreground hover:bg-surface-hover"
                  >
                    Open
                  </Link>
                  {cancellable ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setCancelError(null);
                        setConfirmId(request.public_id);
                      }}
                    >
                      Cancel
                    </Button>
                  ) : null}
                </div>
              )}
              <div className="mt-3">
                <CopyShareActions
                  publicId={request.public_id}
                  amountNim={request.amount_nim}
                  note={request.note}
                  compact
                />
              </div>
            </div>
          );
        })}
      </div>
      {cancelError ? (
        <p className="border-t border-border px-4 py-3 text-xs leading-5 text-red-200" role="alert">
          {cancelError}
        </p>
      ) : null}
    </Card>
  );
}
