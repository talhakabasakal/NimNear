"use client";

import { init, type ErrorResponse } from "@nimiq/mini-app-sdk";
import { useState } from "react";

import { Button } from "@/components/ui/button";

type NimiqConnectProps = {
  description?: string;
  blockedMessage?: string;
};

type ConnectedAccount = {
  address: string;
  count: number;
};

function isErrorResponse(result: string[] | ErrorResponse): result is ErrorResponse {
  return !Array.isArray(result);
}

function isCancellationText(text: string) {
  const normalized = text.toLowerCase();
  return normalized.includes("reject") || normalized.includes("cancel") || normalized.includes("denied") || normalized.includes("permission");
}

function isCancellation(error: unknown) {
  return error instanceof Error && isCancellationText(`${error.name} ${error.message}`);
}

function shortenAddress(address: string) {
  if (address.length <= 16) return address;
  return `${address.slice(0, 9)}…${address.slice(-6)}`;
}

export function NimiqConnect({
  description = "Nimiq Pay hesabınla devam etmek için bağlan.",
  blockedMessage,
}: NimiqConnectProps) {
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState<ConnectedAccount | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleConnect() {
    if (loading) return;

    setLoading(true);
    setConnected(null);
    setError(null);

    try {
      const nimiq = await init({ timeout: 10000 });
      const accounts = await nimiq.listAccounts();

      if (isErrorResponse(accounts)) {
        const responseText = `${accounts.error.type} ${accounts.error.message}`;
        setError(isCancellationText(responseText)
          ? "Nimiq bağlantısı iptal edildi. Tekrar deneyebilirsin."
          : accounts.error.message || "Nimiq Pay hesap bağlantısı başarısız oldu.");
        return;
      }

      if (accounts.length === 0) {
        setError("Nimiq Pay herhangi bir hesap döndürmedi.");
        return;
      }

      setConnected({ address: accounts[0], count: accounts.length });
    } catch (providerError) {
      if (isCancellation(providerError)) {
        setError("Nimiq bağlantısı iptal edildi. Tekrar deneyebilirsin.");
      } else {
        setError("Nimiq Pay sağlayıcısı bulunamadı. Bu işlemi Nimiq Pay içinde yapmalısın.");
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5" aria-labelledby="nimiq-connect-title">
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Hesap</p>
      <h2 id="nimiq-connect-title" className="mt-1 text-xl font-semibold text-foreground">Nimiq ile devam et</h2>
      <p className="mt-2 text-sm leading-6 text-muted">{description}</p>
      {connected ? (
        <div className="mt-4 rounded-lg border border-accent/20 bg-accent/10 px-3 py-3 text-sm text-foreground" role="status">
          <p className="font-medium">Nimiq hesabı bağlandı</p>
          <p className="mt-1 text-xs text-muted">{shortenAddress(connected.address)} · {connected.count} hesap</p>
          {blockedMessage ? <p className="mt-3 text-xs leading-5 text-muted">{blockedMessage}</p> : null}
        </div>
      ) : (
        <Button type="button" className="mt-4 h-11 w-full sm:w-auto" disabled={loading} onClick={() => { void handleConnect(); }}>
          {loading ? "Nimiq Pay bekleniyor…" : "Nimiq ile devam et"}
        </Button>
      )}
      {!connected && blockedMessage ? <p className="mt-3 text-xs leading-5 text-muted">{blockedMessage}</p> : null}
      {error ? <p className="mt-3 text-xs leading-5 text-red-200" role="alert">{error}</p> : null}
    </section>
  );
}
