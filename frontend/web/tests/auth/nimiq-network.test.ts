import assert from "node:assert/strict";
import test from "node:test";

import { AuthApiError, createNimiqChallenge } from "../../lib/api/auth";
import {
  isAllowedNimiqAuthNetwork,
  isNimiqHubEnabled,
  NIMIQ_HUB_MAINNET,
  NIMIQ_HUB_TESTNET,
  nimiqNetworkLabel,
  parseNimiqAuthNetwork,
  publicNimiqAuthIdentity,
  requireNimiqAuthConfig,
  resolveNimiqAuthConfig,
} from "../../lib/auth/nimiq-network";

const address = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";

test("configured testnet maps to testnet challenge network and Hub", () => {
  const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "test-albatross" });
  assert.equal(config.ok, true);
  if (!config.ok) return;
  assert.equal(config.network, "test-albatross");
  assert.equal(config.environment, "testnet");
  assert.equal(config.consensusName, "TestAlbatross");
  assert.equal(config.networkId, 5);
  assert.equal(config.hubEndpoint, NIMIQ_HUB_TESTNET);
  assert.equal(config.hubEndpoint, "https://hub.nimiq-testnet.com");
  assert.equal(config.hubLabel, "Nimiq Testnet Hub");
  assert.equal(config.displayName, "Nimiq Testnet");
  assert.equal(nimiqNetworkLabel({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "test-albatross" }), "Nimiq Testnet");
});

test("configured mainnet maps to mainnet challenge network and Hub", () => {
  const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "main-albatross" });
  assert.equal(config.ok, true);
  if (!config.ok) return;
  assert.equal(config.network, "main-albatross");
  assert.equal(config.environment, "mainnet");
  assert.equal(config.consensusName, "MainAlbatross");
  assert.equal(config.networkId, 24);
  assert.equal(config.hubEndpoint, NIMIQ_HUB_MAINNET);
  assert.equal(config.hubEndpoint, "https://hub.nimiq.com");
  assert.equal(config.hubEndpoint.includes("hub.nimiq-testnet.com"), false);
  assert.equal(config.hubLabel, "Nimiq Hub");
  assert.equal(config.displayName, "Nimiq Mainnet");
  assert.equal(nimiqNetworkLabel({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "main-albatross" }), "Nimiq Mainnet");
});

test("unknown network fails safely", () => {
  const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "ethereum" });
  assert.equal(config.ok, false);
  if (config.ok) return;
  assert.equal(config.code, "nimiq_network_unconfigured");
  assert.equal(parseNimiqAuthNetwork("polygon"), null);
  assert.equal(isAllowedNimiqAuthNetwork("ethereum"), false);
  assert.equal(isAllowedNimiqAuthNetwork("main-albatross"), true);
  assert.equal(isAllowedNimiqAuthNetwork("test-albatross"), true);
  assert.equal(nimiqNetworkLabel({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "ethereum" }), "");
  assert.throws(
    () => requireNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "ethereum" }),
    (error: unknown) => error instanceof Error && error.message.includes("test-albatross or main-albatross"),
  );
});

test("production builds do not default to TestAlbatross when unset", () => {
  const config = resolveNimiqAuthConfig({ NODE_ENV: "production" });
  assert.equal(config.ok, false);
  if (config.ok) return;
  assert.equal(config.code, "nimiq_network_unconfigured");
  assert.equal(nimiqNetworkLabel({ NODE_ENV: "production" }), "");
});

test("non-development unset configuration does not fall back to Testnet", () => {
  const unset = resolveNimiqAuthConfig({ NODE_ENV: undefined });
  assert.equal(unset.ok, false);
  const testEnv = resolveNimiqAuthConfig({ NODE_ENV: "test" });
  assert.equal(testEnv.ok, false);
});

test("production invalid configuration does not fall back to Testnet", () => {
  const config = resolveNimiqAuthConfig({
    NODE_ENV: "production",
    NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "not-a-network",
  });
  assert.equal(config.ok, false);
  if (config.ok) return;
  assert.equal(config.code, "nimiq_network_unconfigured");
});

test("development defaults remain TestAlbatross", () => {
  const config = resolveNimiqAuthConfig({ NODE_ENV: "development" });
  assert.equal(config.ok, true);
  if (!config.ok) return;
  assert.equal(config.network, "test-albatross");
  assert.equal(config.environment, "testnet");
  assert.equal(config.hubEndpoint, NIMIQ_HUB_TESTNET);
  assert.equal(config.displayName, "Nimiq Testnet");
});

test("createNimiqChallenge sends the configured testnet", async () => {
  await withPublicNetwork("test-albatross", async () => {
    const body = await captureChallengeBody();
    assert.equal(body.network, "test-albatross");
    assert.equal(body.environment, "testnet");
    assert.equal(body.purpose, "AUTH_LOGIN");
  });
});

test("createNimiqChallenge sends the configured mainnet", async () => {
  await withPublicNetwork("main-albatross", async () => {
    const body = await captureChallengeBody();
    assert.equal(body.network, "main-albatross");
    assert.equal(body.environment, "mainnet");
  });
});

