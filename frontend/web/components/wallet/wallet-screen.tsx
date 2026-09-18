"use client";

import {
  AlertCircle,
  ArrowDownLeft,
  ArrowUpRight,
  Check,
  CheckCircle2,
  Copy,
  HandCoins,
  Inbox,
  RefreshCw,
  Send,
  ShieldAlert,
  WifiOff,
} from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { NimiqIdenticon } from "@/components/profile/nimiq-identicon";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Sheet } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { WalletSendSheet } from "@/components/wallet/wallet-send-sheet";
import { WalletRequestSheet } from "@/components/wallet/wallet-request-sheet";
import { WalletRequestsList } from "@/components/wallet/wallet-requests-list";
import { clearAuthSession, readAuthSession, restoreAuthSession } from "@/lib/api/auth";
import {
  listPaymentRequests,
  PaymentRequestsApiError,
  type PaymentRequestRecord,
} from "@/lib/api/payment-requests";
import { subscribeRealtime } from "@/lib/realtime/client";
import { createCoalescer, isPaymentRequestEvent, isWalletActivityEvent } from "@/lib/realtime/events";
import {
  getWalletBalance,
  getWalletTransactions,
  type WalletBalance,
  type WalletTransaction,
} from "@/lib/api/wallet";
import { compactNimiqAddress, shortenNimiqAddress } from "@/lib/nimiq/address";
import { cn } from "@/lib/utils";
import {
  formatNimAmount,
  historicTransactionStatus,
  isWalletAuthError,
  isWalletIdentityError,
  isWalletRpcUnavailable,
  signedActivityAmount,
  transactionDirection,
} from "@/lib/wallet/activity";
import {
  receiveAddressView,
  requestedWalletAddress,
} from "@/lib/wallet/send";
import {
  findHistoryTransaction,
  nextWalletRefreshDelay,
  shouldKeepLocalSubmittedState,
  type SubmittedTransfer,
} from "@/lib/wallet/send-refresh";

type ScreenState = "loading" | "anonymous" | "identity-required" | "ready" | "rpc-unavailable" | "error";

function timestampDate(timestamp?: number) {
  if (timestamp == null || !Number.isFinite(timestamp)) return null;
  const milliseconds = timestamp < 10_000_000_000 ? timestamp * 1000 : timestamp;
  const date = new Date(milliseconds);
  return Number.isNaN(date.valueOf()) ? null : date;
}

function formatActivityTime(timestamp?: number) {
  const date = timestampDate(timestamp);
  if (!date) return "Time unavailable";
  const now = new Date();
  const startToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).valueOf();
  const startDate = new Date(date.getFullYear(), date.getMonth(), date.getDate()).valueOf();
  const dayDifference = Math.round((startToday - startDate) / 86_400_000);
  const time = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" }).format(date);
  if (dayDifference === 0) return `Today, ${time}`;
  if (dayDifference === 1) return `Yesterday, ${time}`;
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    year: date.getFullYear() === now.getFullYear() ? undefined : "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatDetailTime(timestamp?: number) {
  const date = timestampDate(timestamp);
  return date ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "medium" }).format(date) : null;
}

function CopyButton({
  value,
  label = "Copy",
  className,
  variant = "ghost",
}: {
  value: string;
  label?: string;
  className?: string;
  variant?: "ghost" | "outline";
}) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
    }
  }

  return (
    <Button type="button" variant={variant} size="sm" className={className} onClick={() => void copy()} aria-live="polite">
      {copied ? <Check size={14} /> : <Copy size={14} />}
      {copied ? "Copied" : label}
    </Button>
  );
}

function TransactionStatusBadge({ status }: { status?: string }) {
  const known = historicTransactionStatus(status);
  if (!status) return null;
  if (known === "confirmed") return <Badge variant="success"><CheckCircle2 size={12} />Confirmed</Badge>;
  if (known === "failed") return <Badge variant="destructive"><AlertCircle size={12} />Failed</Badge>;
  return <Badge variant="outline">{status}</Badge>;
}

