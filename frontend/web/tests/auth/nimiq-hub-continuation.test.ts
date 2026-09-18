import assert from "node:assert/strict";
import test from "node:test";

import {
  captureHubRpcCallbackFromWindow,
  clearPendingHubAuthentication,
  hubAuthFailureDiagnostics,
  hubCallbackConsumeCountForTests,
  NIMIQ_HUB_CALLBACK_STORAGE_KEY,
  NimiqAuthError,
  peekHubCallbackMeta,
  persistHubAuthContinuation,
  persistPendingHubAuthentication,
  persistSelectedHubAddress,
  readHubAuthContinuation,
  readPendingHubAuthentication,
  resetHubRedirectConsumption,
  resumeHubAuthFlow,
  takeHubRedirectResult,
  userMessageForAuthPhase,
  discardStaleHubAuthState,
  type PendingHubAuthentication,
} from "../../lib/auth/nimiq";
import type { AuthSession, NimiqChallenge } from "../../lib/api/auth";

process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK ??= "test-albatross";

const address = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const challenge: NimiqChallenge = {
  challenge_id: "11111111-1111-1111-1111-111111111111",
  message: "exact\ncanonical\nmessage",
  wallet_address: address,
  network: "test-albatross",
  environment: "testnet",
  purpose: "AUTH_LOGIN",
  transport: "hub",
  issued_at: "2026-09-17T12:00:00Z",
  expires_at: "2026-09-17T12:05:00Z",
};
const session: AuthSession = {
  user: { id: "user", email: "", first_name: "", last_name: "", status: "active", created_at: "2026-09-17T12:00:00Z" },
};
const signed = {
  signer: address,
  signerPublicKey: new Uint8Array(32).fill(1),
  signature: new Uint8Array(64).fill(2),
};

function memoryStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value); },
    removeItem: (key: string) => { data.delete(key); },
  };
}

function encodeBytes(bytes: Uint8Array) {
  return { __: 0, v: Buffer.from(bytes).toString("base64") };
}

function installWindow(options: {
  pathname: string;
  search?: string;
  origin?: string;
  href?: string;
  throwOnReplace?: boolean;
  storage?: ReturnType<typeof memoryStorage>;
}) {
  resetHubRedirectConsumption();
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  const storage = options.storage ?? memoryStorage();
  const origin = options.origin ?? "https://nim-near.vercel.app";
  let href = options.href ?? `${origin}${options.pathname}${options.search ?? ""}`;
  const circular: { self?: unknown } = {};
  circular.self = circular;
  const location = {
    origin,
    pathname: options.pathname,
    search: options.search ?? "",
    protocol: "https:",
    hostname: "nim-near.vercel.app",
    get hash() { return new URL(href).hash; },
    get href() { return href; },
    set href(value: string) { href = value; },
  };
  globalThis.document = { referrer: "" } as never;
  globalThis.history = {
    state: { __NA: true, __PRIVATE_NEXTJS_INTERNALS_TREE: circular },
    replaceState(_state: unknown, _title: string, url: string) {
      if (options.throwOnReplace) {
        const error = new Error("The object could not be cloned.");
        error.name = "DataCloneError";
        throw error;
      }
      href = new URL(url, origin).href;
    },
  } as never;
  globalThis.window = { location, sessionStorage: storage, history: globalThis.history } as never;
  return {
    storage,
    get href() { return href; },
    restore() {
      resetHubRedirectConsumption();
      globalThis.window = previousWindow;
      globalThis.document = previousDocument;
      globalThis.history = previousHistory;
    },
  };
}

function chooseAddressHash(pathname: string) {
  const result = encodeURIComponent(JSON.stringify({ address, label: "Teal Address" }));
  return `https://nim-near.vercel.app${pathname}#id=42&status=ok&result=${result}`;
}

function signatureHash(pathname: string) {
  const result = encodeURIComponent(JSON.stringify({
    signer: address,
    signerPublicKey: encodeBytes(signed.signerPublicKey),
    signature: encodeBytes(signed.signature),
  }));
  return `https://nim-near.vercel.app${pathname}#id=43&status=ok&result=${result}`;
}

