"use client";

import { createContext, useContext } from "react";

import type { AuthSession } from "@/lib/api/auth";
import type {
  AuthPhase,
  AuthStage,
  MiniAppWallet,
  PendingHubAuthentication,
} from "@/lib/auth/nimiq";

export type NimiqAuthContextValue = {
  stage: AuthStage;
  phase: AuthPhase;
  session: AuthSession | null;
  pendingHub: PendingHubAuthentication | null;
  miniWallet: MiniAppWallet | null;
  error: string | null;
  isOpen: boolean;
  description: string;
  open: (options?: { description?: string }) => void;
  close: () => void;
  start: () => Promise<void>;
  signWithHub: () => Promise<void>;
  signWithMiniApp: (address: string) => Promise<void>;
  signOut: () => Promise<void>;
};

export const NimiqAuthContext = createContext<NimiqAuthContextValue | null>(null);

export function useNimiqAuth() {
  const value = useContext(NimiqAuthContext);
  if (!value) {
    throw new Error("useNimiqAuth must be used within NimiqAuthProvider");
  }
  return value;
}

export function useOptionalNimiqAuth() {
  return useContext(NimiqAuthContext);
}