export function WalletBalanceCard({
  balance,
  address,
  onReceive,
  onSend,
  onRequest,
}: {
  balance: WalletBalance;
  address: string;
  onReceive: () => void;
  onSend: () => void;
  onRequest: () => void;
}) {
  return (
    <Card className="overflow-hidden bg-[radial-gradient(circle_at_top_right,color-mix(in_srgb,var(--primary)_22%,transparent),transparent_48%)]">
      <CardContent className="p-5 sm:p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-xs font-medium text-muted">Available balance</p>
            <p className="mt-2 break-words text-[34px] font-semibold leading-none tracking-[-0.04em] text-foreground sm:text-[42px]">
              {formatNimAmount(balance.balance_nim)} <span className="text-lg tracking-normal text-muted">NIM</span>
            </p>
          </div>
          {balance.network ? <Badge variant="outline">{balance.network}</Badge> : null}
        </div>
        <div className="mt-6 grid grid-cols-3 gap-2 sm:gap-3">
          <Button type="button" size="lg" className="px-2 sm:px-4" onClick={onSend}><Send size={16} />Send</Button>
          <Button type="button" size="lg" variant="outline" className="px-2 sm:px-4" onClick={onReceive}><ArrowDownLeft size={16} />Receive</Button>
          <Button type="button" size="lg" variant="outline" className="px-2 sm:px-4" onClick={onRequest}><HandCoins size={16} />Request</Button>
        </div>
        <p className="mt-3 truncate font-mono text-[11px] text-muted" title={address}>{address}</p>
      </CardContent>
    </Card>
  );
}

function BalanceErrorCard({ onRetry }: { onRetry: () => void }) {
  return (
    <Card>
      <CardContent className="flex min-h-52 flex-col items-center justify-center px-6 text-center">
        <span className="grid size-10 place-items-center rounded-full bg-surface-hover text-muted"><WifiOff size={18} /></span>
        <p className="mt-3 text-sm font-medium">Balance is temporarily unavailable</p>
        <p className="mt-1 max-w-sm text-xs leading-5 text-muted">Your address is connected, but the Nimiq network did not return a balance.</p>
        <Button type="button" variant="ghost" size="sm" className="mt-3" onClick={onRetry}><RefreshCw size={14} />Try again</Button>
      </CardContent>
    </Card>
  );
}

function ActivitySkeleton() {
  return (
    <Card aria-label="Loading recent activity">
      <div className="divide-y divide-border">
        {Array.from({ length: 3 }, (_, index) => (
          <div key={index} className="flex items-center gap-3 p-4">
            <Skeleton className="size-10 shrink-0 rounded-full" />
            <div className="min-w-0 flex-1 space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-3 w-36" /></div>
            <div className="space-y-2"><Skeleton className="ml-auto h-3 w-16" /><Skeleton className="ml-auto h-3 w-20" /></div>
          </div>
        ))}
      </div>
    </Card>
  );
}

