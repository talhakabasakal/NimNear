import HubApi, { type SignedTransaction } from "@nimiq/hub-api";

import {
  detectNimiqAuthTransport,
  getNimiqHubApi,
  hubRedirectBehavior,
  isNimiqHubEnabled,
  takeHubRedirectResult,
  type HubRedirectResult,
  type NimiqAuthTransport,
} from "@/lib/auth/nimiq";
import {
  paymentNetworkMatchesDeployment,
  resolveNimiqAuthConfig,
  type ResolvedNimiqAuthNetwork,
} from "@/lib/auth/nimiq-network";
import {
  initializeMiniAppProvider,
  lunaAmountForSdk,
  NimiqTransactionError,
  sendBasicNimTransaction,
  userMessageForTransactionFailure,
  type SendBasicTransactionFn,
} from "@/lib/nimiq/transactions";

const APP_NAME = "NIMNear";
const PENDING_PAYMENT_KEY = "nimnear.pay.hub.pending";
const HUB_PAYMENT_STATE_VERSION = 1;

export const NIMIQ_HUB_CHECKOUT_METHOD = "checkout" as const;

export type NimiqPaymentTransport = NimiqAuthTransport;
export type HubPaymentKind = "event-purchase" | "payment-request";

export type HubCheckoutRequest = {
  appName: string;
  recipient: string;
  value: number;
};

export type HubCheckoutClient = {
  checkout: (
    request: HubCheckoutRequest,
    behavior?: InstanceType<typeof HubApi.RedirectRequestBehavior>,
  ) => Promise<SignedTransaction | void>;
};

export type PendingHubPayment = {
  kind: HubPaymentKind;
  id: string;
  network: string;
  environment: string;
  hubEndpoint: string;
  returnPath: string;
  txHash?: string;
  submitted?: boolean;
};

type StoredPendingHubPayment = PendingHubPayment & {
  v: number;
};

type HubPaymentIdentity = {
  network: string;
  environment: string;
  hubEndpoint: string;
};

export type SendNimiqPaymentInput = {
  recipient: string;
  amountLunas: bigint;
  network: string;
  resume: { kind: HubPaymentKind; id: string };
};

export type SendNimiqPaymentDeps = {
  transport?: NimiqPaymentTransport;
  hub?: HubCheckoutClient;
  sendMiniApp?: SendBasicTransactionFn;
  initMiniApp?: typeof initializeMiniAppProvider;
  takeRedirect?: () => Promise<HubRedirectResult>;
};

let hubCheckoutRedirectStarted = false;
let paymentSubmitInFlight: Promise<string | undefined> | null = null;
const hubPaymentSubmitOnce = new Map<string, Promise<unknown>>();

function currentHubPaymentIdentity(): HubPaymentIdentity | null {
  const network = resolveNimiqAuthConfig();
  if (!network.ok) return null;
  return { network: network.network, environment: network.environment, hubEndpoint: network.hubEndpoint };
}

function currentReturnPath() {
  if (typeof window === "undefined") return "";
  return `${window.location.pathname}${window.location.search}`;
}

function isCurrentPendingHubPayment(
  pending: StoredPendingHubPayment,
  identity: HubPaymentIdentity | null,
): pending is StoredPendingHubPayment {
  return Boolean(
    identity &&
      pending.v === HUB_PAYMENT_STATE_VERSION &&
      (pending.kind === "event-purchase" || pending.kind === "payment-request") &&
      pending.id &&
      pending.network === identity.network &&
      pending.environment === identity.environment &&
      pending.hubEndpoint === identity.hubEndpoint,
  );
}

function publicPending(pending: StoredPendingHubPayment): PendingHubPayment {
  return {
    kind: pending.kind,
    id: pending.id,
    network: pending.network,
    environment: pending.environment,
    hubEndpoint: pending.hubEndpoint,
    returnPath: pending.returnPath,
    ...(pending.txHash ? { txHash: pending.txHash } : {}),
    ...(pending.submitted ? { submitted: true } : {}),
  };
}

export function detectNimiqPaymentTransport(): NimiqPaymentTransport {
  return detectNimiqAuthTransport();
}

export function paymentTransportLabel(transport: NimiqPaymentTransport = detectNimiqPaymentTransport()) {
  return transport === "mini-app" ? "Nimiq Pay" : "Nimiq Hub";
}

export function requirePaymentNetworkMatch(
  instructionsNetwork: string,
): ResolvedNimiqAuthNetwork {
  const configured = resolveNimiqAuthConfig();
  if (!configured.ok) {
    throw new NimiqTransactionError("network", configured.message);
  }
  if (!paymentNetworkMatchesDeployment(instructionsNetwork)) {
    throw new NimiqTransactionError(
      "network",
      "Payment network does not match this Nimiq deployment.",
    );
  }
  return configured;
}