const routes = ["/", "/events", "/events/abc", "/wallet"] as const;

test("chooseAddress callback is consumed exactly once across duplicate restore calls", async () => {
  const installed = installWindow({
    pathname: "/",
    href: chooseAddressHash("/"),
  });
  installed.storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  try {
    const first = await takeHubRedirectResult({
      on() { throw new Error("HubApi must not consume a captured callback"); },
      async checkRedirectResponse() { throw new Error("HubApi must not run after capture"); },
    } as never);
    const second = await takeHubRedirectResult({
      on() { throw new Error("second consumer"); },
      async checkRedirectResponse() { throw new Error("second consumer"); },
    } as never);
    assert.equal(first.type, "address");
    assert.equal(second.type, "address");
    assert.equal(hubCallbackConsumeCountForTests(), 1);
    assert.equal(installed.storage.getItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY), null);
  } finally {
    installed.restore();
  }
});

test("Next.js DataCloneError while clearing the hash still restores chooseAddress", async () => {
  const installed = installWindow({
    pathname: "/wallet",
    href: chooseAddressHash("/wallet"),
    throwOnReplace: true,
  });
  installed.storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  try {
    captureHubRpcCallbackFromWindow();
    const meta = peekHubCallbackMeta();
    assert.equal(meta.storagePresent, true);
    assert.equal(meta.operation, "choose-address");
    const redirected = await takeHubRedirectResult({
      on() { throw new Error("HubApi DataCloneError path must not be used"); },
      async checkRedirectResponse() {
        const error = new Error("The object could not be cloned.");
        error.name = "DataCloneError";
        throw error;
      },
    } as never);
    assert.equal(redirected.type, "address");
    if (redirected.type === "address") assert.equal(redirected.address, address);
  } finally {
    installed.restore();
  }
});

for (const pathname of routes) {
  test(`two-redirect Hub auth continues from ${pathname}`, async () => {
    const installed = installWindow({
      pathname,
      href: chooseAddressHash(pathname),
    });
    installed.storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
    const calls: string[] = [];
    try {
      const first = await resumeHubAuthFlow({
        restoreSession: async () => null,
        createChallenge: async (selected, transport) => {
          calls.push(`challenge:${transport}`);
          assert.equal(selected, address);
          return { ...challenge, wallet_address: selected, transport };
        },
        signClient: {
          signMessage: async () => {
            calls.push("signMessage");
            return undefined;
          },
        },
      });
      assert.equal(first.type, "redirecting");
      assert.equal(first.phase, "sign-challenge");
      assert.equal(readPendingHubAuthentication()?.address, address);
      assert.equal(readHubAuthContinuation()?.phase, "sign-challenge");
      assert.equal(readHubAuthContinuation()?.addressSelected, true);
      assert.equal(readHubAuthContinuation()?.returnPath.startsWith(pathname), true);

      resetHubRedirectConsumption();
      installed.storage.setItem("rpcRequests", JSON.stringify({ "43": ["sign-message", { phase: "sign-message" }] }));
      globalThis.window.location.href = signatureHash(pathname);

      const second = await resumeHubAuthFlow({
        restoreSession: async () => null,
        verifyChallenge: async (input) => {
          calls.push("verify");
          assert.equal(input.message, challenge.message);
          assert.equal("signature" in input, true);
          return session;
        },
      });
      assert.equal(second.type, "authenticated");
      assert.equal(second.phase, "restore-session");
      assert.deepEqual(calls, ["challenge:hub", "signMessage", "verify"]);
      assert.equal(readPendingHubAuthentication(), null);
    } finally {
      installed.restore();
    }
  });
}

