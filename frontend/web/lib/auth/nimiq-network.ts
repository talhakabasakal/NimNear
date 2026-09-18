export const NIMIQ_HUB_TESTNET = "https://hub.nimiq-testnet.com";
export const NIMIQ_HUB_MAINNET = "https://hub.nimiq.com";

export const NIMIQ_AUTH_NETWORK_TEST = "test-albatross";
export const NIMIQ_AUTH_NETWORK_MAIN = "main-albatross";

export type NimiqAuthNetwork = typeof NIMIQ_AUTH_NETWORK_TEST | typeof NIMIQ_AUTH_NETWORK_MAIN;

export type ResolvedNimiqAuthNetwork = {
  ok: true;
  network: NimiqAuthNetwork;
  environment: "testnet" | "mainnet";
  consensusName: "TestAlbatross" | "MainAlbatross";
  networkId: 5 | 24;
  hubEndpoint: typeof NIMIQ_HUB_TESTNET | typeof NIMIQ_HUB_MAINNET;
  hubLabel: "Nimiq Testnet Hub" | "Nimiq Hub";
  displayName: "Nimiq Testnet" | "Nimiq Mainnet";
};

export type InvalidNimiqAuthNetwork = {
  ok: false;
  code: "nimiq_network_unconfigured";
  message: string;
};

export type NimiqAuthNetworkConfig = ResolvedNimiqAuthNetwork | InvalidNimiqAuthNetwork;

type EnvSource = Record<string, string | undefined>;

const TESTNET: ResolvedNimiqAuthNetwork = {
  ok: true,
  network: NIMIQ_AUTH_NETWORK_TEST,
  environment: "testnet",
  consensusName: "TestAlbatross",
  networkId: 5,
  hubEndpoint: NIMIQ_HUB_TESTNET,
  hubLabel: "Nimiq Testnet Hub",
  displayName: "Nimiq Testnet",
};

const MAINNET: ResolvedNimiqAuthNetwork = {
  ok: true,
  network: NIMIQ_AUTH_NETWORK_MAIN,
  environment: "mainnet",
  consensusName: "MainAlbatross",
  networkId: 24,
  hubEndpoint: NIMIQ_HUB_MAINNET,
  hubLabel: "Nimiq Hub",
  displayName: "Nimiq Mainnet",
};

const UNCONFIGURED_MESSAGE = "NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.";

function unconfigured(): InvalidNimiqAuthNetwork {
  return {
    ok: false,
    code: "nimiq_network_unconfigured",
    message: UNCONFIGURED_MESSAGE,
  };
}

// Next.js inlines process.env.NEXT_PUBLIC_* and process.env.NODE_ENV only on
// static member access. Passing process.env as an object would hide production
// MainAlbatross and silently look like an unconfigured development Testnet.
function readPublicEnv(): EnvSource {
  return {
    NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK,
    NEXT_PUBLIC_NIMNEAR_HUB_ENABLED: process.env.NEXT_PUBLIC_NIMNEAR_HUB_ENABLED,
    NODE_ENV: process.env.NODE_ENV,
  };
}

export function isNimiqHubEnabled(source: EnvSource = readPublicEnv()) {
  const value = source.NEXT_PUBLIC_NIMNEAR_HUB_ENABLED?.trim().toLowerCase();
  return value !== "false" && value !== "0" && value !== "off";
}

export function isAllowedNimiqAuthNetwork(value: string) {
  return parseNimiqAuthNetwork(value) != null;
}

export function parseNimiqAuthNetwork(value: string): ResolvedNimiqAuthNetwork | null {
  switch (value.trim().toLowerCase().replace(/[-_\s]/g, "")) {
    case "testalbatross":
    case "testnet":
      return TESTNET;
    case "mainalbatross":
    case "mainnet":
      return MAINNET;
    default:
      return null;
  }
}

export function resolveNimiqAuthConfig(source: EnvSource = readPublicEnv()): NimiqAuthNetworkConfig {
  const configured = source.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK?.trim();
  if (!configured) {
    if (source.NODE_ENV === "production") {
      return unconfigured();
    }
    return TESTNET;
  }
  return parseNimiqAuthNetwork(configured) ?? unconfigured();
}

export function requireNimiqAuthConfig(source: EnvSource = readPublicEnv()): ResolvedNimiqAuthNetwork {
  const config = resolveNimiqAuthConfig(source);
  if (!config.ok) {
    throw Object.assign(new Error(config.message), { code: config.code });
  }
  return config;
}

export function nimiqNetworkLabel(source: EnvSource = readPublicEnv()): ResolvedNimiqAuthNetwork["displayName"] | "" {
  const config = resolveNimiqAuthConfig(source);
  return config.ok ? config.displayName : "";
}
