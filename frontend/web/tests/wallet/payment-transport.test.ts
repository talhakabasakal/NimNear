import assert from "node:assert/strict";
import test from "node:test";

import HubApi from "@nimiq/hub-api";

import type { PurchaseRecord } from "../../lib/api/purchases";
import type { PaymentRequestRecord, PublicPaymentRequest } from "../../lib/api/payment-requests";
import {
  NIMIQ_HUB_MAINNET,
  NIMIQ_HUB_TESTNET,
  resolveNimiqAuthConfig,
} from "../../lib/auth/nimiq-network";
import {
  hubRedirectBehavior,
  NIMIQ_HUB_PAYMENT_METHODS,
  resetHubRedirectConsumption,
  takeHubRedirectResult,
} from "../../lib/auth/nimiq";
import {
  createEventPurchaseFlight,
  executeEventPurchasePay,
  resumeHubPaidResource,
} from "../../lib/events/purchase-pay";
import {
  clearPendingHubPayment,
  detectNimiqPaymentTransport,
  discardStaleHubPaymentState,
  persistPendingHubPayment,
  readPendingHubPayment,
  resetHubPaymentState,
  sendNimiqPayment,
  type HubCheckoutRequest,
} from "../../lib/nimiq/payment-transport";
import { NimiqTransactionError } from "../../lib/nimiq/transactions";
import {
  createPaymentRequestFlight,
  executePaymentRequestPay,
} from "../../lib/payment-requests/pay";

process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK ??= "test-albatross";

const recipient = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const otherRecipient = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";
const hash = "ab".repeat(32);
const purchaseId = "11111111-1111-4111-8111-111111111111";
const publicId = "22222222-2222-4222-8222-222222222222";

function memoryStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value); },
    removeItem: (key: string) => { data.delete(key); },
  };
}

function installWindow(pathname = "/events/abc") {
  const storage = memoryStorage();
  const previous = globalThis.window;
  globalThis.window = {
    location: { origin: "http://localhost:3000", pathname, search: "" },
    sessionStorage: storage,
  } as never;
  return {
    storage,
    restore() {
      globalThis.window = previous;
    },
  };
}

