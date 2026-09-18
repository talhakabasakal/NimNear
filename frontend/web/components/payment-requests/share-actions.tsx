"use client";

import { Check, Copy, Share2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  canUseWebShare,
  copyText,
  paymentRequestShareText,
  paymentRequestShareUrl,
  sharePaymentRequest,
} from "@/lib/payment-requests/share";

export function RequestDetailRow({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="border-b border-border py-3 last:border-0">
      <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted">{label}</p>
      <p className={mono ? "mt-1 break-all font-mono text-sm leading-6 text-foreground" : "mt-1 break-words text-sm leading-6 text-foreground"}>
        {value}
      </p>
    </div>
  );
}

export function CopyShareActions({
  publicId,
  amountNim,
  note,
  compact = false,
}: {
  publicId: string;
  amountNim: string;
  note?: string | null;
  compact?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const [shareError, setShareError] = useState<string | null>(null);
  const url = paymentRequestShareUrl(publicId);
  const webShare = canUseWebShare();

  async function copyLink() {
    if (!url) {
      setShareError("A secure HTTPS payment link is not configured.");
      return;
    }
    setShareError(null);
    try {
      await copyText(url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
      setShareError("The link could not be copied.");
    }
  }

  async function share() {
    setShareError(null);
    try {
      await sharePaymentRequest({
        title: "NIMNear payment request",
        text: paymentRequestShareText({ amountNim, note }),
        url,
      });
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setShareError("Sharing was not completed. You can copy the link instead.");
    }
  }

  return (
    <div className="space-y-3">
      {compact ? null : url ? (
        <p className="break-all font-mono text-xs leading-5 text-muted">{url}</p>
      ) : (
        <p className="text-xs leading-5 text-red-200" role="alert">Payment links require a canonical HTTPS origin.</p>
      )}
      <div className={compact ? "flex flex-wrap gap-2" : webShare ? "grid grid-cols-2 gap-3" : ""}>
        <Button
          type="button"
          variant={compact || webShare ? "outline" : "default"}
          size={compact ? "sm" : "default"}
          className={compact ? "" : "h-11 w-full"}
          onClick={() => void copyLink()}
          aria-live="polite"
        >
          {copied ? <Check size={compact ? 14 : 16} /> : <Copy size={compact ? 14 : 16} />}
          {copied ? "Copied" : "Copy Link"}
        </Button>
        {webShare ? (
          <Button
            type="button"
            size={compact ? "sm" : "default"}
            className={compact ? "" : "h-11 w-full"}
            onClick={() => void share()}
          >
            <Share2 size={compact ? 14 : 16} />
            Share
          </Button>
        ) : null}
      </div>
      {shareError ? (
        <p className="text-xs leading-5 text-red-200" role="alert">
          {shareError}
        </p>
      ) : null}
    </div>
  );
}