export function WalletTransactionList({
  transactions,
  walletAddress,
  onSelect,
}: {
  transactions: WalletTransaction[];
  walletAddress: string;
  onSelect: (transaction: WalletTransaction) => void;
}) {
  if (transactions.length === 0) {
    return (
      <Card className="border-dashed border-border-faint bg-surface/60">
        <CardContent className="flex min-h-44 flex-col items-center justify-center px-6 text-center">
          <span className="grid size-10 place-items-center rounded-full bg-surface-hover text-muted"><Inbox size={18} /></span>
          <p className="mt-3 text-sm font-medium">No transactions yet</p>
          <p className="mt-1 max-w-sm text-xs leading-5 text-muted">Incoming and outgoing NIM activity will appear here.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="overflow-hidden">
      <div className="divide-y divide-border">
        {transactions.map((transaction) => {
          const direction = transactionDirection(transaction, walletAddress);
          const counterparty = direction === "sent" ? transaction.recipient : transaction.sender;
          const DirectionIcon = direction === "sent" ? ArrowUpRight : ArrowDownLeft;
          return (
            <button
              key={transaction.hash}
              type="button"
              onClick={() => onSelect(transaction)}
              className="flex min-h-20 w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
            >
              <span className={cn("grid size-10 shrink-0 place-items-center rounded-full", direction === "received" ? "bg-emerald-400/15 text-emerald-300" : "bg-primary/15 text-accent")}><DirectionIcon size={18} /></span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2"><span className="text-sm font-medium text-foreground">{direction === "sent" ? "Sent" : "Received"}</span><TransactionStatusBadge status={transaction.status} /></span>
                <span className="mt-1 block truncate font-mono text-[11px] text-muted">{shortenNimiqAddress(counterparty)}</span>
              </span>
              <span className="min-w-0 shrink-0 text-right">
                <span className={cn("block text-sm font-semibold", direction === "received" ? "text-emerald-300" : "text-foreground")}>{signedActivityAmount(direction, transaction.value_nim)} NIM</span>
                <span className="mt-1 block text-[11px] text-muted">{formatActivityTime(transaction.timestamp)}</span>
              </span>
            </button>
          );
        })}
      </div>
    </Card>
  );
}

function DetailRow({ label, value, copyValue }: { label: string; value: string | null; copyValue?: string }) {
  return (
    <div className="border-b border-border py-3 last:border-0">
      <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted">{label}</p>
      <div className="mt-1 flex min-w-0 items-start justify-between gap-3">
        <p className="min-w-0 break-all text-sm leading-6 text-foreground">{value ?? "Not provided"}</p>
        {copyValue ? <CopyButton value={copyValue} /> : null}
      </div>
    </div>
  );
}

function TransactionDetailSheet({ transaction, walletAddress, onClose }: { transaction: WalletTransaction | null; walletAddress: string; onClose: () => void }) {
  if (!transaction) return null;
  const direction = transactionDirection(transaction, walletAddress);
  const network = transaction.network ?? (transaction.network_id != null ? String(transaction.network_id) : null);
  return (
    <Sheet open onOpenChange={(open) => { if (!open) onClose(); }} title="Transaction details" description="Network details reported for this transaction.">
      <div className="mb-4 flex items-center justify-between gap-3 rounded-xl border border-border bg-surface p-4">
        <div><p className="text-xs text-muted">{direction === "sent" ? "Sent" : "Received"}</p><p className="mt-1 text-2xl font-semibold tracking-[-0.03em]">{signedActivityAmount(direction, transaction.value_nim)} NIM</p></div>
        <TransactionStatusBadge status={transaction.status} />
      </div>
      <div className="rounded-xl border border-border bg-surface px-4">
        <DetailRow label="Status" value={transaction.status ?? null} />
        <DetailRow label="Amount" value={`${formatNimAmount(transaction.value_nim)} NIM`} />
        <DetailRow label="From" value={transaction.sender} copyValue={transaction.sender} />
        <DetailRow label="To" value={transaction.recipient} copyValue={transaction.recipient} />
        <DetailRow label="Transaction hash" value={transaction.hash} copyValue={transaction.hash} />
        <DetailRow label="Block" value={transaction.block_number != null ? String(transaction.block_number) : null} />
        <DetailRow label="Timestamp" value={formatDetailTime(transaction.timestamp)} />
        <DetailRow label="Network" value={network} />
      </div>
    </Sheet>
  );
}

function ReceiveSheet({ address, open, onOpenChange }: { address: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const receive = receiveAddressView(address);
  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Receive NIM" description="Share this verified Nimiq address with the sender.">
      <div className="flex flex-col items-center rounded-xl border border-border bg-surface p-5 text-center">
        <NimiqIdenticon seed={receive.identiconSeed} className="size-20" alt="Wallet identicon" />
        <p className="mt-4 w-full break-all font-mono text-sm leading-6 text-foreground">{receive.display}</p>
        <div className="mt-4 w-full">
          <CopyButton value={receive.copy} label="Copy Address" variant="outline" className="h-11 w-full" />
        </div>
      </div>
    </Sheet>
  );
}

function SubmittedTransferCard({
  transfer,
  history,
  onRefresh,
}: {
  transfer: SubmittedTransfer;
  history: WalletTransaction[];
  onRefresh: () => void;
}) {
  if (!shouldKeepLocalSubmittedState(transfer, history)) return null;
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase tracking-[0.12em] text-accent">Submitted</p>
            <p className="mt-2 text-sm font-medium text-foreground">Waiting for network history</p>
            <p className="mt-1 text-xs leading-5 text-muted">
              {transfer.amountNim} NIM to {shortenNimiqAddress(transfer.recipient)}. Historic activity can take a moment to appear.
            </p>
          </div>
          <Button type="button" variant="ghost" size="sm" onClick={onRefresh}>
            <RefreshCw size={14} />Refresh
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function WalletHeader({ address }: { address: string }) {
  return (
    <div className="flex min-w-0 items-center gap-3">
      <NimiqIdenticon seed={compactNimiqAddress(address)} className="size-12 shrink-0" alt="Wallet identicon" />
      <div className="min-w-0 flex-1"><p className="text-xs font-medium text-muted">Connected wallet</p><p className="mt-0.5 truncate font-mono text-sm font-medium text-foreground">{shortenNimiqAddress(address)}</p></div>
      <CopyButton value={address} label="Copy" />
    </div>
  );
}

function WalletLoading() {
  return (
    <div className="space-y-6" aria-label="Loading wallet">
      <div className="flex items-center gap-3"><Skeleton className="size-12 rounded-full" /><div className="space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-4 w-36" /></div></div>
      <Card><CardContent className="space-y-5 p-5 sm:p-6"><Skeleton className="h-3 w-28" /><Skeleton className="h-10 w-56 max-w-full" /><div className="grid grid-cols-3 gap-3"><Skeleton className="h-11" /><Skeleton className="h-11" /><Skeleton className="h-11" /></div></CardContent></Card>
      <div><Skeleton className="mb-4 h-6 w-36" /><ActivitySkeleton /></div>
    </div>
  );
}

function WalletState({ icon: Icon, title, description, onRetry }: { icon: typeof AlertCircle; title: string; description: string; onRetry?: () => void }) {
  return (
    <Card className="border-dashed border-border-faint bg-surface/60">
      <CardContent className="flex min-h-64 flex-col items-center justify-center px-6 text-center">
        <span className="grid size-11 place-items-center rounded-full bg-surface-hover text-muted"><Icon size={19} /></span>
        <h2 className="mt-4 text-base font-semibold">{title}</h2>
        <p className="mt-1 max-w-md text-sm leading-6 text-muted">{description}</p>
        {onRetry ? <Button type="button" variant="outline" className="mt-5" onClick={onRetry}><RefreshCw size={15} />Try again</Button> : null}
      </CardContent>
    </Card>
  );
}

export function WalletScreen() {
  const [screenState, setScreenState] = useState<ScreenState>("loading");
  const [balance, setBalance] = useState<WalletBalance | null>(null);
  const [transactions, setTransactions] = useState<WalletTransaction[] | null>(null);
  const [walletAddress, setWalletAddress] = useState("");
  const [balanceError, setBalanceError] = useState(false);
  const [activityError, setActivityError] = useState(false);
  const [selectedTransaction, setSelectedTransaction] = useState<WalletTransaction | null>(null);
  const [receiveOpen, setReceiveOpen] = useState(false);
  const [sendOpen, setSendOpen] = useState(false);
  const [requestOpen, setRequestOpen] = useState(false);
  const [paymentRequests, setPaymentRequests] = useState<PaymentRequestRecord[]>([]);
  const [paymentRequestsError, setPaymentRequestsError] = useState<string | null>(null);
  const [pendingSend, setPendingSend] = useState<SubmittedTransfer | null>(null);
  const walletAddressRef = useRef(walletAddress);
  const refreshTimers = useRef<number[]>([]);

  const clearRefreshTimers = useCallback(() => {
    for (const timer of refreshTimers.current) window.clearTimeout(timer);
    refreshTimers.current = [];
  }, []);

  const applyWalletResults = useCallback((
    requested: string,
    balanceResult: PromiseSettledResult<WalletBalance>,
    transactionResult: PromiseSettledResult<{ data: WalletTransaction[] }>,
  ): ScreenState | "ready" => {
    const errors = [balanceResult, transactionResult]
      .filter((result): result is PromiseRejectedResult => result.status === "rejected")
      .map((result) => result.reason as unknown);
    if (errors.some(isWalletAuthError)) {
      clearAuthSession();
      return "anonymous";
    }
    if (errors.some(isWalletIdentityError)) return "identity-required";
    if (errors.length === 2) return errors.some(isWalletRpcUnavailable) ? "rpc-unavailable" : "error";
    if (balanceResult.status === "fulfilled") {
      setBalance(balanceResult.value);
      setWalletAddress(balanceResult.value.address || requested);
      walletAddressRef.current = balanceResult.value.address || requested;
      setBalanceError(false);
    } else {
      setBalance(null);
      setBalanceError(true);
    }
    if (transactionResult.status === "fulfilled") {
      setTransactions(transactionResult.value.data);
      setActivityError(false);
    } else {
      setTransactions(null);
      setActivityError(true);
    }
    return "ready";
  }, []);

  const loadPaymentRequests = useCallback(async () => {
    try {
      const items = await listPaymentRequests(undefined, 20);
      setPaymentRequests(items);
      setPaymentRequestsError(null);
    } catch (error) {
      if (error instanceof PaymentRequestsApiError && error.status === 401) {
        clearAuthSession();
        setScreenState("anonymous");
        return;
      }
      setPaymentRequestsError(
        error instanceof Error ? error.message : "Payment requests could not be loaded.",
      );
    }
  }, []);

  const loadWallet = useCallback(async () => {
    setScreenState("loading");
    setBalanceError(false);
    setActivityError(false);
    const stored = readAuthSession();
    const restored = await restoreAuthSession();
    const session = restored ?? stored;
    if (!session) {
      setScreenState("anonymous");
      return;
    }
    const queryAddress = typeof window === "undefined"
      ? ""
      : new URLSearchParams(window.location.search).get("address");
    const requested = requestedWalletAddress(queryAddress, session.user.wallet_address);
    if (!requested) {
      setScreenState("identity-required");
      return;
    }
    setWalletAddress(requested);
    walletAddressRef.current = requested;

    const [balanceResult, transactionResult] = await Promise.allSettled([
      getWalletBalance(undefined, requested),
      getWalletTransactions({ address: requested, max: 20 }),
    ]);
    setScreenState(applyWalletResults(requested, balanceResult, transactionResult));
    void loadPaymentRequests();
  }, [applyWalletResults, loadPaymentRequests]);

  const refreshWalletData = useCallback(async () => {
    const address = walletAddressRef.current;
    if (!address) return [];
    const [balanceResult, transactionResult] = await Promise.allSettled([
      getWalletBalance(undefined, address),
      getWalletTransactions({ address, max: 20 }),
    ]);
    const nextState = applyWalletResults(address, balanceResult, transactionResult);
    if (nextState !== "ready") setScreenState(nextState);
    return transactionResult.status === "fulfilled" ? transactionResult.value.data : [];
  }, [applyWalletResults]);

  const startPostSendRefresh = useCallback((hash: string) => {
    clearRefreshTimers();
    let attempt = 0;
    const run = async () => {
      const history = await refreshWalletData();
      if (findHistoryTransaction(history, hash)) return;
      attempt += 1;
      const delay = nextWalletRefreshDelay(attempt);
      if (delay == null) return;
      const timer = window.setTimeout(() => { void run(); }, delay);
      refreshTimers.current.push(timer);
    };
    const initial = nextWalletRefreshDelay(0) ?? 0;
    const timer = window.setTimeout(() => { void run(); }, initial);
    refreshTimers.current.push(timer);
  }, [clearRefreshTimers, refreshWalletData]);

  useEffect(() => {
    void loadWallet();
    return () => clearRefreshTimers();
  }, [clearRefreshTimers, loadWallet]);

  useEffect(() => {
    if (screenState !== "ready") return;
    const requests = createCoalescer(300);
    const wallet = createCoalescer(300);
    const unsubscribe = subscribeRealtime({
      onEvent(event) {
        if (isPaymentRequestEvent(event.type)) {
          requests.run(() => {
            void loadPaymentRequests();
          });
        }
        if (isWalletActivityEvent(event.type)) {
          wallet.run(() => {
            void refreshWalletData();
          });
        }
      },
      onReconnect() {
        requests.flush(() => {
          void loadPaymentRequests();
        });
        wallet.flush(() => {
          void refreshWalletData();
        });
      },
    });
    return () => {
      requests.clear();
      wallet.clear();
      unsubscribe();
    };
  }, [loadPaymentRequests, refreshWalletData, screenState]);

  const activity = useMemo(() => transactions ?? [], [transactions]);
  const visiblePending = pendingSend && shouldKeepLocalSubmittedState(pendingSend, activity)
    ? pendingSend
    : null;

  return (
    <main className="mx-auto w-full max-w-[760px] px-4 py-6 pb-[max(2rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-10 lg:px-8">
      <div className="mb-6"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Nimiq wallet</p><h1 className="mt-1 text-[28px] font-semibold tracking-[-0.03em] text-foreground">Wallet</h1></div>
      {screenState === "loading" ? <WalletLoading /> : null}
      {screenState === "anonymous" ? <NimiqConnect description="Sign in with your verified Nimiq wallet to see your balance and activity." blockedMessage="Wallet data is available after secure Nimiq sign-in." /> : null}
      {screenState === "identity-required" ? (
        <div className="space-y-4">
          <WalletState icon={ShieldAlert} title="Verified Nimiq identity required" description="This account does not have a verified Nimiq wallet address. Connect and verify a wallet before viewing wallet data." />
          <Link href="/profile" className={cn(buttonVariants({ variant: "outline" }), "w-full")}>Go to profile</Link>
        </div>
      ) : null}
      {screenState === "rpc-unavailable" ? <WalletState icon={WifiOff} title="Nimiq network is unavailable" description="Balance and transaction activity could not be refreshed. Please try again in a moment." onRetry={() => void loadWallet()} /> : null}
      {screenState === "error" ? <WalletState icon={AlertCircle} title="Wallet data could not be loaded" description="We could not complete the wallet request. Check your connection and try again." onRetry={() => void loadWallet()} /> : null}
      {screenState === "ready" ? (
        <div className="space-y-7">
          <WalletHeader address={walletAddress} />
          {balance && !balanceError ? (
            <WalletBalanceCard
              balance={balance}
              address={walletAddress}
              onReceive={() => setReceiveOpen(true)}
              onSend={() => setSendOpen(true)}
              onRequest={() => setRequestOpen(true)}
            />
          ) : (
            <BalanceErrorCard onRetry={() => void loadWallet()} />
          )}
          <section aria-labelledby="payment-requests-title">
            <div className="mb-4 flex items-end justify-between gap-4">
              <div>
                <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Requests</p>
                <h2 id="payment-requests-title" className="mt-1 text-xl font-semibold tracking-[-0.02em]">Payment requests</h2>
              </div>
              <Button type="button" variant="ghost" size="sm" onClick={() => void loadPaymentRequests()}>
                <RefreshCw size={14} />Refresh
              </Button>
            </div>
            <WalletRequestsList
              requests={paymentRequests}
              error={paymentRequestsError}
              onRefresh={() => void loadPaymentRequests()}
              onUpdated={(updated) => {
                setPaymentRequests((current) =>
                  current.map((item) => (item.public_id === updated.public_id ? updated : item)),
                );
              }}
            />
          </section>
          <section aria-labelledby="recent-activity-title">
            <div className="mb-4 flex items-end justify-between gap-4">
              <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Activity</p><h2 id="recent-activity-title" className="mt-1 text-xl font-semibold tracking-[-0.02em]">Recent activity</h2></div>
              <Button type="button" variant="ghost" size="sm" onClick={() => void refreshWalletData()}>
                <RefreshCw size={14} />Refresh
              </Button>
            </div>
            {visiblePending ? (
              <div className="mb-3">
                <SubmittedTransferCard
                  transfer={visiblePending}
                  history={activity}
                  onRefresh={() => void refreshWalletData()}
                />
              </div>
            ) : null}
            {activityError ? <WalletState icon={WifiOff} title="Activity is temporarily unavailable" description="Recent transactions could not be refreshed from the Nimiq network." onRetry={() => void refreshWalletData()} /> : transactions ? <WalletTransactionList transactions={activity} walletAddress={walletAddress} onSelect={setSelectedTransaction} /> : <ActivitySkeleton />}
          </section>
        </div>
      ) : null}
      <ReceiveSheet address={walletAddress} open={receiveOpen && Boolean(walletAddress)} onOpenChange={setReceiveOpen} />
      <WalletRequestSheet
        open={requestOpen && Boolean(walletAddress)}
        onOpenChange={setRequestOpen}
        selectedAddress={walletAddress}
        onCreated={(created) => {
          setPaymentRequests((current) => [created, ...current.filter((item) => item.public_id !== created.public_id)].slice(0, 20));
        }}
      />
      {balance && !balanceError ? (
        <WalletSendSheet
          open={sendOpen}
          onOpenChange={setSendOpen}
          fromAddress={walletAddress}
          balanceLunas={balance.balance_lunas}
          balanceNim={balance.balance_nim}
          network={balance.network}
          onSubmitted={(transfer) => {
            setPendingSend(transfer);
            startPostSendRefresh(transfer.hash);
          }}
        />
      ) : null}
      <TransactionDetailSheet transaction={selectedTransaction} walletAddress={walletAddress} onClose={() => setSelectedTransaction(null)} />
    </main>
  );
}
