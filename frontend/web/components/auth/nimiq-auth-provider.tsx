"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { getHostLanguage } from "@nimiq/mini-app-sdk";

import {
  NimiqAuthContext,
  type NimiqAuthContextValue,
} from "@/components/auth/nimiq-auth-context";
import { NimiqConnectDialog } from "@/components/auth/nimiq-connect-dialog";
import {
  clearAuthSession,
  logout,
  writeAuthSession,
  type AuthSession,
} from "@/lib/api/auth";
import {
  authenticateMiniApp,
  beginHubAuthentication,
  completeHubAuthentication,
  createHubChallenge,
  detectNimiqAuthTransport,
  isMiniAppHost,
  NimiqAuthError,
  persistHubAuthContinuation,
  prepareNimiqHub,
  readPendingHubAuthentication,
  readSelectedHubAddress,
  resumeHubAuthFlow,
  userMessageForAuthPhase,
  type AuthPhase,
  type AuthStage,
  type MiniAppWallet,
  type PendingHubAuthentication,
} from "@/lib/auth/nimiq";
import { isNimiqHubEnabled } from "@/lib/auth/nimiq-network";

function messageForError(value: unknown): string | null {
  if (value instanceof NimiqAuthError) {
    if (value.cancelled) return null;
    return value.message || userMessageForAuthPhase(value.phase, value.code);
  }
  if (value instanceof Error && value.message) return value.message;
  return "Nimiq sign-in could not be completed.";
}

export function NimiqAuthProvider({ children }: { children: ReactNode }) {
  const [stage, setStage] = useState<AuthStage>("restoring-session");
  const [phase, setPhase] = useState<AuthPhase>("restore-session");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [miniWallet, setMiniWallet] = useState<MiniAppWallet | null>(null);
  const [pendingHub, setPendingHub] = useState<PendingHubAuthentication | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [description, setDescription] = useState("Sign in securely with your Nimiq wallet.");

  const showError = useCallback((value: unknown) => {
    const authError = value instanceof NimiqAuthError ? value : null;
    if (authError) setPhase(authError.phase);
    setError(messageForError(value));
    setStage(readPendingHubAuthentication() ? "awaiting-signature" : "idle");
  }, []);

  const finish = useCallback((nextSession: AuthSession) => {
    writeAuthSession(nextSession);
    setSession(nextSession);
    setStage("authenticated");
    setPhase("restore-session");
    setIsOpen(false);
    setError(null);
    console.info("[auth] session restored");
    window.location.reload();
  }, []);

  useEffect(() => {
    prepareNimiqHub();
    let active = true;
    void resumeHubAuthFlow().then((result) => {
      if (!active) return;
      setPhase(result.phase);
      if (result.type === "authenticated") {
        setSession(result.session);
        setStage("authenticated");
        return;
      }
      if (result.type === "payment") {
        setStage("idle");
        return;
      }
      if (result.type === "cancelled") {
        const pending = readPendingHubAuthentication();
        setError(null);
        setPendingHub(pending);
        setStage(pending ? "awaiting-signature" : "idle");
        setIsOpen(Boolean(pending));
        return;
      }
      if (result.type === "error") {
        showError(result.error);
        setIsOpen(true);
        return;
      }
      if (result.type === "awaiting-signature" || result.type === "redirecting") {
        if (result.pending) setPendingHub(result.pending);
        setStage(result.type === "redirecting" ? "requesting-signature" : "awaiting-signature");
        setIsOpen(true);
        return;
      }
      setStage("idle");
    }).catch((value) => {
      if (!active) return;
      showError(value);
      setIsOpen(true);
    });
    return () => {
      active = false;
    };
  }, [showError]);

  const open = useCallback((options?: { description?: string }) => {
    if (options?.description) setDescription(options.description);
    setIsOpen(true);
  }, []);

  const close = useCallback(() => {
    setIsOpen(false);
  }, []);

  const start = useCallback(async () => {
    const existing = pendingHub ?? readPendingHubAuthentication();
    if (existing) {
      setError(null);
      setPendingHub(existing);
      setStage("requesting-signature");
      setPhase("sign-challenge");
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
      setPhase("request-challenge");
      try {
        const pending = await createHubChallenge(selectedAddress);
        setPendingHub(pending);
        setStage("requesting-signature");
        setPhase("sign-challenge");
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
      setPhase("choose-address");
      persistHubAuthContinuation({ phase: "choose-address", addressSelected: false });
      const pending = await beginHubAuthentication();
      if (!pending) return;
      setPendingHub(pending);
      setStage("awaiting-signature");
      setPhase("sign-challenge");
    } catch (value) {
      showError(value);
    }
  }, [finish, pendingHub, showError, stage]);

  const signWithMiniApp = useCallback(async (address: string) => {
    if (!miniWallet) return;
    setError(null);
    setStage("requesting-challenge");
    setPhase("request-challenge");
    try {
      finish(await authenticateMiniApp(miniWallet, address));
    } catch (value) {
      showError(value);
    }
  }, [finish, miniWallet, showError]);

  const signWithHub = useCallback(async () => {
    if (!pendingHub) return;
    setError(null);
    setStage("requesting-signature");
    setPhase("sign-challenge");
    try {
      const nextSession = await completeHubAuthentication(pendingHub);
      if (nextSession) finish(nextSession);
    } catch (value) {
      showError(value);
    }
  }, [finish, pendingHub, showError]);

  const signOut = useCallback(async () => {
    await logout().catch(() => undefined);
    clearAuthSession();
    setSession(null);
    setPendingHub(null);
    setMiniWallet(null);
    setStage("idle");
    setPhase("idle");
    window.location.reload();
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

  const value = useMemo<NimiqAuthContextValue>(() => ({
    stage,
    phase,
    session,
    pendingHub,
    miniWallet,
    error,
    isOpen,
    description,
    open,
    close,
    start,
    signWithHub,
    signWithMiniApp,
    signOut,
  }), [
    close,
    description,
    error,
    isOpen,
    miniWallet,
    open,
    pendingHub,
    phase,
    session,
    signOut,
    signWithHub,
    signWithMiniApp,
    stage,
    start,
  ]);

  return (
    <NimiqAuthContext.Provider value={value}>
      {children}
      <NimiqConnectDialog />
    </NimiqAuthContext.Provider>
  );
}
