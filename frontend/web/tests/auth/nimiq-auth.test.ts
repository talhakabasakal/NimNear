import assert from "node:assert/strict";
import test from "node:test";

import {
  bytesToHex,
  NimiqAuthError,
  normalizeHex,
  selectNimiqAuthTransport,
} from "../../lib/auth/nimiq";
import {
  NIMIQ_HUB_MAINNET,
  NIMIQ_HUB_TESTNET,
  resolveNimiqAuthConfig,
} from "../../lib/auth/nimiq-network";

test("Mini App host signals select the Mini App adapter", () => {
  assert.equal(selectNimiqAuthTransport({ hasNimiqPay: true, hasNimiqProvider: false }), "mini-app");
  assert.equal(selectNimiqAuthTransport({ hasNimiqPay: false, hasNimiqProvider: true }), "mini-app");
  assert.equal(selectNimiqAuthTransport({ hasNimiqPay: false, hasNimiqProvider: false, hostLanguage: "tr" }), "mini-app");
});

test("development Hub defaults to Testnet and does not use mainnet constants", () => {
  assert.equal(selectNimiqAuthTransport({ hasNimiqPay: false, hasNimiqProvider: false }), "hub");
  const config = resolveNimiqAuthConfig({ NODE_ENV: "development" });
  assert.equal(config.ok, true);
  if (!config.ok) return;
  assert.equal(config.hubEndpoint, "https://hub.nimiq-testnet.com");
  assert.equal(config.network, "test-albatross");
  assert.equal(config.environment, "testnet");
  assert.equal(config.hubEndpoint.includes("https://hub.nimiq.com"), false);
});

test("Hub bytes and Mini App hex normalize at one boundary", () => {
  assert.equal(bytesToHex(new Uint8Array([0, 1, 15, 16, 255])), "00010f10ff");
  assert.equal(normalizeHex(`0x${"AB".repeat(32)}`, 32, "public key"), "ab".repeat(32));
  assert.throws(() => normalizeHex("not-hex", 64, "signature"), (error) => error instanceof NimiqAuthError && error.code === "signature_representation_error");
});

import type { NimiqProvider } from "@nimiq/mini-app-sdk";
import HubApi from "@nimiq/hub-api";
import {
  authenticateMiniApp,
  beginHubAuthentication,
  clearPendingHubAuthentication,
  completeHubAuthentication,
  createHubChallenge,
  discardStaleHubAuthState,
  finishHubSignature,
  hasHubUiIntent,
  hubRedirectBehavior,
  hubAuthFailureDiagnostics,
  NIMIQ_HUB_PUBLIC_METHODS,
  nimiqHubReturnUrl,
  persistPendingHubAuthentication,
  persistSelectedHubAddress,
  readPendingHubAuthentication,
  requestMiniAppWallet,
  resetHubRedirectConsumption,
  takeHubRedirectResult,
  toHex,
  type PendingHubAuthentication,
} from "../../lib/auth/nimiq";
import type { AuthSession, NimiqChallenge } from "../../lib/api/auth";
import { clearAuthSession, readAuthSession, writeAuthSession } from "../../lib/api/auth";
import { withAuthSession } from "../../lib/api/session-request";

process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK ??= "test-albatross";

const address = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const challenge: NimiqChallenge = { challenge_id: "11111111-1111-1111-1111-111111111111", message: "exact\ncanonical\nmessage", wallet_address: address, network: "test-albatross", environment: "testnet", purpose: "AUTH_LOGIN", transport: "mini-app", issued_at: "2026-09-17T12:00:00Z", expires_at: "2026-09-17T12:05:00Z" };
const session: AuthSession = { token: "session", user: { id: "user", email: "", first_name: "", last_name: "", status: "active", created_at: "2026-09-17T12:00:00Z" } };

