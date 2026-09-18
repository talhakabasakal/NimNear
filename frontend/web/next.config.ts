import { networkInterfaces } from "node:os";
import type { NextConfig } from "next";

import { resolveNimiqAuthConfig } from "./lib/auth/nimiq-network";

const detectedLanHosts = Object.values(networkInterfaces())
  .flatMap((entries) => entries ?? [])
  .filter((entry) => entry.family === "IPv4" && !entry.internal)
  .map((entry) => entry.address);
const configuredLanHost = process.env.NIMNEAR_DEV_LAN_IP?.trim();
const configuredNimiqNetwork = resolveNimiqAuthConfig();
if (!configuredNimiqNetwork.ok) {
  throw new Error(configuredNimiqNetwork.message);
}

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
