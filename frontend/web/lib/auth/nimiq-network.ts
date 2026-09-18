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
};

const MAINNET: ResolvedNimiqAuthNetwork = {
  ok: true,
  network: NIMIQ_AUTH_NETWORK_MAIN,
  environment: "mainnet",
  consensusName: "MainAlbatross",
  networkId: 24,
  hubEndpoint: NIMIQ_HUB_MAINNET,
  hubLabel: "Nimiq Hub",
};

export function isNimiqHubEnabled(source: EnvSource = process.env) {
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

export function resolveNimiqAuthConfig(source: EnvSource = process.env): NimiqAuthNetworkConfig {
  const configured = source.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK?.trim();
  if (!configured) {
    if (source.NODE_ENV === "production") {
      return {
        ok: false,
        code: "nimiq_network_unconfigured",
        message: "NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.",
      };
    }
    return TESTNET;
  }
  const parsed = parseNimiqAuthNetwork(configured);
  if (!parsed) {
    return {
      ok: false,
      code: "nimiq_network_unconfigured",
      message: "NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK must be test-albatross or main-albatross.",
    };
  }
  return parsed;
}

const current = resolveNimiqAuthConfig();

export const NIMIQ_AUTH_NETWORK = current.ok ? current.network : NIMIQ_AUTH_NETWORK_TEST;
export const NIMIQ_AUTH_ENVIRONMENT = current.ok ? current.environment : "testnet";
export const NIMIQ_HUB_ENDPOINT = current.ok ? current.hubEndpoint : NIMIQ_HUB_TESTNET;