test("Mini App initializes, lists accounts, signs the exact canonical message, and submits normalized proof", async () => {
  const calls: string[] = [];
  const provider = {
    listAccounts: async () => { calls.push("listAccounts"); return [address]; },
    sign: async (message: string) => { calls.push(`sign:${message}`); return { publicKey: "AA".repeat(32), signature: "BB".repeat(64) }; },
  } as unknown as NimiqProvider;
  const wallet = await requestMiniAppWallet(async () => { calls.push("init"); return provider; });
  let verifyInput: unknown;
  const result = await authenticateMiniApp(wallet, address, {
    createChallenge: async (selected, transport) => { assert.equal(selected, address); assert.equal(transport, "mini-app"); return challenge; },
    verifyChallenge: async (input) => { verifyInput = input; return session; },
  });
  assert.deepEqual(calls, ["init", "listAccounts", `sign:${challenge.message}`]);
  assert.deepEqual(verifyInput, { challenge_id: challenge.challenge_id, message: challenge.message, public_key: "aa".repeat(32), signature: "bb".repeat(64) });
  assert.equal(result, session);
});

test("Mini App empty accounts and permission rejection are user-facing outcomes", async () => {
  const empty = { listAccounts: async () => [] } as unknown as NimiqProvider;
  await assert.rejects(() => requestMiniAppWallet(async () => empty), (error) => error instanceof NimiqAuthError && error.code === "empty_account_list");
  const denied = { listAccounts: async () => ({ error: { type: "PermissionDeniedError", message: "denied" } }) } as unknown as NimiqProvider;
  await assert.rejects(() => requestMiniAppWallet(async () => denied), (error) => error instanceof NimiqAuthError && error.cancelled);
});

test("Hub phases keep address selection and signing as separate calls", async () => {
  let chose = false;
  const pending = await beginHubAuthentication({ chooseAddress: async () => { chose = true; return { address, label: "Test", meta: { account: { label: "Test", color: "blue" } } }; } } as never, async (selected, transport) => ({ ...challenge, wallet_address: selected, transport }));
  assert.equal(chose, true);
  assert.ok(pending);
  assert.equal(pending.address, address);
  let signedMessage = "";
  const hubProof = { signer: address, signerPublicKey: new Uint8Array(32).fill(1), signature: new Uint8Array(64).fill(2) };
  const verify = async (input: { message: string }) => { signedMessage = input.message; return session; };
  const result = await completeHubAuthentication(pending as PendingHubAuthentication, { signMessage: async (request: { message: string }) => { assert.equal(request.message, challenge.message); return hubProof; } } as never, verify as never);
  assert.equal(signedMessage, challenge.message);
  assert.equal(result, session);
});

test("Hub cancellation is handled as a normal user outcome", async () => {
  await assert.rejects(
    () => beginHubAuthentication({ chooseAddress: async () => { throw new Error("Request was cancelled"); } } as never, async () => challenge),
    (error) => error instanceof NimiqAuthError && error.cancelled && error.code === "wallet_cancelled",
  );
});

test("Mini App signing rejection never reaches backend verification", async () => {
  let verified = false;
  const wallet = {
    transport: "mini-app" as const,
    accounts: [address],
    provider: { sign: async () => ({ error: { type: "PermissionDeniedError", message: "User denied" } }) } as unknown as NimiqProvider,
  };
  await assert.rejects(
    () => authenticateMiniApp(wallet, address, { createChallenge: async () => challenge, verifyChallenge: async () => { verified = true; return session; } }),
    (error) => error instanceof NimiqAuthError && error.cancelled,
  );
  assert.equal(verified, false);
});

test("challenge retrieval failure stops before wallet signing", async () => {
  let signed = false;
  const wallet = {
    transport: "mini-app" as const,
    accounts: [address],
    provider: { sign: async () => { signed = true; return { publicKey: "aa".repeat(32), signature: "bb".repeat(64) }; } } as unknown as NimiqProvider,
  };
  await assert.rejects(() => authenticateMiniApp(wallet, address, { createChallenge: async () => { throw new Error("backend unavailable"); }, verifyChallenge: async () => session }));
  assert.equal(signed, false);
});