test("React Strict Mode-like duplicate resume requests the challenge once", async () => {
  const installed = installWindow({
    pathname: "/events",
    href: chooseAddressHash("/events"),
  });
  installed.storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  let challenges = 0;
  try {
    const deps = {
      restoreSession: async () => null,
      createChallenge: async () => {
        challenges += 1;
        return { ...challenge, transport: "hub" as const };
      },
      signClient: { signMessage: async () => undefined },
      autoStartSignature: false as const,
    };
    const [first, second] = await Promise.all([resumeHubAuthFlow(deps), resumeHubAuthFlow(deps)]);
    assert.equal(first.type, "awaiting-signature");
    assert.equal(second.type, "awaiting-signature");
    assert.equal(challenges, 1);
    assert.equal(hubCallbackConsumeCountForTests(), 1);
  } finally {
    installed.restore();
  }
});

test("refresh after chooseAddress keeps continuation and can start signing", async () => {
  const storage = memoryStorage();
  const installed = installWindow({
    pathname: "/",
    href: chooseAddressHash("/"),
    storage,
  });
  storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  try {
    const first = await resumeHubAuthFlow({
      restoreSession: async () => null,
      createChallenge: async () => ({ ...challenge, transport: "hub" }),
      signClient: { signMessage: async () => undefined },
    });
    assert.equal(first.type, "redirecting");
    resetHubRedirectConsumption();
    const refreshed = await resumeHubAuthFlow({
      restoreSession: async () => null,
      takeRedirect: async () => ({ type: "empty" }),
      signClient: { signMessage: async () => undefined },
    });
    assert.equal(refreshed.type, "redirecting");
    assert.equal(refreshed.phase, "sign-challenge");
  } finally {
    installed.restore();
  }
});

test("refresh after signMessage still verifies the stored pending challenge", async () => {
  const storage = memoryStorage();
  const pending: PendingHubAuthentication = { transport: "hub", address, challenge };
  const installed = installWindow({
    pathname: "/events/abc",
    href: signatureHash("/events/abc"),
    storage,
  });
  persistPendingHubAuthentication(pending);
  persistHubAuthContinuation({ phase: "sign-challenge", addressSelected: true, returnPath: "/events/abc" });
  storage.setItem("rpcRequests", JSON.stringify({ "43": ["sign-message", { phase: "sign-message" }] }));
  try {
    const result = await resumeHubAuthFlow({
      restoreSession: async () => null,
      verifyChallenge: async () => session,
    });
    assert.equal(result.type, "authenticated");
  } finally {
    installed.restore();
  }
});

test("cancelling chooseAddress is idle without a scary error", async () => {
  const installed = installWindow({ pathname: "/", href: "https://nim-near.vercel.app/" });
  try {
    const result = await resumeHubAuthFlow({
      restoreSession: async () => null,
      takeRedirect: async () => ({
        type: "error",
        stage: "requesting-wallet",
        phase: "restore-address",
        error: new NimiqAuthError("requesting-wallet", "wallet_cancelled", "", true, new Error("Request was cancelled"), "restore-address"),
      }),
    });
    assert.equal(result.type, "cancelled");
    assert.equal(userMessageForAuthPhase(result.phase), "Sign-in could not be resumed. Please try again.");
  } finally {
    installed.restore();
  }
});

test("cancelling signature keeps a resume-safe phase", async () => {
  const installed = installWindow({ pathname: "/wallet", href: "https://nim-near.vercel.app/wallet" });
  persistPendingHubAuthentication({ transport: "hub", address, challenge });
  try {
    const result = await resumeHubAuthFlow({
      restoreSession: async () => null,
      takeRedirect: async () => ({
        type: "error",
        stage: "requesting-signature",
        phase: "restore-signature",
        error: new NimiqAuthError("requesting-signature", "wallet_cancelled", "", true, new Error("Request was cancelled"), "restore-signature"),
      }),
    });
    assert.equal(result.type, "cancelled");
    assert.equal(result.phase, "restore-signature");
    assert.equal(readPendingHubAuthentication()?.address, address);
  } finally {
    clearPendingHubAuthentication();
    installed.restore();
  }
});