export function discardStaleHubPaymentState(
  storage: Pick<Storage, "getItem" | "removeItem"> | undefined = typeof window !== "undefined"
    ? window.sessionStorage
    : undefined,
) {
  if (!storage) return;
  const stored = storage.getItem(PENDING_PAYMENT_KEY);
  if (!stored) return;
  try {
    const pending = JSON.parse(stored) as StoredPendingHubPayment;
    if (!isCurrentPendingHubPayment(pending, currentHubPaymentIdentity())) {
      storage.removeItem(PENDING_PAYMENT_KEY);
    }
  } catch {
    storage.removeItem(PENDING_PAYMENT_KEY);
  }
}

export function persistPendingHubPayment(
  pending: PendingHubPayment,
  storage: Pick<Storage, "setItem" | "removeItem"> | undefined = typeof window !== "undefined"
    ? window.sessionStorage
    : undefined,
) {
  if (!storage) return;
  const identity = currentHubPaymentIdentity();
  if (!identity) {
    storage.removeItem(PENDING_PAYMENT_KEY);
    return;
  }
  const stored: StoredPendingHubPayment = {
    ...pending,
    v: HUB_PAYMENT_STATE_VERSION,
    network: identity.network,
    environment: identity.environment,
    hubEndpoint: identity.hubEndpoint,
  };
  storage.setItem(PENDING_PAYMENT_KEY, JSON.stringify(stored));
}

export function readPendingHubPayment(
  storage: Pick<Storage, "getItem" | "removeItem"> | undefined = typeof window !== "undefined"
    ? window.sessionStorage
    : undefined,
): PendingHubPayment | null {
  if (!storage) return null;
  discardStaleHubPaymentState(storage);
  const stored = storage.getItem(PENDING_PAYMENT_KEY);
  if (!stored) return null;
  try {
    const pending = JSON.parse(stored) as StoredPendingHubPayment;
    if (!isCurrentPendingHubPayment(pending, currentHubPaymentIdentity())) {
      storage.removeItem(PENDING_PAYMENT_KEY);
      return null;
    }
    return publicPending(pending);
  } catch {
    storage.removeItem(PENDING_PAYMENT_KEY);
    return null;
  }
}

export function clearPendingHubPayment(
  storage: Pick<Storage, "removeItem"> | undefined = typeof window !== "undefined"
    ? window.sessionStorage
    : undefined,
) {
  storage?.removeItem(PENDING_PAYMENT_KEY);
}

export function markHubPaymentHash(txHash: string) {
  const pending = readPendingHubPayment();
  if (!pending) return;
  persistPendingHubPayment({ ...pending, txHash });
}

export function markHubPaymentSubmitted(txHash?: string) {
  const pending = readPendingHubPayment();
  if (!pending) return;
  persistPendingHubPayment({
    ...pending,
    ...(txHash ? { txHash } : {}),
    submitted: true,
  });
}

export function prepareNimiqPayment() {
  discardStaleHubPaymentState();
}

function wrapPaymentError(error: unknown, transport: NimiqPaymentTransport): NimiqTransactionError {
  if (error instanceof NimiqTransactionError) return error;
  const text = error instanceof Error ? `${error.name} ${error.message}` : String(error);
  const lowered = text.toLowerCase();
  if (lowered.includes("cancel") || lowered.includes("reject") || lowered.includes("denied") || lowered.includes("permission")) {
    return new NimiqTransactionError("cancelled", "The transaction was cancelled in the wallet.");
  }
  if (lowered.includes("popup")) {
    return new NimiqTransactionError("unavailable", "The browser blocked the Hub window. Allow popups and try again.");
  }
  if (lowered.includes("insufficient") || lowered.includes("not enough")) {
    return new NimiqTransactionError("insufficient_funds", userMessageForTransactionFailure("insufficient_funds"));
  }
  if (lowered.includes("network") || lowered.includes("test-albatross") || lowered.includes("main-albatross")) {
    return new NimiqTransactionError("network", userMessageForTransactionFailure("network"));
  }
  if (transport === "hub") {
    return new NimiqTransactionError("unavailable", "Nimiq Hub could not complete this payment. Try again.");
  }
  return new NimiqTransactionError("unavailable", userMessageForTransactionFailure("unavailable"));
}

function extractCheckoutHash(result: SignedTransaction | { hash?: unknown } | void): string {
  const hash = result && typeof result === "object" ? normalizeHash(result.hash) : null;
  if (!hash) {
    throw new NimiqTransactionError(
      "unexpected",
      "The Hub checkout result did not include a transaction hash.",
    );
  }
  return hash;
}

function normalizeHash(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const normalized = value.trim().toLowerCase().replace(/^0x/, "");
  if (!/^[0-9a-f]{64}$/.test(normalized)) return null;
  return normalized;
}

function pendingMatchesResume(pending: PendingHubPayment | null, resume: SendNimiqPaymentInput["resume"]) {
  return Boolean(pending && pending.kind === resume.kind && pending.id === resume.id);
}

