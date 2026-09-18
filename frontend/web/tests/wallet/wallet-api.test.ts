import assert from "node:assert/strict";
import test from "node:test";

import { withAuthSession } from "../../lib/api/session-request";
import {
  getWalletBalance,
  getWalletTransactions,
  WalletApiError,
} from "../../lib/api/wallet";

test("wallet API requests send credentials cookies and do not persist JWTs", async () => {
  const previousFetch = globalThis.fetch;
  const previousWindow = globalThis.window;
  const storage = new Map<string, string>();
  const calls: Array<{ url: string; init?: RequestInit }> = [];

  globalThis.window = {
    sessionStorage: {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => {
        storage.set(key, value);
      },
      removeItem: (key: string) => {
        storage.delete(key);
      },
    },
    localStorage: {
      getItem: (key: string) => storage.get("local:" + key) ?? null,
      setItem: (key: string, value: string) => {
        storage.set("local:" + key, value);
      },
      removeItem: (key: string) => {
        storage.delete("local:" + key);
      },
    },
  } as never;

  globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(
      JSON.stringify({
        data: {
          address: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
          balance_lunas: "250000",
          balance_nim: "2.5",
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const balance = await getWalletBalance();
    assert.equal(balance.balance_lunas, "250000");
    assert.equal(calls[0]?.init?.credentials, "include");
    assert.equal(new Headers(calls[0]?.init?.headers).has("Authorization"), false);

    globalThis.fetch = (async (input: URL | RequestInfo, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      return new Response(
        JSON.stringify({
          data: [
            {
              hash: "ab".repeat(32),
              sender: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
              recipient: "NQ07 0000 0000 0000 0000 0000 0000 0000 0000",
              value_lunas: "250000",
              value_nim: "2.5",
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const history = await getWalletTransactions({ max: 10 });
    assert.equal(history.data[0]?.value_lunas, "250000");
    assert.equal(calls[1]?.init?.credentials, "include");
    assert.ok(String(calls[1]?.url).includes("/api/v1/wallet/transactions"));
    assert.ok(String(calls[1]?.url).includes("max=10"));

    for (const [key, value] of storage.entries()) {
      assert.equal(value.includes("eyJ"), false, key + " stored a JWT-like value");
      assert.equal(value.toLowerCase().includes("token"), false, key + " stored a token");
    }
    assert.equal(storage.size, 0);
  } finally {
    globalThis.fetch = previousFetch;
    globalThis.window = previousWindow;
  }
});

test("wallet API uses the cookie session helper without requiring a stored JWT", () => {
  const request = withAuthSession(undefined, { cache: "no-store" });
  assert.equal(request.credentials, "include");
  assert.equal(new Headers(request.headers).has("Authorization"), false);
});

test("wallet API surfaces backend error codes", async () => {
  const previousFetch = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(
      JSON.stringify({
        error: "Forbidden",
        error_code: "wallet_identity_forbidden",
        message: "wallet address is not a verified identity of this user",
      }),
      { status: 403, headers: { "Content-Type": "application/json" } },
    )) as typeof fetch;
  try {
    await assert.rejects(
      () => getWalletBalance(),
      (error: unknown) =>
        error instanceof WalletApiError &&
        error.status === 403 &&
        error.errorCode === "wallet_identity_forbidden",
    );
  } finally {
    globalThis.fetch = previousFetch;
  }
});

test("wallet API surfaces unauthorized and RPC unavailable responses", async () => {
  const previousFetch = globalThis.fetch;
  try {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ error: "missing authentication session" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      })) as typeof fetch;
    await assert.rejects(
      () => getWalletBalance(),
      (error: unknown) => error instanceof WalletApiError && error.status === 401,
    );

    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          error: "Internal Server Error",
          error_code: "nimiq_rpc_unavailable",
          message: "an internal error occurred",
        }),
        { status: 500, headers: { "Content-Type": "application/json" } },
      )) as typeof fetch;
    await assert.rejects(
      () => getWalletTransactions(),
      (error: unknown) =>
        error instanceof WalletApiError &&
        error.status === 500 &&
        error.errorCode === "nimiq_rpc_unavailable",
    );
  } finally {
    globalThis.fetch = previousFetch;
  }
});