test("Back from Hub is cancelled instead of wallet-unreachable", async () => {
  const installed = installWindow({ pathname: "/", href: "https://nim-near.vercel.app/" });
  try {
    const result = await resumeHubAuthFlow({
      restoreSession: async () => null,
      takeRedirect: async () => {
        throw new Error("Request aborted");
      },
    });
    assert.equal(result.type, "cancelled");
  } finally {
    installed.restore();
  }
});

test("malformed Hub callback is a restore failure, not wallet unreachable", async () => {
  const installed = installWindow({
    pathname: "/",
    href: "https://nim-near.vercel.app/#id=42&status=ok&result=%7Bnot-json",
  });
  installed.storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  try {
    const redirected = await takeHubRedirectResult({
      on() { throw new Error("unused"); },
      async checkRedirectResponse() { throw new Error("unused"); },
    } as never);
    assert.equal(redirected.type, "error");
    if (redirected.type === "error") {
      assert.equal(redirected.error instanceof NimiqAuthError, true);
      const error = redirected.error as NimiqAuthError;
      assert.equal(error.phase, "restore-address");
      assert.equal(error.message, "Sign-in could not be resumed. Please try again.");
      assert.equal(error.message.includes("could not be reached"), false);
    }
  } finally {
    installed.restore();
  }
});

test("stale Hub callback without a known result does not authenticate", async () => {
  const installed = installWindow({
    pathname: "/",
    href: `https://nim-near.vercel.app/#id=99&status=ok&result=${encodeURIComponent(JSON.stringify({ unexpected: true }))}`,
  });
  installed.storage.setItem("rpcRequests", JSON.stringify({ "42": ["choose-address", { phase: "choose-address" }] }));
  try {
    const result = await resumeHubAuthFlow({
      restoreSession: async () => null,
      autoStartSignature: false,
    });
    assert.equal(result.type, "idle");
  } finally {
    installed.restore();
  }
});

test("continuation state is discarded on network mismatch", () => {
  const installed = installWindow({ pathname: "/", href: "https://nim-near.vercel.app/" });
  const previousNetwork = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  try {
    process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = "test-albatross";
    persistSelectedHubAddress(address, "Teal");
    persistHubAuthContinuation({ phase: "sign-challenge", addressSelected: true, returnPath: "/" });
    process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = "main-albatross";
    discardStaleHubAuthState();
    assert.equal(readHubAuthContinuation(), null);
  } finally {
    if (previousNetwork == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previousNetwork;
    installed.restore();
  }
});

test("phase-aware user messages never use the generic wallet-unreachable copy", () => {
  assert.equal(userMessageForAuthPhase("choose-address"), "Could not open the Nimiq wallet.");
  assert.equal(userMessageForAuthPhase("restore-address"), "Sign-in could not be resumed. Please try again.");
  assert.equal(userMessageForAuthPhase("sign-challenge"), "Wallet signature could not be completed.");
  assert.equal(userMessageForAuthPhase("verify-challenge"), "Sign-in could not be verified.");
  for (const phase of ["choose-address", "restore-address", "request-challenge", "sign-challenge", "restore-signature", "verify-challenge"] as const) {
    assert.equal(userMessageForAuthPhase(phase).includes("could not be reached"), false);
  }
});

test("failure diagnostics keep phase and omit secrets", () => {
  const installed = installWindow({ pathname: "/", href: "https://nim-near.vercel.app/?token=secret-jwt" });
  persistSelectedHubAddress(address);
  try {
    const diagnostics = hubAuthFailureDiagnostics("requesting-signature", new Error("boom"), "sign-challenge");
    assert.equal(diagnostics.phase, "sign-challenge");
    assert.equal(diagnostics.walletAddress?.includes("KLJE"), false);
    assert.equal(JSON.stringify(diagnostics).includes("secret-jwt"), false);
    assert.equal(JSON.stringify(diagnostics).includes(challenge.message), false);
  } finally {
    installed.restore();
  }
});
