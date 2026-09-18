"use client";

import { X } from "lucide-react";
import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";

import { trapTabKey } from "@/lib/a11y/focus-trap";
import { useNimiqAuth } from "@/components/auth/nimiq-auth-context";
import { Button } from "@/components/ui/button";
import { clearHubUiIntent, type AuthStage } from "@/lib/auth/nimiq";
import { nimiqNetworkLabel, resolveNimiqAuthConfig } from "@/lib/auth/nimiq-network";

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

export function NimiqConnectDialog() {
  const {
    stage,
    pendingHub,
    miniWallet,
    error,
    isOpen,
    description,
    close,
    start,
    signWithHub,
    signWithMiniApp,
  } = useNimiqAuth();
  const closeRef = useRef<HTMLButtonElement>(null);
  const dialogRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeRef.current?.focus();

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        close();
        if (stage !== "authenticated") clearHubUiIntent();
      }
      if (dialogRef.current) trapTabKey(dialogRef.current, event);
    };
    window.addEventListener("keydown", closeOnEscape);

    return () => {
      window.removeEventListener("keydown", closeOnEscape);
      document.body.style.overflow = previousOverflow;
    };
  }, [close, isOpen, stage]);

  if (!isOpen || typeof document === "undefined") return null;

  return createPortal(
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          close();
          if (stage !== "authenticated") clearHubUiIntent();
        }
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
          onClick={() => {
            close();
            if (stage !== "authenticated") clearHubUiIntent();
          }}
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
  );
}
