"use client";

import { useEffect, useState } from "react";

import { paymentRequestShareUrl, shareUrlContainsSecrets } from "@/lib/payment-requests/share";

export function PaymentRequestQr({ publicId }: { publicId: string }) {
  const [src, setSrc] = useState<string | null>(null);
  const url = paymentRequestShareUrl(publicId);

  useEffect(() => {
    let cancelled = false;
    if (!url || shareUrlContainsSecrets(url) || !url.startsWith("http")) {
      return;
    }
    void import("qrcode")
      .then((module) =>
        module.default.toDataURL(url, {
          width: 192,
          margin: 2,
          errorCorrectionLevel: "M",
          color: { dark: "#111111", light: "#ffffff" },
        }),
      )
      .then((dataUrl) => {
        if (!cancelled) setSrc(dataUrl);
      })
      .catch(() => {
        if (!cancelled) setSrc(null);
      });
    return () => {
      cancelled = true;
    };
  }, [url]);

  if (!src) return null;

  return (
    <div className="flex justify-center rounded-xl border border-border bg-white p-4">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={src}
        alt={`QR code for payment request ${publicId}`}
        width={192}
        height={192}
        className="size-48 max-w-full"
      />
    </div>
  );
}
