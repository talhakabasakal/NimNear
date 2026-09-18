"use client";

import { useEffect, useRef } from "react";

import { useNimiqAuth } from "@/components/auth/nimiq-auth-context";
import { Button } from "@/components/ui/button";
import { accountDisplayName } from "@/lib/api/auth";

type NimiqConnectProps = {
  description?: string;
  blockedMessage?: string;
  autoOpen?: boolean;
  restoreOnly?: boolean;
  hideWhenAuthenticated?: boolean;
};

export function NimiqConnect({
  description = "Sign in securely with your Nimiq wallet.",
  blockedMessage,
  autoOpen = false,
  restoreOnly = false,
  hideWhenAuthenticated = false,
}: NimiqConnectProps) {
  const {
    stage,
    session,
    isOpen,
    open,
    signOut,
  } = useNimiqAuth();
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!autoOpen) return;
    if (stage === "authenticated" || stage === "restoring-session") return;
    open({ description });
  }, [autoOpen, description, open, stage]);

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

  if (restoreOnly && !autoOpen) return null;

  return (
    <div>
      <Button
        ref={triggerRef}
        type="button"
        className="h-11"
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        onClick={() => open({ description })}
      >
        Sign in with Nimiq
      </Button>
      {blockedMessage ? (
        <p className="mt-3 text-xs leading-5 text-muted">{blockedMessage}</p>
      ) : null}
    </div>
  );
}
