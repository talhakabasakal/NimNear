import assert from "node:assert/strict";
import test from "node:test";

import { AuthApiError, createNimiqChallenge } from "../../lib/api/auth";
import {
  isNimiqHubEnabled,
  NIMIQ_HUB_MAINNET,
  NIMIQ_HUB_TESTNET,
  parseNimiqAuthNetwork,
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
});

test("unknown network fails safely", () => {
  const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "ethereum" });
  assert.equal(config.ok, false);
  if (config.ok) return;
  assert.equal(config.code, "nimiq_network_unconfigured");
  assert.equal(parseNimiqAuthNetwork("polygon"), null);
});

test("production builds do not default to TestAlbatross when unset", () => {
  const config = resolveNimiqAuthConfig({ NODE_ENV: "production" });
  assert.equal(config.ok, false);
  if (config.ok) return;
  assert.equal(config.code, "nimiq_network_unconfigured");
});

test("development defaults remain TestAlbatross", () => {
  const config = resolveNimiqAuthConfig({ NODE_ENV: "development" });
  assert.equal(config.ok, true);
  if (!config.ok) return;
  assert.equal(config.network, "test-albatross");
  assert.equal(config.hubEndpoint, NIMIQ_HUB_TESTNET);
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

async function withPublicNetwork(value: string, run: () => Promise<void>) {
  const previous = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = value;
  try {
    await run();
  } finally {
    if (previous == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previous;
  }
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