async function withPublicNetwork<T>(value: string | undefined, run: () => Promise<T> | T) {
  const previous = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  if (value == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
  else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = value;
  try {
    return await run();
  } finally {
    if (previous == null) delete process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
    else process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK = previous;
  }
}

function purchaseRecord(status: PurchaseRecord["status"] = "submitted"): PurchaseRecord {
  return {
    id: purchaseId,
    event_id: "event-1",
    amount_lunas: "100000",
    amount_nim: "1",
    status,
    created_at: "2026-09-17T12:00:00Z",
    updated_at: "2026-09-17T12:00:00Z",
  };
}

function pendingPaymentRequest(): PublicPaymentRequest {
  return {
    public_id: publicId,
    recipient,
    amount_lunas: "2500000",
    amount_nim: "25",
    note: "Dinner",
    status: "pending",
    expires_at: "2099-09-18T12:00:00Z",
    network: "main-albatross",
  };
}

function paymentRequestRecord(status: PaymentRequestRecord["status"] = "submitted"): PaymentRequestRecord {
  return {
    ...pendingPaymentRequest(),
    status,
    created_at: "2026-09-17T12:00:00Z",
    updated_at: "2026-09-17T12:00:00Z",
  };
}

function signedCheckout(transactionHash = hash) {
  return { hash: transactionHash, serializedTx: "00", raw: { recipient, value: 100000 } };
}

test("detectNimiqPaymentTransport matches auth Mini App vs Hub selection", () => {
  const previous = globalThis.window;
  globalThis.window = { nimiqPay: {}, nimiq: undefined } as never;
  try {
    assert.equal(detectNimiqPaymentTransport(), "mini-app");
  } finally {
    globalThis.window = previous;
  }
});

test("Mainnet browser payments use Hub checkout and never open Mini App", async () => {
  resetHubPaymentState();
  resetHubRedirectConsumption();
  const { restore } = installWindow();
  let miniAppCalled = false;
  let checkout: HubCheckoutRequest | undefined;
  try {
    await withPublicNetwork("main-albatross", async () => {
      const result = await sendNimiqPayment(
        {
          recipient,
          amountLunas: BigInt("100000"),
          network: "main-albatross",
          resume: { kind: "event-purchase", id: purchaseId },
        },
        {
          transport: "hub",
          initMiniApp: async () => {
            miniAppCalled = true;
            throw new Error("Mini App must not open on the browser Hub path");
          },
          hub: {
            async checkout(request) {
              checkout = request;
              return signedCheckout() as never;
            },
          },
        },
      );
      assert.equal(result, hash);
      assert.equal(miniAppCalled, false);
      assert.deepEqual(checkout, { appName: "NIMNear", recipient, value: 100000 });
      const config = resolveNimiqAuthConfig();
      assert.equal(config.ok, true);
      if (config.ok) {
        assert.equal(config.hubEndpoint, NIMIQ_HUB_MAINNET);
        assert.equal(config.consensusName, "MainAlbatross");
        assert.equal(config.networkId, 24);
        assert.equal(config.environment, "mainnet");
      }
    });
  } finally {
    clearPendingHubPayment();
    restore();
  }
});

test("Mainnet Nimiq Pay uses Mini App sendBasicTransaction and not Hub checkout", async () => {
  resetHubPaymentState();
  const { restore } = installWindow();
  let hubCalled = false;
  let miniAppTx: { recipient: string; value: number } | undefined;
  try {
    await withPublicNetwork("main-albatross", async () => {
      const result = await sendNimiqPayment(
        {
          recipient,
          amountLunas: BigInt("100000"),
          network: "MainAlbatross",
          resume: { kind: "event-purchase", id: purchaseId },
        },
        {
          transport: "mini-app",
          hub: {
            async checkout() {
              hubCalled = true;
              throw new Error("Hub must not open inside Nimiq Pay");
            },
          },
          sendMiniApp: async (tx) => {
            miniAppTx = tx;
            return hash.toUpperCase();
          },
        },
      );
      assert.equal(result, hash);
      assert.equal(hubCalled, false);
      assert.deepEqual(miniAppTx, { recipient, value: 100000 });
    });
  } finally {
    restore();
  }
});

test("Testnet/mainnet mismatch fails before wallet opens", async () => {
  resetHubPaymentState();
  const { restore } = installWindow();
  let hubCalled = false;
  let miniAppCalled = false;
  try {
    await withPublicNetwork("main-albatross", async () => {
      await assert.rejects(
        () => sendNimiqPayment(
          {
            recipient,
            amountLunas: BigInt("100000"),
            network: "test-albatross",
            resume: { kind: "event-purchase", id: purchaseId },
          },
          {
            transport: "hub",
            hub: {
              async checkout() {
                hubCalled = true;
                return signedCheckout() as never;
              },
            },
            sendMiniApp: async () => {
              miniAppCalled = true;
              return hash;
            },
          },
        ),
        (error: unknown) => error instanceof NimiqTransactionError
          && error.kind === "network"
          && error.message.includes("does not match"),
      );
    });
    assert.equal(hubCalled, false);
    assert.equal(miniAppCalled, false);
  } finally {
    restore();
  }
});

test("Hub callback resumes the correct purchase and submits the Hub hash", async () => {
  resetHubPaymentState();
  resetHubRedirectConsumption();
  const { restore } = installWindow();
  try {
    await withPublicNetwork("main-albatross", async () => {
      persistPendingHubPayment({
        kind: "event-purchase",
        id: purchaseId,
        network: "main-albatross",
        environment: "mainnet",
        hubEndpoint: NIMIQ_HUB_MAINNET,
        returnPath: "/events/abc",
      });
      let submitted: { id: string; hash: string } | undefined;
      const resumed = await resumeHubPaidResource({
        kind: "event-purchase",
        id: purchaseId,
        takeRedirect: async () => ({ type: "checkout", hash }),
        submitHash: async (id, transactionHash) => {
          submitted = { id, hash: transactionHash };
          return purchaseRecord("submitted");
        },
      });
      assert.ok(resumed);
      assert.equal(resumed?.hash, hash);
      assert.deepEqual(submitted, { id: purchaseId, hash });
      assert.notEqual(resumed?.record.status, "confirmed");
    });
  } finally {
    clearPendingHubPayment();
    restore();
  }
});

test("stale Testnet payment state is discarded on Mainnet", async () => {
  const { restore } = installWindow();
  try {
    await withPublicNetwork("test-albatross", () => {
      persistPendingHubPayment({
        kind: "event-purchase",
        id: purchaseId,
        network: "test-albatross",
        environment: "testnet",
        hubEndpoint: NIMIQ_HUB_TESTNET,
        returnPath: "/events/abc",
        txHash: hash,
      });
      assert.equal(readPendingHubPayment()?.id, purchaseId);
    });
    await withPublicNetwork("main-albatross", () => {
      discardStaleHubPaymentState();
      assert.equal(readPendingHubPayment(), null);
      const stored = window.sessionStorage.getItem("nimnear.pay.hub.pending");
      assert.equal(stored, null);
    });
  } finally {
    restore();
  }
});

test("duplicate Hub callback cannot submit twice", async () => {
  resetHubPaymentState();
  const { restore } = installWindow();
  try {
    await withPublicNetwork("main-albatross", async () => {
      persistPendingHubPayment({
        kind: "event-purchase",
        id: purchaseId,
        network: "main-albatross",
        environment: "mainnet",
        hubEndpoint: NIMIQ_HUB_MAINNET,
        returnPath: "/events/abc",
      });
      let submits = 0;
      const submitHash = async () => {
        submits += 1;
        await new Promise((resolve) => setTimeout(resolve, 10));
        return purchaseRecord("submitted");
      };
      const first = resumeHubPaidResource({
        kind: "event-purchase",
        id: purchaseId,
        takeRedirect: async () => ({ type: "checkout", hash }),
        submitHash,
      });
      const second = resumeHubPaidResource({
        kind: "event-purchase",
        id: purchaseId,
        takeRedirect: async () => ({ type: "checkout", hash }),
        submitHash,
      });
      const [a, b] = await Promise.all([first, second]);
      assert.equal(a?.hash, hash);
      assert.equal(b?.hash, hash);
      assert.equal(submits, 1);
      const third = await resumeHubPaidResource({
        kind: "event-purchase",
        id: purchaseId,
        takeRedirect: async () => ({ type: "checkout", hash }),
        submitHash,
      });
      assert.equal(third, null);
      assert.equal(submits, 1);
    });
  } finally {
    clearPendingHubPayment();
    restore();
  }
});

test("backend recipient and amount remain authoritative for both transports", async () => {
  resetHubPaymentState();
  const { restore } = installWindow();
  try {
    await withPublicNetwork("main-albatross", async () => {
      let hubRequest: HubCheckoutRequest | undefined;
      const paid = await executeEventPurchasePay({
        purchaseId,
        instructions: { recipient, amount_lunas: "100000", network: "main-albatross" },
        submitHash: async (_id, transactionHash) => {
          assert.equal(transactionHash, hash);
          return purchaseRecord("submitted");
        },
        flight: createEventPurchaseFlight(),
        payment: {
          transport: "hub",
          hub: {
            async checkout(request) {
              hubRequest = request;
              return signedCheckout() as never;
            },
          },
        },
      });
      assert.equal(paid.redirected, false);
      if (!paid.redirected) {
        assert.equal(paid.record.status, "submitted");
        assert.notEqual(paid.record.status, "confirmed");
      }
      assert.deepEqual(hubRequest, { appName: "NIMNear", recipient, value: 100000 });
      assert.notEqual(hubRequest?.recipient, otherRecipient);
      assert.notEqual(hubRequest?.value, 999999);

      let miniAppTx: { recipient: string; value: number } | undefined;
      const requestPaid = await executePaymentRequestPay({
        request: pendingPaymentRequest(),
        locallyExpired: false,
        flight: createPaymentRequestFlight(),
        sendTransaction: async (to, amount) => sendNimiqPayment(
          {
            recipient: to,
            amountLunas: amount,
            network: "main-albatross",
            resume: { kind: "payment-request", id: publicId },
          },
          {
            transport: "mini-app",
            sendMiniApp: async (tx) => {
              miniAppTx = tx;
              return hash;
            },
          },
        ),
        submitHash: async () => paymentRequestRecord("submitted"),
      });
      assert.equal(requestPaid.ok, true);
      assert.deepEqual(miniAppTx, { recipient, value: 2_500_000 });
    });
  } finally {
    clearPendingHubPayment();
    restore();
  }
});

test("EventPurchase and PaymentRequest both work through Hub checkout", async () => {
  resetHubPaymentState();
  const { restore } = installWindow("/pay/" + publicId);
  try {
    await withPublicNetwork("main-albatross", async () => {
      const eventResult = await executeEventPurchasePay({
        purchaseId,
        instructions: { recipient, amount_lunas: "100000", network: "main-albatross" },
        submitHash: async () => purchaseRecord("submitted"),
        flight: createEventPurchaseFlight(),
        payment: {
          transport: "hub",
          hub: { async checkout() { return signedCheckout() as never; } },
        },
      });
      assert.equal(eventResult.redirected, false);

      resetHubPaymentState();
      const payResult = await executePaymentRequestPay({
        request: pendingPaymentRequest(),
        locallyExpired: false,
        flight: createPaymentRequestFlight(),
        sendTransaction: async (to, amount) => sendNimiqPayment(
          {
            recipient: to,
            amountLunas: amount,
            network: "main-albatross",
            resume: { kind: "payment-request", id: publicId },
          },
          {
            transport: "hub",
            hub: {
              async checkout(request) {
                assert.equal(request.recipient, recipient);
                assert.equal(request.value, 2_500_000);
                return signedCheckout() as never;
              },
            },
          },
        ),
        submitHash: async () => paymentRequestRecord("submitted"),
      });
      assert.equal(payResult.ok, true);
    });
  } finally {
    clearPendingHubPayment();
    restore();
  }
});

test("production never silently falls back to Testnet Hub checkout", async () => {
  resetHubPaymentState();
  const { restore } = installWindow();
  let hubCalled = false;
  try {
    await withPublicNetwork(undefined, async () => {
      const previousNodeEnv = process.env.NODE_ENV;
      const env = process.env as Record<string, string | undefined>;
      env.NODE_ENV = "production";
      try {
        const config = resolveNimiqAuthConfig();
        assert.equal(config.ok, false);
        await assert.rejects(
          () => sendNimiqPayment(
            {
              recipient,
              amountLunas: BigInt("1"),
              network: "test-albatross",
              resume: { kind: "event-purchase", id: purchaseId },
            },
            {
              transport: "hub",
              hub: {
                async checkout() {
                  hubCalled = true;
                  return signedCheckout() as never;
                },
              },
            },
          ),
          (error: unknown) => error instanceof NimiqTransactionError && error.kind === "network" && !hubCalled,
        );
      } finally {
        if (previousNodeEnv == null) delete env.NODE_ENV;
        else env.NODE_ENV = previousNodeEnv;
      }
    });
    await withPublicNetwork("main-albatross", async () => {
      const config = resolveNimiqAuthConfig();
      assert.equal(config.ok, true);
      if (!config.ok) return;
      assert.equal(config.hubEndpoint, "https://hub.nimiq.com");
      assert.equal(config.hubEndpoint.includes("hub.nimiq-testnet.com"), false);
      assert.deepEqual([...NIMIQ_HUB_PAYMENT_METHODS], ["checkout"]);
    });
  } finally {
    restore();
  }
});

test("Hub checkout redirect encodes checkout on the Mainnet Hub URL", async () => {
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  let href = "http://localhost:3000/events/abc";
  globalThis.document = { referrer: "" } as never;
  globalThis.history = { state: null, replaceState() {} } as never;
  globalThis.window = {
    location: {
      origin: "http://localhost:3000",
      pathname: "/events/abc",
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
    await withPublicNetwork("main-albatross", async () => {
      const config = resolveNimiqAuthConfig();
      assert.equal(config.ok, true);
      if (!config.ok) return;
      const hub = new HubApi(config.hubEndpoint);
      const navigation = hub.checkout(
        { appName: "NIMNear", recipient, value: 100000 },
        hubRedirectBehavior({ phase: "checkout" }) as never,
      );
      await Promise.race([navigation, new Promise((resolve) => setTimeout(resolve, 50))]);
      const redirected = new URL(href);
      assert.equal(redirected.origin, NIMIQ_HUB_MAINNET);
      assert.equal(redirected.hash.includes("command=checkout"), true);
      assert.match(decodeURIComponent(redirected.hash), /NIMNear/);
      assert.match(decodeURIComponent(redirected.hash), /100000/);
      assert.equal(href.startsWith("https://hub.nimiq-testnet.com"), false);
    });
  } finally {
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("Hub checkout hash return is recovered without document.referrer", async () => {
  resetHubRedirectConsumption();
  resetHubPaymentState();
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousHistory = globalThis.history;
  const storage = memoryStorage();
  storage.setItem("rpcRequests", JSON.stringify({ "7": ["checkout", { phase: "checkout" }] }));
  const result = encodeURIComponent(JSON.stringify({ hash, serializedTx: "00", raw: { recipient, value: 100000 } }));
  let href = `http://localhost:3000/events/abc#id=7&status=ok&result=${result}`;
  const location = {
    origin: "http://localhost:3000",
    pathname: "/events/abc",
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
      on() { throw new Error("HubApi handlers are not required when the hash already contains the checkout result"); },
      async checkRedirectResponse() { throw new Error("HubApi must not depend on document.referrer for HTTP returns"); },
    } as never);
    assert.equal(redirected.type, "checkout");
    if (redirected.type === "checkout") assert.equal(redirected.hash, hash);
  } finally {
    resetHubRedirectConsumption();
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    globalThis.history = previousHistory;
  }
});

test("pending Hub payment state stores no secrets", async () => {
  const { restore } = installWindow();
  try {
    await withPublicNetwork("main-albatross", () => {
      persistPendingHubPayment({
        kind: "event-purchase",
        id: purchaseId,
        network: "main-albatross",
        environment: "mainnet",
        hubEndpoint: NIMIQ_HUB_MAINNET,
        returnPath: "/events/abc",
      });
      const raw = window.sessionStorage.getItem("nimnear.pay.hub.pending") ?? "";
      assert.equal(raw.includes("jwt"), false);
      assert.equal(raw.includes("Bearer"), false);
      assert.equal(raw.includes("signature"), false);
      assert.equal(raw.includes("private"), false);
      const pending = readPendingHubPayment();
      assert.equal(pending?.id, purchaseId);
      assert.equal(pending?.hubEndpoint, NIMIQ_HUB_MAINNET);
    });
  } finally {
    clearPendingHubPayment();
    restore();
  }
});
