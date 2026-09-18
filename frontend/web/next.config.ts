import { networkInterfaces } from "node:os";
import type { NextConfig } from "next";

import {
  NIMIQ_AUTH_NETWORK_TEST,
  parseNimiqAuthNetwork,
} from "./lib/auth/nimiq-albatross";

const detectedLanHosts = Object.values(networkInterfaces())
  .flatMap((entries) => entries ?? [])
  .filter((entry) => entry.family === "IPv4" && !entry.internal)
  .map((entry) => entry.address);
const configuredLanHost = process.env.NIMNEAR_DEV_LAN_IP?.trim();

// Resolve and re-inject the canonical public network here so the client bundle
// cannot miss NEXT_PUBLIC inlining. next.config.ts must not import the env-read
// module used by the browser bundle.
const configuredNimiqNetwork = (() => {
  const raw = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK?.trim();
  if (!raw) {
    if (process.env.NODE_ENV === "development") {
      return parseNimiqAuthNetwork(NIMIQ_AUTH_NETWORK_TEST);
    }
    throw new Error("NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.");
  }
  const parsed = parseNimiqAuthNetwork(raw);
  if (!parsed) {
    throw new Error("NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.");
  }
  return parsed;
})();
if (!configuredNimiqNetwork) {
  throw new Error("NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.");
}
process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = configuredNimiqNetwork.network;

const hubEnabled = process.env.NEXT_PUBLIC_NIMNEAR_HUB_ENABLED?.trim().toLowerCase() !== "false";

const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(self)" },
  {
    key: "Cross-Origin-Opener-Policy",
    value: hubEnabled ? "same-origin-allow-popups" : "same-origin",
  },
  { key: "Cross-Origin-Embedder-Policy", value: "unsafe-none" },
];

const nextConfig: NextConfig = {
  // Canonicalize and force-inline so the browser bundle cannot miss the public
  // network the way NEXT_PUBLIC_* inlining can when this file used to import
  // the client env module.
  env: {
    NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: configuredNimiqNetwork.network,
  },
  // Docker/self-hosted uses standalone. Vercel infers its own output and
  // must not inherit a conflicting standalone export.
  ...(process.env.VERCEL ? {} : { output: "standalone" as const }),
  transpilePackages: ["@nimiq/identicons"],
  // Development-only origins used when Nimiq Pay loads the app over the LAN.
  allowedDevOrigins: Array.from(new Set([
    ...detectedLanHosts,
    ...(configuredLanHost ? [configuredLanHost] : []),
  ])),
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
};

export default nextConfig;
