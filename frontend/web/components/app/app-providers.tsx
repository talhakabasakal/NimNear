"use client";

import type { ReactNode } from "react";

import { NimiqAuthProvider } from "@/components/auth/nimiq-auth-provider";

export function AppProviders({ children }: { children: ReactNode }) {
  return <NimiqAuthProvider>{children}</NimiqAuthProvider>;
}
