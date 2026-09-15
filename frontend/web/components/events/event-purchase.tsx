"use client";

import { init } from "@nimiq/mini-app-sdk";
import { useEffect, useRef, useState } from "react";

import { AuthPanel } from "@/components/auth/auth-panel";
import { Button } from "@/components/ui/button";
import {
  clearAuthSession,
  fetchCurrentUser,
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

type EventPurchaseProps = {
  eventId: string;
  isPast: boolean;
  isSoldOut: boolean;
};

type ViewState =
  | "checking"
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

const transactionHashPattern = /^[0-9a-fA-F]{64}$/;

async function initializeNimiqProvider() {
  try {
    return await init({ timeout: 10000 });
  } catch {
    throw new Error("Nimiq Pay bağlantısı bulunamadı. Ödeme için uygulamayı Nimiq Pay içinde açmalısın.");
  }
}

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
  if (!(error instanceof Error)) return false;
  const text = (error.name + " " + error.message).toLowerCase();
  return text.includes("reject") || text.includes("cancel") || text.includes("denied") || text.includes("permission");
}

function parseSafeLuna(value: string) {
  if (!/^[0-9]+$/.test(value)) throw new Error("Ödeme tutarı güvenli bir tam sayı değil.");
  const lunas = BigInt(value);
  if (lunas <= BigInt(0) || lunas > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new Error("Ödeme tutarı desteklenen güvenli sayı aralığında değil.");
  }
  return Number(lunas);
}

function verificationMessage(state: ViewState) {
  if (state === "submitted") return "İşlem ağa gönderildi. Doğrulama bekleniyor.";
  return "Ödeme doğrulanıyor. İşlem kesinleştiğinde durum burada güncellenecek.";
}

