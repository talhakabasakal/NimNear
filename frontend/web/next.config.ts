import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Development-only origin required when Nimiq Pay loads the LAN URL.
  allowedDevOrigins: ["192.168.1.11"],
};

export default nextConfig;