export async function recoverHubPaymentHash(
  resume: SendNimiqPaymentInput["resume"],
  takeRedirect: () => Promise<HubRedirectResult> = takeHubRedirectResult,
): Promise<string | null> {
  discardStaleHubPaymentState();
  const pending = readPendingHubPayment();
  if (!pending) return null;
  if (pending.kind !== resume.kind) return null;
  if (pending.id !== resume.id) {
    clearPendingHubPayment();
    return null;
  }
  if (pending?.submitted && pending.txHash) {
    return pending.txHash;
  }
  if (pending?.txHash) return pending.txHash;

  let redirected: HubRedirectResult;
  try {
    redirected = await takeRedirect();
  } catch (error) {
    throw wrapPaymentError(error, "hub");
  }
  if (redirected.type === "checkout-error") {
    hubCheckoutRedirectStarted = false;
    throw wrapPaymentError(redirected.error, "hub");
  }
  if (redirected.type !== "checkout") {
    hubCheckoutRedirectStarted = false;
    return pending?.txHash ?? null;
  }
  markHubPaymentHash(redirected.hash);
  return redirected.hash;
}

async function sendHubCheckoutPayment(
  input: SendNimiqPaymentInput,
  identity: ResolvedNimiqAuthNetwork,
  deps: SendNimiqPaymentDeps,
): Promise<string | undefined> {
  if (!isNimiqHubEnabled()) {
    throw new NimiqTransactionError(
      "unavailable",
      "Nimiq Hub is disabled. Open NIMNear in Nimiq Pay to continue.",
    );
  }
  const existing = readPendingHubPayment();
  if (pendingMatchesResume(existing, input.resume)) {
    const recovered = await recoverHubPaymentHash(input.resume, deps.takeRedirect ?? takeHubRedirectResult);
    if (recovered) return recovered;
  }

  persistPendingHubPayment({
    kind: input.resume.kind,
    id: input.resume.id,
    network: identity.network,
    environment: identity.environment,
    hubEndpoint: identity.hubEndpoint,
    returnPath: currentReturnPath(),
  });

  const request: HubCheckoutRequest = {
    appName: APP_NAME,
    recipient: input.recipient,
    value: lunaAmountForSdk(input.amountLunas),
  };
  const behavior = typeof window === "undefined" ? undefined : hubRedirectBehavior({ phase: NIMIQ_HUB_CHECKOUT_METHOD });
  if (behavior && hubCheckoutRedirectStarted) {
    const recovered = await recoverHubPaymentHash(input.resume, deps.takeRedirect ?? takeHubRedirectResult);
    if (recovered) return recovered;
    hubCheckoutRedirectStarted = false;
  }
  if (behavior) hubCheckoutRedirectStarted = true;

  const client = deps.hub ?? (getNimiqHubApi() as HubCheckoutClient);
  let signed: SignedTransaction | void;
  try {
    signed = await client.checkout(request, behavior);
  } catch (error) {
    hubCheckoutRedirectStarted = false;
    throw wrapPaymentError(error, "hub");
  }
  if (!signed) return undefined;
  const hash = extractCheckoutHash(signed);
  markHubPaymentHash(hash);
  return hash;
}

async function sendMiniAppPayment(
  input: SendNimiqPaymentInput,
  deps: SendNimiqPaymentDeps,
): Promise<string> {
  const send = deps.sendMiniApp;
  if (send) {
    return sendBasicNimTransaction(
      { recipient: input.recipient, valueLunas: input.amountLunas },
      send,
    );
  }
  const init = deps.initMiniApp ?? initializeMiniAppProvider;
  const provider = await init();
  return sendBasicNimTransaction(
    { recipient: input.recipient, valueLunas: input.amountLunas },
    (tx) => provider.sendBasicTransaction(tx),
  );
}

export async function sendNimiqPayment(
  input: SendNimiqPaymentInput,
  deps: SendNimiqPaymentDeps = {},
): Promise<string | undefined> {
  const identity = requirePaymentNetworkMatch(input.network);
  const transport = deps.transport ?? detectNimiqPaymentTransport();
  if (paymentSubmitInFlight) return paymentSubmitInFlight;

  const pending = readPendingHubPayment();
  if (pending?.submitted && pendingMatchesResume(pending, input.resume) && pending.txHash) {
    return pending.txHash;
  }

  const run = (async () => {
    if (transport === "mini-app") {
      return sendMiniAppPayment(input, deps);
    }
    return sendHubCheckoutPayment(input, identity, deps);
  })();
  paymentSubmitInFlight = run.finally(() => {
    paymentSubmitInFlight = null;
  });
  return paymentSubmitInFlight;
}

export function runHubPaymentSubmitOnce<T>(purchaseKey: string, hash: string, fn: () => Promise<T>): Promise<T> {
  const key = `${purchaseKey}:${hash}`;
  const existing = hubPaymentSubmitOnce.get(key);
  if (existing) return existing as Promise<T>;
  const pending = fn();
  hubPaymentSubmitOnce.set(key, pending);
  return pending.finally(() => {
    if (hubPaymentSubmitOnce.get(key) === pending) hubPaymentSubmitOnce.delete(key);
  }) as Promise<T>;
}

export function resetHubPaymentState() {
  hubCheckoutRedirectStarted = false;
  paymentSubmitInFlight = null;
  hubPaymentSubmitOnce.clear();
}

export function hubPaymentStateVersion() {
  return HUB_PAYMENT_STATE_VERSION;
}