test("createNimiqChallenge fails closed on unknown network without calling Hub", async () => {
  await withPublicNetwork("ethereum", async () => {
    let fetched = false;
    const previousFetch = globalThis.fetch;
    globalThis.fetch = (async () => {
      fetched = true;
      return new Response("{}", { status: 500 });
    }) as typeof fetch;
    try {
      await assert.rejects(
        () => createNimiqChallenge(address, "hub"),
        (error: unknown) => error instanceof AuthApiError && error.code === "nimiq_network_unconfigured" && !fetched,
      );
    } finally {
      globalThis.fetch = previousFetch;
    }
  });
});

test("createNimiqChallenge surfaces backend network mismatch diagnostics", async () => {
  await withPublicNetwork("test-albatross", async () => {
    const previousFetch = globalThis.fetch;
    globalThis.fetch = (async () => new Response(JSON.stringify({
      error: "Bad Request",
      error_code: "unsupported_nimiq_network",
      message: "bad request: Nimiq network does not match this deployment (requested_network=test-albatross requested_environment=testnet expected_network=main-albatross expected_environment=mainnet)",
      details: {
        requested_network: "test-albatross",
        requested_environment: "testnet",
        expected_network: "main-albatross",
        expected_environment: "mainnet",
      },
    }), { status: 400, headers: { "Content-Type": "application/json" } })) as typeof fetch;
    try {
      await assert.rejects(
        () => createNimiqChallenge(address, "hub"),
        (error: unknown) => error instanceof AuthApiError
          && error.code === "unsupported_nimiq_network"
          && error.message.includes("requested_network=test-albatross")
          && error.message.includes("expected_network=main-albatross"),
      );
    } finally {
      globalThis.fetch = previousFetch;
    }
  });
});

test("createNimiqChallenge fails closed in production when the network is unset", async () => {
  await withPublicNetwork(undefined, async () => {
    const previousNodeEnv = process.env.NODE_ENV;
    setNodeEnv("production");
    let fetched = false;
    const previousFetch = globalThis.fetch;
    globalThis.fetch = (async () => {
      fetched = true;
      return new Response("{}", { status: 500 });
    }) as typeof fetch;
    try {
      const config = resolveNimiqAuthConfig();
      assert.equal(config.ok, false);
      await assert.rejects(
        () => createNimiqChallenge(address, "hub"),
        (error: unknown) => error instanceof AuthApiError && error.code === "nimiq_network_unconfigured" && !fetched,
      );
    } finally {
      globalThis.fetch = previousFetch;
      setNodeEnv(previousNodeEnv);
    }
  });
});

async function withPublicNetwork(value: string | undefined, run: () => Promise<void>) {
  const previous = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  if (value == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = value;
  try {
    await run();
  } finally {
    if (previous == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previous;
  }
}

function setNodeEnv(value: string | undefined) {
  const env = process.env as Record<string, string | undefined>;
  if (value == null) delete env.NODE_ENV;
  else env.NODE_ENV = value;
}

async function captureChallengeBody() {
  const previousFetch = globalThis.fetch;
  let body: { network?: string; environment?: string; purpose?: string } = {};
  globalThis.fetch = (async (_url, init) => {
    body = JSON.parse(String(init?.body)) as typeof body;
    return new Response(JSON.stringify({
      challenge_id: "11111111-1111-1111-1111-111111111111",
      message: "canonical",
      wallet_address: address,
      network: body.network,
      environment: body.environment,
      purpose: "AUTH_LOGIN",
      transport: "hub",
      issued_at: "2026-09-17T12:00:00Z",
      expires_at: "2026-09-17T12:05:00Z",
    }), { status: 201, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
  try {
    await createNimiqChallenge(address, "hub");
    return body;
  } finally {
    globalThis.fetch = previousFetch;
  }
}

test("Hub fallback can be disabled for Mini App-only production", () => {
  assert.equal(isNimiqHubEnabled({}), true);
  assert.equal(isNimiqHubEnabled({ NEXT_PUBLIC_NIMNEAR_HUB_ENABLED: "false" }), false);
  assert.equal(isNimiqHubEnabled({ NEXT_PUBLIC_NIMNEAR_HUB_ENABLED: "true" }), true);
});

test("public identity exposes only non-secret production network metadata", () => {
  const identity = publicNimiqAuthIdentity({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "main-albatross" });
  assert.equal(identity.configured, true);
  if (!identity.configured) return;
  assert.equal(identity.network, "main-albatross");
  assert.equal(identity.environment, "mainnet");
  assert.equal(identity.consensus, "MainAlbatross");
  assert.equal(identity.networkId, 24);
  assert.equal(identity.hub, NIMIQ_HUB_MAINNET);
});

test("next.config force-inlines the canonical network without importing client env reads", async () => {
  const { readFile } = await import("node:fs/promises");
  const config = await readFile(new URL("../../next.config.ts", import.meta.url), "utf8");
  assert.match(config, /NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: configuredNimiqNetwork\.network/);
  assert.match(config, /from "\.\/lib\/auth\/nimiq-albatross"/);
  assert.doesNotMatch(config, /nimiq-network|nimiq-public-env/);
});