test("protected requests use the cookie when no bearer token is available", () => {
  const cookieOnly = withAuthSession("", { method: "POST", headers: { "Content-Type": "application/json" } });
  const cookieHeaders = new Headers(cookieOnly.headers);
  assert.equal(cookieOnly.credentials, "include");
  assert.equal(cookieHeaders.has("Authorization"), false);
  assert.equal(cookieHeaders.get("Content-Type"), "application/json");

  const bearer = withAuthSession("jwt-token", { cache: "no-store" });
  assert.equal(new Headers(bearer.headers).get("Authorization"), "Bearer jwt-token");
  assert.equal(bearer.credentials, "include");
});

test("Nimiq frontend session cache never stores a JWT", () => {
  const previous = globalThis.window;
  globalThis.window = { sessionStorage: memoryStorage(), dispatchEvent() {} } as never;
  try {
    writeAuthSession({ token: "secret-jwt", user: session.user });
    const stored = JSON.parse(window.sessionStorage.getItem("nimnear.auth.session") ?? "{}") as { token?: string; user?: unknown };
    assert.equal(stored.token, undefined);
    assert.deepEqual(readAuthSession(), { user: session.user });
    assert.equal(readAuthSession()?.token, undefined);
    clearAuthSession();
    assert.equal(readAuthSession(), null);
  } finally {
    globalThis.window = previous;
  }
});

function memoryStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value); },
    removeItem: (key: string) => { data.delete(key); },
  };
}

test("development Hub defaults never fall back to mainnet and uses public methods only", () => {
  const config = resolveNimiqAuthConfig({ NODE_ENV: "development" });
  assert.equal(config.ok, true);
  if (!config.ok) return;
  assert.equal(config.hubEndpoint.includes("https://hub.nimiq.com"), false);
  assert.equal(config.hubEndpoint, NIMIQ_HUB_TESTNET);
  assert.deepEqual([...NIMIQ_HUB_PUBLIC_METHODS], ["chooseAddress", "signMessage"]);
});

test("browser Hub uses redirect behavior so Hub receives the request in the URL", async () => {
  const previous = globalThis.window;
  globalThis.window = {
    location: { origin: "http://localhost:3000", pathname: "/profile", search: "" },
    sessionStorage: memoryStorage(),
  } as never;
  let captured: unknown;
  try {
    const pending = await beginHubAuthentication({
      chooseAddress: async (request, behavior) => {
        assert.equal(request.appName, "NIMNear");
        captured = behavior;
        return { address };
      },
    }, async (selected, transport) => ({ ...challenge, wallet_address: selected, transport }));
    assert.ok(pending);
    assert.equal(captured instanceof HubApi.RedirectRequestBehavior, true);
    assert.equal(hubRedirectBehavior({ phase: "choose-address" }) instanceof HubApi.RedirectRequestBehavior, true);
  } finally {
    globalThis.window = previous;
  }
});