export function EventPurchase({ eventId, isPast, isSoldOut }: EventPurchaseProps) {
  const [state, setState] = useState<ViewState>("checking");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [purchase, setPurchase] = useState<PurchaseRecord | null>(null);
  const [error, setError] = useState<string | null>(null);
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
        const user = await fetchCurrentUser(storedSession.token);
        const current = await fetchCurrentPurchase(eventId, storedSession.token);
        if (!active) return;
        const refreshed: AuthSession = { ...storedSession, user };
        writeAuthSession(refreshed);
        setSession(refreshed);
        setPurchase(current);
        setState(stateForPurchase(current));
      } catch (requestError) {
        if (!active) return;
        if (requestError instanceof PurchasesApiError && requestError.status === 401) {
          clearAuthSession();
          setSession(null);
          setState("anonymous");
        } else {
          setError(requestError instanceof PurchasesApiError ? requestError.message : "Ödeme durumu alınamadı.");
          setState("ready");
        }
      }
    }

    void restore();
    return () => {
      active = false;
    };
  }, [eventId, isPast]);

  useEffect(() => {
    const purchaseId = purchase?.id;
    const purchaseStatus = purchase?.status;
    const sessionToken = session?.token;
    if (!purchaseId || !sessionToken || (purchaseStatus !== "submitted" && purchaseStatus !== "verifying")) return;

    const verifiedPurchaseId = purchaseId;
    const verifiedSessionToken = sessionToken;
    let active = true;
    let attempts = 0;

    async function poll() {
      try {
        const next = await fetchPurchase(verifiedPurchaseId, verifiedSessionToken);
        if (!active) return;
        setPurchase(next);
        setState(stateForPurchase(next));
        if (next.status === "submitted" || next.status === "verifying") {
          attempts += 1;
          if (attempts < 40) {
            pollTimer.current = setTimeout(() => { void poll(); }, 3000);
          }
        }
      } catch (requestError) {
        if (!active) return;
        setError(requestError instanceof PurchasesApiError ? requestError.message : "Ödeme durumu alınamadı.");
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
  }, [purchase?.id, purchase?.status, session?.token]);

  if (isPast) return null;

  if (state === "checking") {
    return <div className="h-32 animate-pulse rounded-xl border border-border bg-surface" aria-label="Ödeme durumu kontrol ediliyor" />;
  }

  if (state === "confirmed") {
    return (
      <section className="rounded-xl border border-accent/30 bg-accent/10 p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Ödeme</p>
        <p className="mt-2 text-sm font-medium text-foreground">Ödeme doğrulandı.</p>
        <p className="mt-1 text-xs leading-5 text-muted">Katılımın etkinlik kaydında onaylandı. Bilet ve QR akışı henüz desteklenmiyor.</p>
      </section>
    );
  }

  if (state === "submitted" || state === "verifying") {
    return (
      <section className="rounded-xl border border-border bg-surface p-5" aria-live="polite">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Ödeme</p>
        <p className="mt-2 text-sm font-medium text-foreground">{verificationMessage(state)}</p>
        <p className="mt-1 text-xs leading-5 text-muted">Sayfayı kapatsan da ödeme durumu hesabında korunur.</p>
      </section>
    );
  }

  if (state === "anonymous") {
    if (isSoldOut) {
      return <Button type="button" variant="outline" disabled className="w-full">Tükendi</Button>;
    }
    return (
      <AuthPanel
        onAuthenticated={(nextSession) => {
          setSession(nextSession);
          setState("ready");
          setError(null);
        }}
        title="Satın almak için giriş yap"
        description="Ücretli etkinliklerde ödeme yapmak için hesabınla devam et."
      />
    );
  }

  async function handlePurchase() {
    if (!session || state === "preparing" || state === "wallet") return;
    setState("preparing");
    setError(null);

    try {
      const current = purchase && purchase.status === "pending"
        ? purchase
        : await createOrGetPurchase(eventId, session.token);
      setPurchase(current);

      if (current.status === "confirmed") {
        setState("confirmed");
        return;
      }
      if (current.status === "submitted" || current.status === "verifying") {
        setState(stateForPurchase(current));
        return;
      }

      const instructions = await fetchPaymentInstructions(current.id, session.token);
      const amount = parseSafeLuna(instructions.amount_lunas);
      const provider = await initializeNimiqProvider();

      setState("wallet");
      const result = await provider.sendBasicTransaction({
        recipient: instructions.recipient,
        value: amount,
      });
      if (typeof result !== "string") {
        throw new Error(result.error?.message || "Cüzdan işlemi tamamlanamadı.");
      }
      if (!transactionHashPattern.test(result)) {
        throw new Error("Cüzdan işlem kimliği beklenen hash formatında değil.");
      }

      const submitted = await submitPurchaseTransaction(current.id, result, session.token);
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
        setError("Oturumun sona ermiş. Tekrar giriş yapmalısın.");
        return;
      }
      setState("ready");
      setError(requestError instanceof Error ? requestError.message : "Ödeme başlatılamadı.");
    }
  }

  if (state === "failed") {
    return (
      <section className="rounded-xl border border-red-300/20 bg-red-400/10 p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-red-200">Ödeme</p>
        <p className="mt-2 text-sm font-medium text-foreground">Ödeme doğrulanamadı.</p>
        <p className="mt-1 text-xs leading-5 text-muted">İşlem etkinlik tutarı, alıcı veya ağ ile eşleşmedi.</p>
      </section>
    );
  }

  if (state === "expired") {
    return (
      <section className="rounded-xl border border-border bg-surface p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Ödeme</p>
        <p className="mt-2 text-sm font-medium text-foreground">Ödeme süresi doldu.</p>
        <p className="mt-1 text-xs leading-5 text-muted">Yeni bir ödeme denemesi başlatabilirsin.</p>
        <Button type="button" className="mt-4 w-full" onClick={() => { setPurchase(null); setState("ready"); }}>Tekrar dene</Button>
      </section>
    );
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Ödeme</p>
      <p className="mt-2 text-sm leading-6 text-muted">
        {state === "rejected" ? "İşlem cüzdanda iptal edildi. İstersen tekrar deneyebilirsin." : "Nimiq Pay ile güvenli ödeme yap."}
      </p>
      {isSoldOut ? (
        <Button type="button" variant="outline" disabled className="mt-4 w-full">Tükendi</Button>
      ) : (
        <Button type="button" disabled={state === "preparing" || state === "wallet"} onClick={() => { void handlePurchase(); }} className="mt-4 w-full">
          {state === "preparing" ? "Hazırlanıyor…" : state === "wallet" ? "Nimiq Pay onayı bekleniyor…" : "Satın al"}
        </Button>
      )}
      {error ? <p className="mt-3 text-xs leading-5 text-red-200">{error}</p> : null}
    </section>
  );
}
