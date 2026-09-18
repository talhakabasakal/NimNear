import {
  NIMIQ_AUTH_NETWORK_TEST,
  parseNimiqAuthNetwork,
  type ResolvedNimiqAuthNetwork,
} from "@/lib/auth/nimiq-albatross";
import {
  PUBLIC_NIMIQ_HUB_ENABLED,
  PUBLIC_NIMIQ_NETWORK,
  PUBLIC_NODE_ENV,
} from "@/lib/auth/nimiq-public-env";

export {
  NIMIQ_AUTH_NETWORK_MAIN,
  NIMIQ_AUTH_NETWORK_TEST,
  NIMIQ_HUB_MAINNET,
  NIMIQ_HUB_TESTNET,
  parseNimiqAuthNetwork,
  type NimiqAuthNetwork,
  type ResolvedNimiqAuthNetwork,
} from "@/lib/auth/nimiq-albatross";

export type InvalidNimiqAuthNetwork = {
  ok: false;
  code: "nimiq_network_unconfigured";
  message: string;
};

export type NimiqAuthNetworkConfig = ResolvedNimiqAuthNetwork | InvalidNimiqAuthNetwork;

export type NimiqAuthEnvSource = Record<string, string | undefined>;

const UNCONFIGURED_MESSAGE = "NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.";

function unconfigured(): InvalidNimiqAuthNetwork {
  return {
    ok: false,
    code: "nimiq_network_unconfigured",
    message: UNCONFIGURED_MESSAGE,
  };
}

function readPublicEnv(): NimiqAuthEnvSource {
  return {
    NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK ?? PUBLIC_NIMIQ_NETWORK,
    NEXT_PUBLIC_NIMNEAR_HUB_ENABLED: process.env.NEXT_PUBLIC_NIMNEAR_HUB_ENABLED ?? PUBLIC_NIMIQ_HUB_ENABLED,
    NODE_ENV: process.env.NODE_ENV ?? PUBLIC_NODE_ENV,
  };
}

export function isNimiqHubEnabled(source: NimiqAuthEnvSource = readPublicEnv()) {
  const value = source.NEXT_PUBLIC_NIMNEAR_HUB_ENABLED?.trim().toLowerCase();
  return value !== "false" && value !== "0" && value !== "off";
}

export function isAllowedNimiqAuthNetwork(value: string) {
  return parseNimiqAuthNetwork(value) != null;
}

export function resolveNimiqAuthConfig(source: NimiqAuthEnvSource = readPublicEnv()): NimiqAuthNetworkConfig {
  const configured = source.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK?.trim();
  if (!configured) {
    // Only `next dev` may omit the public network. Any other NODE_ENV, including
    // an uninlined/undefined production bundle, must fail closed instead of
    // silently becoming TestAlbatross.
    if (source.NODE_ENV === "development") {
      return parseNimiqAuthNetwork(NIMIQ_AUTH_NETWORK_TEST)!;
    }
    return unconfigured();
  }
  return parseNimiqAuthNetwork(configured) ?? unconfigured();
}

export function requireNimiqAuthConfig(source: NimiqAuthEnvSource = readPublicEnv()): ResolvedNimiqAuthNetwork {
  const config = resolveNimiqAuthConfig(source);
  if (!config.ok) {
    throw Object.assign(new Error(config.message), { code: config.code });
  }
  return config;
}

export function nimiqNetworkLabel(source: NimiqAuthEnvSource = readPublicEnv()): ResolvedNimiqAuthNetwork["displayName"] | "" {
  const config = resolveNimiqAuthConfig(source);
  return config.ok ? config.displayName : "";
}

export function publicNimiqAuthIdentity(source: NimiqAuthEnvSource = readPublicEnv()) {
  const config = resolveNimiqAuthConfig(source);
  if (!config.ok) {
    return { configured: false as const, code: config.code };
  }
  return {
    configured: true as const,
    network: config.network,
    environment: config.environment,
    consensus: config.consensusName,
    networkId: config.networkId,
    hub: config.hubEndpoint,
  };
}

export function paymentNetworkMatchesDeployment(
  instructionsNetwork: string,
  source: NimiqAuthEnvSource = readPublicEnv(),
) {
  const configured = resolveNimiqAuthConfig(source);
  const instructed = parseNimiqAuthNetwork(instructionsNetwork);
  return configured.ok && instructed != null && configured.networkId === instructed.networkId;
}