test("pending Hub challenge survives the redirect return", () => {
  const previous = globalThis.window;
  globalThis.window = { sessionStorage: memoryStorage() } as never;
  const previousNetwork = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = "test-albatross";
  try {
    const pending = { transport: "hub" as const, address, challenge: { ...challenge, transport: "hub" as const } };
    persistPendingHubAuthentication(pending);
    assert.equal(hasHubUiIntent(), true);
    assert.deepEqual(readPendingHubAuthentication(), pending);
    clearPendingHubAuthentication();
    assert.equal(readPendingHubAuthentication(), null);
  } finally {
    if (previousNetwork == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previousNetwork;
    globalThis.window = previous;
  }
});

test("stale Testnet Hub pending state is discarded after a Mainnet deploy", () => {
  const previous = globalThis.window;
  const storage = memoryStorage();
  globalThis.window = { sessionStorage: storage } as never;
  const previousNetwork = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  try {
    process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = "test-albatross";
    persistPendingHubAuthentication({ transport: "hub", address, challenge: { ...challenge, transport: "hub" } });
    persistSelectedHubAddress(address, "Teal Address");
    storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
    process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = "main-albatross";
    discardStaleHubAuthState();
    assert.equal(readPendingHubAuthentication(), null);
    assert.equal(storage.getItem("nimnear.auth.hub.pending"), null);
    assert.equal(storage.getItem("nimnear.auth.hub.selectedAddress"), null);
    assert.equal(storage.getItem("rpcRequests"), null);
  } finally {
    if (previousNetwork == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previousNetwork;
    globalThis.window = previous;
  }
});

test("Hub redirect results restore address selection and the signed canonical message", async () => {
  resetHubRedirectConsumption();
  const handlers = new Map<string, { resolve: (result: unknown, state: unknown) => void; reject: (error: Error, state: unknown) => void }>();
  const client = {
    on(command: string, resolve: (result: unknown, state: unknown) => void, reject: (error: Error, state: unknown) => void) {
      handlers.set(command, { resolve, reject });
    },
    async checkRedirectResponse() {
      handlers.get("choose-address")?.resolve({ address }, {});
    },
  };
  const redirected = await takeHubRedirectResult(client as never);
  assert.deepEqual(redirected, { type: "address", address });
  resetHubRedirectConsumption();
  const signed = { signer: address, signerPublicKey: new Uint8Array(32).fill(1), signature: new Uint8Array(64).fill(2) };
  const signClient = {
    on(command: string, resolve: (result: unknown, state: unknown) => void, reject: (error: Error, state: unknown) => void) {
      handlers.set(command, { resolve, reject });
    },
    async checkRedirectResponse() {
      handlers.get("sign-message")?.resolve(signed, {});
    },
  };
  const signatureRedirect = await takeHubRedirectResult(signClient as never);
  assert.equal(signatureRedirect.type, "signature");
  if (signatureRedirect.type === "signature") {
    let signedMessage = "";
    const pending = await createHubChallenge(address, async (selected, transport) => ({ ...challenge, wallet_address: selected, transport }));
    const result = await finishHubSignature(pending, signatureRedirect.signed, (async (input: { message: string }) => { signedMessage = input.message; return session; }) as never);
    assert.equal(signedMessage, challenge.message);
    assert.equal(result, session);
  }
  resetHubRedirectConsumption();
});

test("Hub address return is recovered when HTTPS Hub leaves HTTP referrer empty", async () => {
  resetHubRedirectConsumption();
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  const storage = memoryStorage();
  storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  const result = encodeURIComponent(JSON.stringify({ address, label: "Teal Address", meta: { account: { label: "Teal Address", color: "teal" } } }));
  let href = `http://localhost:3000/profile#id=42&status=ok&result=${result}`;
  const location = {
    origin: "http://localhost:3000",
    pathname: "/profile",
    search: "",
    protocol: "http:",
    hostname: "localhost",
    get hash() { return new URL(href).hash; },
    get href() { return href; },
    set href(value: string) { href = value; },
  };
  globalThis.document = { referrer: "" } as never;
  globalThis.history = {
    state: null,
    replaceState(_state: unknown, _title: string, url: string) {
      href = new URL(url, "http://localhost:3000").href;
    },
  } as never;
  globalThis.window = { location, sessionStorage: storage, history: globalThis.history } as never;
  try {
    const redirected = await takeHubRedirectResult({
      on() { throw new Error("HubApi handlers are not required when the hash already contains the result"); },
      async checkRedirectResponse() { throw new Error("HubApi must not depend on document.referrer for HTTP returns"); },
    } as never);
    assert.equal(redirected.type, "address");
    if (redirected.type === "address") {
      assert.equal(redirected.address, address);
      assert.equal(redirected.label, "Teal Address");
    }
    assert.equal(storage.getItem("nimnear.auth.hub.selectedAddress"), address);
    assert.equal(storage.getItem("nimnear.auth.hub.accountLabel"), "Teal Address");
  } finally {
    resetHubRedirectConsumption();
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("Hub signature verification submits the Hub account label", async () => {
  resetHubRedirectConsumption();
  const previous = globalThis.window;
  globalThis.window = { sessionStorage: memoryStorage() } as never;
  try {
    let submitted: { account_label?: string } | undefined;
    const pending = {
      transport: "hub" as const,
      address,
      accountLabel: "Teal Address",
      challenge: { ...challenge, transport: "hub" as const },
    };
    const signed = { signer: address, signerPublicKey: new Uint8Array(32).fill(1), signature: new Uint8Array(64).fill(2) };
    const result = await finishHubSignature(pending, signed, (async (input: { account_label?: string }) => {
      submitted = input;
      return { ...session, user: { ...session.user, display_name: "Teal Address", wallet_address: address } };
    }) as never);
    assert.equal(submitted?.account_label, "Teal Address");
    assert.equal(result.user.display_name, "Teal Address");
    assert.equal(result.user.wallet_address, address);
  } finally {
    resetHubRedirectConsumption();
    globalThis.window = previous;
  }
});

test("Hub redirect cancellation is a normal user outcome", async () => {
  resetHubRedirectConsumption();
  const handlers = new Map<string, { reject: (error: Error, state: unknown) => void }>();
  const client = {
    on(_command: string, _resolve: unknown, reject: (error: Error, state: unknown) => void) {
      handlers.set(_command, { reject });
    },
    async checkRedirectResponse() {
      handlers.get("choose-address")?.reject(new Error("Request was cancelled"), {});
    },
  };
  const redirected = await takeHubRedirectResult(client as never);
  assert.equal(redirected.type, "error");
  if (redirected.type === "error") {
    assert.equal(redirected.stage, "requesting-wallet");
    assert.equal(redirected.error instanceof NimiqAuthError && redirected.error.cancelled, true);
  }
  resetHubRedirectConsumption();
});

test("toHex normalizes Hub byte arrays and Mini App hex at one boundary", () => {
  assert.equal(toHex(new Uint8Array(32).fill(10), 32, "public key"), "0a".repeat(32));
  assert.equal(toHex("BB".repeat(64), 64, "signature"), "bb".repeat(64));
  assert.throws(() => toHex("zz", 32, "public key"), (error) => error instanceof NimiqAuthError && error.code === "signature_representation_error");
});

test("Hub redirect invocation encodes choose-address on the Testnet Hub URL", async () => {
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  let href = "http://localhost:3000/profile";
  globalThis.document = { referrer: "" } as never;
  globalThis.history = { state: null, replaceState() {} } as never;
  globalThis.window = {
    location: {
      origin: "http://localhost:3000",
      pathname: "/profile",
      search: "",
      hash: "",
      protocol: "http:",
      hostname: "localhost",
    },
    sessionStorage: memoryStorage(),
    history: globalThis.history,
    addEventListener() {},
    removeEventListener() {},
  } as never;
  Object.defineProperty(globalThis.window.location, "href", {
    get() { return href; },
    set(value: string) { href = value; },
    configurable: true,
  });
  try {
    const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "test-albatross" });
    assert.equal(config.ok, true);
    if (!config.ok) return;
    const hub = new HubApi(config.hubEndpoint);
    const navigation = hub.chooseAddress(
      { appName: "NIMNear" },
      new HubApi.RedirectRequestBehavior("http://localhost:3000/profile", { phase: "choose-address" }) as never,
    );
    await Promise.race([navigation, new Promise((resolve) => setTimeout(resolve, 50))]);
    const redirected = new URL(href);
    assert.equal(redirected.origin, "https://hub.nimiq-testnet.com");
    assert.equal(redirected.hash.includes("command=choose-address"), true);
    assert.match(decodeURIComponent(redirected.hash), /NIMNear/);
    assert.equal(href.startsWith("https://hub.nimiq.com"), false);
  } finally {
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("Hub redirect invocation encodes choose-address on the Mainnet Hub URL", async () => {
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  let href = "http://localhost:3000/profile";
  globalThis.document = { referrer: "" } as never;
  globalThis.history = { state: null, replaceState() {} } as never;
  globalThis.window = {
    location: {
      origin: "http://localhost:3000",
      pathname: "/profile",
      search: "",
      hash: "",
      protocol: "http:",
      hostname: "localhost",
    },
    sessionStorage: memoryStorage(),
    history: globalThis.history,
    addEventListener() {},
    removeEventListener() {},
  } as never;
  Object.defineProperty(globalThis.window.location, "href", {
    get() { return href; },
    set(value: string) { href = value; },
    configurable: true,
  });
  try {
    const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "main-albatross" });
    assert.equal(config.ok, true);
    if (!config.ok) return;
    assert.equal(config.hubEndpoint, NIMIQ_HUB_MAINNET);
    const hub = new HubApi(config.hubEndpoint);
    const navigation = hub.chooseAddress(
      { appName: "NIMNear" },
      new HubApi.RedirectRequestBehavior("http://localhost:3000/profile", { phase: "choose-address" }) as never,
    );
    await Promise.race([navigation, new Promise((resolve) => setTimeout(resolve, 50))]);
    const redirected = new URL(href);
    assert.equal(redirected.origin, "https://hub.nimiq.com");
    assert.equal(redirected.hash.includes("command=choose-address"), true);
    assert.match(decodeURIComponent(redirected.hash), /NIMNear/);
    assert.equal(href.startsWith("https://hub.nimiq-testnet.com"), false);
  } finally {
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("Hub return URLs follow the Events root and related routes without secrets", () => {
  const previous = globalThis.window;
  const origin = "https://nim-near.vercel.app";
  try {
    for (const [pathname, search] of [
      ["/", "?view=upcoming"],
      ["/events", ""],
      ["/events/abc", ""],
      ["/wallet", ""],
    ] as const) {
      globalThis.window = { location: { origin, pathname, search } } as never;
      const url = nimiqHubReturnUrl();
      const parsed = new URL(url);
      assert.equal(parsed.origin, origin);
      assert.equal(parsed.pathname, pathname);
      assert.equal(parsed.search, search);
      assert.equal(url.includes("token="), false);
      assert.equal(url.includes("jwt="), false);
    }
  } finally {
    globalThis.window = previous;
  }
});

test("Hub choose-address callback is recovered from the Events root", async () => {
  resetHubRedirectConsumption();
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  const storage = memoryStorage();
  storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  const result = encodeURIComponent(JSON.stringify({ address, label: "Events Address" }));
  let href = `https://nim-near.vercel.app/#id=42&status=ok&result=${result}`;
  const location = {
    origin: "https://nim-near.vercel.app",
    pathname: "/",
    search: "",
    protocol: "https:",
    hostname: "nim-near.vercel.app",
    get hash() { return new URL(href).hash; },
    get href() { return href; },
    set href(value: string) { href = value; },
  };
  globalThis.document = { referrer: "" } as never;
  globalThis.history = {
    state: null,
    replaceState(_state: unknown, _title: string, url: string) {
      href = new URL(url, "https://nim-near.vercel.app").href;
    },
  } as never;
  globalThis.window = { location, sessionStorage: storage, history: globalThis.history } as never;
  try {
    const redirected = await takeHubRedirectResult({
      on() { throw new Error("HubApi handlers are not required when the hash already contains the result"); },
      async checkRedirectResponse() { throw new Error("HubApi must not depend on document.referrer for HTTP returns"); },
    } as never);
    assert.equal(redirected.type, "address");
    if (redirected.type === "address") {
      assert.equal(redirected.address, address);
      assert.equal(redirected.label, "Events Address");
    }
    assert.equal(storage.getItem("nimnear.auth.hub.selectedAddress"), address);
  } finally {
    resetHubRedirectConsumption();
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("Hub initiation from the Events root encodes Mainnet choose-address and returns to /", async () => {
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  let href = "https://nim-near.vercel.app/?view=upcoming";
  globalThis.document = { referrer: "" } as never;
  globalThis.history = { state: { __NA: true, __PRIVATE_NEXTJS_INTERNALS_TREE: { circular: null } }, replaceState() { throw new Error("DataCloneError"); } } as never;
  globalThis.window = {
    location: {
      origin: "https://nim-near.vercel.app",
      pathname: "/",
      search: "?view=upcoming",
      hash: "",
      protocol: "https:",
      hostname: "nim-near.vercel.app",
    },
    sessionStorage: memoryStorage(),
    history: globalThis.history,
    addEventListener() {},
    removeEventListener() {},
  } as never;
  Object.defineProperty(globalThis.window.location, "href", {
    get() { return href; },
    set(value: string) { href = value; },
    configurable: true,
  });
  const previousNetwork = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = "main-albatross";
  try {
    const config = resolveNimiqAuthConfig({ NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK: "main-albatross" });
    assert.equal(config.ok, true);
    if (!config.ok) return;
    const hub = new HubApi(config.hubEndpoint);
    const navigation = hub.chooseAddress({ appName: "NIMNear" }, hubRedirectBehavior({ phase: "choose-address" }) as never);
    await Promise.race([navigation, new Promise((resolve) => setTimeout(resolve, 50))]);
    const redirected = new URL(href);
    assert.equal(redirected.origin, NIMIQ_HUB_MAINNET);
    assert.equal(redirected.hash.includes("command=choose-address"), true);
    assert.match(decodeURIComponent(redirected.hash), /returnURL=https:\/\/nim-near\.vercel\.app\/\?view=upcoming/);
    assert.equal(href.startsWith("https://hub.nimiq-testnet.com"), false);
  } finally {
    if (previousNetwork == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previousNetwork;
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("Hub back-navigation is cancelled instead of reported as unreachable", async () => {
  await assert.rejects(
    () => beginHubAuthentication({ chooseAddress: async () => { throw new Error("Request aborted"); } } as never, async () => challenge),
    (error) => error instanceof NimiqAuthError && error.cancelled && error.code === "wallet_cancelled" && error.cause instanceof Error,
  );
});

test("unknown Hub failures keep a safe UI message and structured diagnostics", async () => {
  const previous = globalThis.window;
  globalThis.window = {
    location: { origin: "https://nim-near.vercel.app", pathname: "/", search: "", href: "https://nim-near.vercel.app/", hash: "" },
    sessionStorage: memoryStorage(),
  } as never;
  try {
    const original = new Error("ECONNREFUSED hub.nimiq.com");
    await assert.rejects(
      () => beginHubAuthentication({ chooseAddress: async () => { throw original; } } as never, async () => challenge),
      (error) => {
        if (!(error instanceof NimiqAuthError)) return false;
        assert.equal(error.code, "wallet_unavailable");
        assert.equal(error.message, "The Nimiq wallet could not be reached.");
        assert.equal(error.cause, original);
        const diagnostics = hubAuthFailureDiagnostics(error.stage, original, "requesting-wallet");
        assert.equal(diagnostics.errorName, "Error");
        assert.equal(diagnostics.errorMessage, "ECONNREFUSED hub.nimiq.com");
        assert.equal(diagnostics.requestPhase, "requesting-wallet");
        assert.equal(diagnostics.hubEndpoint, "https://hub.nimiq-testnet.com");
        assert.equal(diagnostics.pathname, "/");
        assert.equal(diagnostics.returnUrl, "https://nim-near.vercel.app/");
        assert.equal(diagnostics.returnUrl.includes("token="), false);
        return true;
      },
    );
  } finally {
    globalThis.window = previous;
  }
});

