import { NextResponse } from "next/server";

import { publicNimiqAuthIdentity } from "@/lib/auth/nimiq-network";

export function GET() {
  const nimiq = publicNimiqAuthIdentity();
  const headers: Record<string, string> = { "cache-control": "no-store" };
  if (nimiq.configured) {
    headers["x-nimnear-nimiq-network"] = nimiq.network;
    headers["x-nimnear-nimiq-environment"] = nimiq.environment;
    headers["x-nimnear-nimiq-consensus"] = nimiq.consensus;
    headers["x-nimnear-nimiq-network-id"] = String(nimiq.networkId);
  } else {
    headers["x-nimnear-nimiq-network"] = nimiq.code;
  }

  return NextResponse.json(
    { status: "ok", nimiq },
    { status: 200, headers },
  );
}
