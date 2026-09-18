"use client";

import { X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { trapTabKey } from "@/lib/a11y/focus-trap";

import { Button } from "@/components/ui/button";
import {
  accountDisplayName,
  clearAuthSession,
  logout,
  restoreAuthSession,
  writeAuthSession,
  type AuthSession,
} from "@/lib/api/auth";
import {
  authenticateMiniApp,
  beginHubAuthentication,
  claimHubReturnRestore,
  clearHubUiIntent,
  completeHubAuthentication,
  createHubChallenge,
  detectNimiqAuthTransport,
  finishHubSignature,
  hasHubUiIntent,
  isHubPaymentRedirect,
  isMiniAppHost,
  NimiqAuthError,
  prepareNimiqHub,
  readPendingHubAuthentication,
  readSelectedHubAddress,
  releaseHubReturnRestore,
  takeHubRedirectResult,
  type AuthStage,
  type MiniAppWallet,
  type PendingHubAuthentication,
} from "@/lib/auth/nimiq";
import { isNimiqHubEnabled, nimiqNetworkLabel, resolveNimiqAuthConfig } from "@/lib/auth/nimiq-network";
import { getHostLanguage } from "@nimiq/mini-app-sdk";

type NimiqConnectProps = {
  description?: string;
  blockedMessage?: string;
  autoOpen?: boolean;
  restoreOnly?: boolean;
  hideWhenAuthenticated?: boolean;
};

const stageLabel: Partial<Record<AuthStage, string>> = {
  "detecting-environment": "Detecting wallet environment…",
  "requesting-wallet": "Waiting for wallet permission…",
  "selecting-account": "Select an account",
  "requesting-challenge": "Preparing a secure sign-in message…",
  "awaiting-signature": "Waiting for signature approval",
  "requesting-signature": "Waiting for wallet signature…",
  "verifying-signature": "Verifying signature…",
  "creating-session": "Creating session…",
  "restoring-session": "Restoring session…",
};

function configuredHubLabel() {
  const network = resolveNimiqAuthConfig();
  return network.ok ? network.hubLabel : "Nimiq Hub";
}

function shortenAddress(address: string) {
  return address.length <= 18
    ? address
    : `${address.slice(0, 9)}…${address.slice(-6)}`;
}

export function NimiqConnect({
  description = "Sign in securely with your Nimiq wallet.",
  blockedMessage,
  autoOpen = false,
  restoreOnly = false,
  hideWhenAuthenticated = false,
}: NimiqConnectProps) {
  const [stage, setStage] = useState<AuthStage>("restoring-session");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [miniWallet, setMiniWallet] = useState<MiniAppWallet | null>(null);
  const [pendingHub, setPendingHub] = useState<PendingHubAuthentication | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const dialogRef = useRef<HTMLElement>(null);

  useEffect(() => {
    prepareNimiqHub();
    const ownsHubReturn = claimHubReturnRestore();
    let active = true;

    async function restore() {
      try {
        const restored = await restoreAuthSession();
        if (!active) return;
        if (restored) {
          setSession(restored);
          setStage("authenticated");
          return;
        }

        if (!ownsHubReturn) {
          const pending = readPendingHubAuthentication();
          if (pending) {
            setPendingHub(pending);
            setStage("awaiting-signature");
            setIsOpen(hasHubUiIntent());
            return;
          }
          setStage("idle");
          return;
        }

        const redirected = await takeHubRedirectResult();
        if (!active) return;
        if (isHubPaymentRedirect(redirected)) {
          setStage("idle");
          return;
        }
        if (redirected.type === "error") {
          showError(redirected.error);
          setIsOpen(true);
          return;
        }
        if (redirected.type === "address") {
          setIsOpen(true);
          setStage("requesting-challenge");
          const pending = await createHubChallenge(redirected.address, undefined, redirected.label);
          if (!active) return;
          setPendingHub(pending);
          setStage("requesting-signature");
          const nextSession = await completeHubAuthentication(pending);
          if (!active) return;
          if (nextSession) finish(nextSession);
          return;
        }
        if (redirected.type === "signature") {
          const pending = readPendingHubAuthentication();
          if (!pending) {
            throw new NimiqAuthError(
              "verifying-signature",
              "missing_hub_challenge",
              "The Hub signature was received, but the sign-in message was not found. Try again.",
            );
          }
          setIsOpen(true);
          setPendingHub(pending);
          setStage("verifying-signature");
          finish(await finishHubSignature(pending, redirected.signed));
          return;
        }

        const pending = readPendingHubAuthentication();
        if (pending) {
          setPendingHub(pending);
          setStage("awaiting-signature");
          setIsOpen(hasHubUiIntent());
          return;
        }

        const selectedAddress = readSelectedHubAddress();
        if (selectedAddress) {
          setIsOpen(true);
          setStage("requesting-challenge");
          const pendingFromAddress = await createHubChallenge(selectedAddress);
          if (!active) return;
          setPendingHub(pendingFromAddress);
          setStage("requesting-signature");
          const nextSession = await completeHubAuthentication(pendingFromAddress);
          if (!active) return;
          if (nextSession) finish(nextSession);
          return;
        }

        setStage("idle");
        if (hasHubUiIntent()) setIsOpen(true);
      } catch (value) {
        if (!active) return;
        showError(value);
        setIsOpen(true);
      }
    }

    void restore();
    return () => {
      active = false;
      if (ownsHubReturn) releaseHubReturnRestore();
    };
  }, []);

  useEffect(() => {
    function onPageShow() {
      setStage((current) => {
        if (current !== "requesting-wallet" && current !== "requesting-signature") return current;
        return readPendingHubAuthentication() ? "awaiting-signature" : "idle";
      });
    }
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, []);

  useEffect(() => {
    if (!autoOpen) return;
    if (stage === "authenticated" || stage === "restoring-session") return;
    setIsOpen(true);
  }, [autoOpen, stage]);

  useEffect(() => {
    if (!isOpen) return;
    const previousOverflow = document.body.style.overflow;
    const trigger = triggerRef.current;
    document.body.style.overflow = "hidden";
    closeRef.current?.focus();

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") closeModal();
      if (dialogRef.current) trapTabKey(dialogRef.current, event);
    };
    window.addEventListener("keydown", closeOnEscape);

    return () => {
      window.removeEventListener("keydown", closeOnEscape);
      document.body.style.overflow = previousOverflow;
      trigger?.focus();
    };
  }, [isOpen]);

  function closeModal() {
    setIsOpen(false);
    if (stage !== "authenticated") clearHubUiIntent();
  }

  function finish(nextSession: AuthSession) {
    writeAuthSession(nextSession);
    setSession(nextSession);
    setStage("authenticated");
    setIsOpen(false);
    console.info("[auth] session restored");
    window.location.reload();
  }

  function showError(value: unknown) {
    const message =
      value instanceof NimiqAuthError
        ? value.message
        : value instanceof Error
          ? value.message
          : "Nimiq sign-in could not be completed.";
    setError(message);
    setStage(pendingHub || readPendingHubAuthentication() ? "awaiting-signature" : "idle");
  }

  async function start() {
    const existing = pendingHub ?? readPendingHubAuthentication();
    if (existing) {
      setError(null);
      setPendingHub(existing);
      setStage("requesting-signature");
      try {
        const nextSession = await completeHubAuthentication(existing);
        if (nextSession) finish(nextSession);
      } catch (value) {
        showError(value);
      }
      return;
    }

    const selectedAddress = readSelectedHubAddress();
    if (selectedAddress) {
      setError(null);
      setStage("requesting-challenge");
      try {
        const pending = await createHubChallenge(selectedAddress);
        setPendingHub(pending);
        setStage("requesting-signature");
        const nextSession = await completeHubAuthentication(pending);
        if (nextSession) finish(nextSession);
      } catch (value) {
        showError(value);
      }
      return;
    }

    if (stage !== "idle") return;
    setError(null);
    setStage("detecting-environment");
    try {
      const transport = detectNimiqAuthTransport();
      if (transport === "mini-app") {
        if (!isNimiqHubEnabled() && !isMiniAppHost({
          hasNimiqPay: Boolean(window.nimiqPay),
          hasNimiqProvider: Boolean(window.nimiq),
          hostLanguage: getHostLanguage(),
        })) {
          throw new NimiqAuthError(
            "detecting-environment",
            "hub_disabled",
            "Open NIMNear in Nimiq Pay to sign in. Browser Hub fallback is disabled.",
          );
        }
        setStage("requesting-wallet");
        const wallet = await import("@/lib/auth/nimiq").then(
          ({ requestMiniAppWallet }) => requestMiniAppWallet(),
        );
        setMiniWallet(wallet);
        if (wallet.accounts.length > 1) {
          setStage("selecting-account");
          return;
        }
        setStage("requesting-challenge");
        finish(await authenticateMiniApp(wallet, wallet.accounts[0]));
        return;
      }
      setStage("requesting-wallet");
      const pending = await beginHubAuthentication();
      if (!pending) return;
      setPendingHub(pending);
      setStage("awaiting-signature");
    } catch (value) {
      showError(value);
    }
  }

  async function signWithMiniApp(address: string) {
    if (!miniWallet) return;
    setError(null);
    setStage("requesting-challenge");
    try {
      finish(await authenticateMiniApp(miniWallet, address));
    } catch (value) {
      showError(value);
    }
  }

  async function signWithHub() {
    if (!pendingHub) return;
    setError(null);
    setStage("requesting-signature");
    try {
      const nextSession = await completeHubAuthentication(pendingHub);
      if (nextSession) finish(nextSession);
    } catch (value) {
      showError(value);
    }
  }

  async function signOut() {
    await logout().catch(() => undefined);
    clearAuthSession();
    setSession(null);
    setPendingHub(null);
    setMiniWallet(null);
    setStage("idle");
    window.location.reload();
  }

  if (stage === "restoring-session") {
    if (restoreOnly || hideWhenAuthenticated) return null;
    return (
      <div
        className="h-11 w-44 animate-pulse rounded-lg border border-border bg-surface"
        aria-label="Checking session"
      />
    );
  }

  if (session) {
    if (hideWhenAuthenticated) return null;
    const name = accountDisplayName(session.user);
    return (
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-surface px-4 py-3">
        <div className="min-w-0">
          <p className="text-sm text-foreground">{name}</p>
          {session.user.wallet_address ? (
            <p className="mt-0.5 truncate font-mono text-xs text-muted">{session.user.wallet_address}</p>
          ) : null}
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            void signOut();
          }}
        >
          Sign out
        </Button>
      </div>
    );
  }

  const modal = isOpen
    ? createPortal(
        <div
          className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) closeModal();
          }}
        >
          <section
            ref={dialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="nimiq-connect-title"
            aria-describedby="nimiq-connect-description"
            className="relative w-full max-w-md rounded-2xl border border-border bg-surface p-6 shadow-2xl"
          >
            <button
              ref={closeRef}
              type="button"
              aria-label="Close sign-in dialog"
              className="absolute right-4 top-4 rounded-lg p-2 text-muted transition-colors hover:bg-surface-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              onClick={closeModal}
            >
              <X className="size-5" aria-hidden="true" />
            </button>

            <p className="pr-10 text-xs font-medium uppercase tracking-[0.16em] text-accent">
              Account
            </p>
            <h2
              id="nimiq-connect-title"
              className="mt-1 pr-10 text-xl font-semibold text-foreground"
            >
              Continue with Nimiq
            </h2>
            <p
              id="nimiq-connect-description"
              className="mt-2 text-sm leading-6 text-muted"
            >
              {description}
            </p>

            {stage === "selecting-account" && miniWallet ? (
              <div
                className="mt-5 space-y-2"
                role="group"
                aria-label="Select a Nimiq account"
              >
                {miniWallet.accounts.map((address) => (
                  <Button
                    key={address}
                    type="button"
                    variant="outline"
                    className="w-full justify-start"
                    onClick={() => {
                      void signWithMiniApp(address);
                    }}
                  >
                    {shortenAddress(address)}
                  </Button>
                ))}
              </div>
            ) : pendingHub ? (
              <div className="mt-5 rounded-lg border border-accent/20 bg-accent/10 p-3">
                <p className="text-sm font-medium text-foreground">
                  Address selected: {shortenAddress(pendingHub.address)}
                </p>
                <p className="mt-1 text-xs leading-5 text-muted">
                  Press the button again to sign the secure sign-in message.
                  The {configuredHubLabel()} opens in a full page and returns here after signing.
                </p>
                <Button
                  type="button"
                  className="mt-3 w-full"
                  disabled={stage === "requesting-signature"}
                  onClick={() => {
                    void signWithHub();
                  }}
                >
                  {stage === "requesting-signature"
                    ? stageLabel[stage]
                    : "Sign the sign-in message"}
                </Button>
              </div>
            ) : (
              <Button
                type="button"
                className="mt-5 h-11 w-full"
                disabled={stage !== "idle"}
                onClick={() => {
                  void start();
                }}
              >
                {stage === "idle"
                  ? "Open Nimiq wallet"
                  : (stageLabel[stage] ?? "Waiting…")}
              </Button>
            )}

            <p
              className="mt-3 min-h-5 text-center text-xs text-muted"
              aria-live="polite"
            >
              {stage !== "idle" && !pendingHub
                ? stageLabel[stage]
                : nimiqNetworkLabel()}
            </p>
            {error ? (
              <p
                className="mt-2 text-center text-xs leading-5 text-red-200"
                role="alert"
              >
                {error}
              </p>
            ) : null}
          </section>
        </div>,
        document.body,
      )
    : null;

  if (restoreOnly && !autoOpen) {
    return modal;
  }

  return (
    <div>
      <Button
        ref={triggerRef}
        type="button"
        className="h-11"
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        onClick={() => setIsOpen(true)}
      >
        Sign in with Nimiq
      </Button>
      {blockedMessage ? (
        <p className="mt-3 text-xs leading-5 text-muted">{blockedMessage}</p>
      ) : null}
      {modal}
    </div>
  );
}
