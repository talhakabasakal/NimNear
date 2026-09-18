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
