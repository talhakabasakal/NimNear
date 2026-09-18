import HubApi, { type SignedMessage } from "@nimiq/hub-api";
import {
  getHostLanguage,
  init,
  type ErrorResponse,
  type NimiqProvider,
  type SignatureResult,
} from "@nimiq/mini-app-sdk";
import { RedirectRpcClient } from "@nimiq/rpc";
import { normalizeTransactionHash } from "@/lib/nimiq/transactions";
import { PUBLIC_NODE_ENV } from "@/lib/auth/nimiq-public-env";

import {
  AuthApiError,
  createNimiqChallenge,
  restoreAuthSession,
  verifyNimiqChallenge,
  type AuthSession,
  type NimiqChallenge,
} from "@/lib/api/auth";
import { isNimiqHubEnabled, resolveNimiqAuthConfig } from "@/lib/auth/nimiq-network";
import {
  callbackOperationFromCommand,
  HUB_AUTH_CONTINUATION_VERSION,
  isHistoryStateCloneError,
  NIMIQ_HUB_CALLBACK_STORAGE_KEY,
  NIMIQ_HUB_CONTINUATION_KEY,
  phaseFromAuthStage,
  redactWalletAddress,
  userMessageForAuthPhase,
  type AuthPhase,
  type AuthStage,
  type HubAuthContinuation,
  type HubAuthFailureDiagnostics,
  type HubCallbackMeta,
  type StoredHubRpcCallback,
} from "@/lib/auth/nimiq-auth-phase";

export {
  isNimiqHubEnabled,
  requireNimiqAuthConfig,
  resolveNimiqAuthConfig,
} from "@/lib/auth/nimiq-network";
export {
  HUB_AUTH_CONTINUATION_VERSION,
  NIMIQ_HUB_CALLBACK_STORAGE_KEY,
  NIMIQ_HUB_CONTINUATION_KEY,
  phaseFromAuthStage,
  redactWalletAddress,
  userMessageForAuthPhase,
  type AuthPhase,
  type AuthStage,
  type HubAuthContinuation,
  type HubAuthFailureDiagnostics,
  type HubCallbackMeta,
} from "@/lib/auth/nimiq-auth-phase";
export const NIMIQ_HUB_PUBLIC_METHODS = ["chooseAddress", "signMessage"] as const;
export const NIMIQ_HUB_PAYMENT_METHODS = ["checkout"] as const;
const APP_NAME = "NIMNear";
const PENDING_HUB_KEY = "nimnear.auth.hub.pending";
const SELECTED_HUB_ADDRESS_KEY = "nimnear.auth.hub.selectedAddress";
const SELECTED_HUB_LABEL_KEY = "nimnear.auth.hub.accountLabel";
const SELECTED_HUB_NETWORK_KEY = "nimnear.auth.hub.network";
const HUB_UI_KEY = "nimnear.auth.hub.ui";
const RPC_REQUESTS_KEY = "rpcRequests";
const HUB_AUTH_STATE_VERSION = 2;
const HUB_CALLBACK_STORAGE_VERSION = 1;

type HubAuthIdentity = {
  network: string;
  environment: string;
  hubEndpoint: string;
};

type StoredPendingHubAuthentication = PendingHubAuthentication & {
  v: number;
  network: string;
  environment: string;
  hubEndpoint: string;
};

export type NimiqAuthTransport = "mini-app" | "hub";

export class NimiqAuthError extends Error {
  override readonly cause?: unknown;
  readonly phase: AuthPhase;
  constructor(
    public readonly stage: AuthStage,
    public readonly code: string,
    message: string,
    public readonly cancelled = false,
    cause?: unknown,
    phase?: AuthPhase,
  ) {
    super(message);
    this.name = "NimiqAuthError";
    this.phase = phase ?? phaseFromAuthStage(stage);
    if (cause !== undefined) this.cause = cause;
  }
}

export type MiniAppWallet = {
  transport: "mini-app";
  provider: NimiqProvider;
  accounts: string[];
};

export type PendingHubAuthentication = {
  transport: "hub";
  address: string;
  accountLabel?: string;
  challenge: NimiqChallenge;
};

export type HubRedirectResult =
  | { type: "address"; address: string; label?: string }
  | { type: "signature"; signed: SignedMessage }
  | { type: "checkout"; hash: string }
  | { type: "checkout-error"; error: unknown }
  | { type: "error"; error: unknown; stage: AuthStage; phase?: AuthPhase }
  | { type: "empty" };

export type HubAuthResumeResult =
  | { type: "authenticated"; session: AuthSession; phase: AuthPhase }
  | { type: "redirecting"; phase: AuthPhase; pending?: PendingHubAuthentication }
  | { type: "awaiting-signature"; pending: PendingHubAuthentication; phase: AuthPhase }
  | { type: "idle"; phase: AuthPhase }
  | { type: "payment"; phase: AuthPhase }
  | { type: "cancelled"; phase: AuthPhase }
  | { type: "error"; error: NimiqAuthError; phase: AuthPhase };

type HubChooseAddressClient = {
  chooseAddress: (
    request: { appName: string },
    behavior?: InstanceType<typeof HubApi.RedirectRequestBehavior>,
  ) => Promise<{ address: string; label?: string; meta?: { account?: { label?: string } } } | void>;
};

type HubSignMessageClient = {
  signMessage: (
    request: { appName: string; signer: string; message: string },
    behavior?: InstanceType<typeof HubApi.RedirectRequestBehavior>,
  ) => Promise<SignedMessage | void>;
};

type HubRedirectClient = {
  on: HubApi["on"];
  checkRedirectResponse: HubApi["checkRedirectResponse"];
};

let hubApi: HubApi | null = null;
let hubApiEndpoint: string | null = null;
let redirectWaiter: Promise<HubRedirectResult> | null = null;
let capturedHubRedirect: HubRedirectResult | null = null;
let capturedHubRedirectRead = false;
let hubSignRedirectStarted = false;
let hubReturnRestoreClaimed = false;
let hubAuthResume: Promise<HubAuthResumeResult> | null = null;
let hubCallbackConsumeCount = 0;
const challengeInFlight = new Map<string, Promise<PendingHubAuthentication>>();
let verificationInFlight: Promise<AuthSession> | null = null;

function getHubApi() {
  if (typeof window === "undefined") {
    throw new NimiqAuthError("requesting-wallet", "browser_required", "Nimiq Hub is only available in a browser.");
  }
  if (!isNimiqHubEnabled()) {
    throw new NimiqAuthError(
      "requesting-wallet",
      "hub_disabled",
      "Nimiq Hub fallback is disabled. Open NIMNear in Nimiq Pay to sign in.",
    );
  }
  const network = resolveNimiqAuthConfig();
  if (!network.ok) {
    throw new NimiqAuthError("detecting-environment", network.code, network.message);
  }
  if (!hubApi || hubApiEndpoint !== network.hubEndpoint) {
    hubApi = new HubApi(network.hubEndpoint);
    hubApiEndpoint = network.hubEndpoint;
  }
  return hubApi;
}

export function getNimiqHubApi() {
  return getHubApi();
}

export function isHubPaymentRedirect(
  result: HubRedirectResult,
): result is Extract<HubRedirectResult, { type: "checkout" } | { type: "checkout-error" }> {
  return result.type === "checkout" || result.type === "checkout-error";
}

function currentHubAuthIdentity(): HubAuthIdentity | null {
  const network = resolveNimiqAuthConfig();
  if (!network.ok) return null;
  return { network: network.network, environment: network.environment, hubEndpoint: network.hubEndpoint };
}

function hubAuthIdentityKey(identity: HubAuthIdentity) {
  return `${identity.network}|${identity.environment}|${identity.hubEndpoint}`;
}

function clearHubRpcRequests() {
  if (typeof window === "undefined") return;
  window.sessionStorage.removeItem(RPC_REQUESTS_KEY);
}

function clearHubAuthContinuation() {
  if (typeof window === "undefined") return;
  window.sessionStorage.removeItem(NIMIQ_HUB_CONTINUATION_KEY);
}

export function discardStaleHubAuthState() {
  if (typeof window === "undefined") return;
  const identity = currentHubAuthIdentity();
  const storedPending = window.sessionStorage.getItem(PENDING_HUB_KEY);
  if (storedPending) {
    try {
      const pending = JSON.parse(storedPending) as StoredPendingHubAuthentication;
      if (!isCurrentPendingHubAuthentication(pending, identity)) {
        clearPendingHubAuthentication();
        clearHubRpcRequests();
      }
    } catch {
      clearPendingHubAuthentication();
      clearHubRpcRequests();
    }
  }
  const storedIdentity = window.sessionStorage.getItem(SELECTED_HUB_NETWORK_KEY);
  if (!identity || storedIdentity !== hubAuthIdentityKey(identity)) {
    window.sessionStorage.removeItem(SELECTED_HUB_ADDRESS_KEY);
    window.sessionStorage.removeItem(SELECTED_HUB_LABEL_KEY);
    window.sessionStorage.removeItem(SELECTED_HUB_NETWORK_KEY);
    if (storedIdentity) clearHubRpcRequests();
  }
  const continuation = readHubAuthContinuation();
  if (continuation && identity) {
    if (
      continuation.network !== identity.network ||
      continuation.environment !== identity.environment ||
      continuation.hubEndpoint !== identity.hubEndpoint
    ) {
      clearHubAuthContinuation();
    }
  } else if (continuation && !identity) {
    clearHubAuthContinuation();
  }
}

function isCurrentPendingHubAuthentication(
  pending: StoredPendingHubAuthentication,
  identity: HubAuthIdentity | null,
): pending is StoredPendingHubAuthentication {
  return Boolean(
    identity &&
      pending.v === HUB_AUTH_STATE_VERSION &&
      pending.transport === "hub" &&
      pending.address &&
      pending.challenge?.challenge_id &&
      pending.challenge.message &&
      pending.network === identity.network &&
      pending.environment === identity.environment &&
      pending.hubEndpoint === identity.hubEndpoint &&
      pending.challenge.network === identity.network &&
      pending.challenge.environment === identity.environment,
  );
}

export function prepareNimiqHub() {
  if (typeof window === "undefined") return;
  discardStaleHubAuthState();
  captureHubRpcCallbackFromWindow();
  captureHubRedirectFromLocation();
  try {
    void getHubApi();
  } catch {
    hubApi = null;
    hubApiEndpoint = null;
  }
}

export function nimiqHubReturnUrl() {
  return `${window.location.origin}${window.location.pathname}${window.location.search}`;
}

function hubReturnUrl() {
  return nimiqHubReturnUrl();
}

export function hubRedirectBehavior(localState: Record<string, string> = {}) {
  if (typeof window === "undefined") {
    throw new NimiqAuthError("requesting-wallet", "browser_required", "Nimiq Hub is only available in a browser.");
  }
  const returnUrl = hubReturnUrl();
  const behavior = new HubApi.RedirectRequestBehavior(returnUrl, localState);
  // Do not call RedirectRpcClient.init() here. init() parses the current URL as a
  // Hub *response*, can throw DataCloneError via history.replaceState(Next.js state),
  // and can consume the other half of a two-step auth flow (chooseAddress vs signMessage).
  behavior.request = async (endpoint, command, args) => {
    const client = new RedirectRpcClient(endpoint, new URL(endpoint).origin);
    client.call(
      returnUrl,
      command,
      { handleHistoryBack: false, state: { ...localState, __command: command } },
      ...(await Promise.all(Array.from(args))),
    );
  };
  return behavior;
}

function parseHubRpcJson(raw: string): unknown {
  return JSON.parse(raw, (_key, value) => {
    if (value && typeof value === "object" && value.__ === 0 && typeof value.v === "string") {
      const binary = atob(value.v);
      const bytes = new Uint8Array(binary.length);
      for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
      return bytes;
    }
    return value;
  });
}

function commandForRpcId(id: number): string | null {
  if (typeof window === "undefined") return null;
  const stored = window.sessionStorage.getItem(RPC_REQUESTS_KEY);
  if (!stored) return null;
  try {
    const requests = JSON.parse(stored) as Record<string, [string, unknown]>;
    const entry = requests[String(id)];
    return Array.isArray(entry) && typeof entry[0] === "string" ? entry[0] : null;
  } catch {
    return null;
  }
}

function isSignedHubMessage(value: unknown): value is SignedMessage {
  return Boolean(value && typeof value === "object" && "signer" in value && "signature" in value && "signerPublicKey" in value);
}

function isChosenHubAddress(value: unknown): value is { address: string; label?: string; meta?: { account?: { label?: string } } } {
  return Boolean(value && typeof value === "object" && "address" in value && typeof (value as { address: unknown }).address === "string");
}

function isSignedHubTransaction(value: unknown): value is { hash: string } {
  if (!value || typeof value !== "object") return false;
  const record = value as { hash?: unknown; serializedTx?: unknown; raw?: unknown; success?: unknown };
  if (record.success === true && record.hash == null) return false;
  return typeof record.hash === "string" && (typeof record.serializedTx === "string" || record.raw != null || Boolean(normalizeTransactionHash(record.hash)));
}

function checkoutHashFromResult(value: unknown): string | null {
  if (!isSignedHubTransaction(value)) return null;
  return normalizeTransactionHash(value.hash);
}

function hubAccountLabel(value: { label?: string; meta?: { account?: { label?: string } } } | null | undefined): string | undefined {
  const label = value?.meta?.account?.label?.trim() || value?.label?.trim();
  return label || undefined;
}

function hubRpcErrorValue(result: unknown) {
  if (result && typeof result === "object" && "message" in result) {
    return new Error(String((result as { message: unknown }).message));
  }
  return result instanceof Error ? result : new Error(typeof result === "string" ? result : "The Hub request was cancelled.");
}

function interpretHubRpcResult(command: string | null, status: "ok" | "error", result: unknown): HubRedirectResult {
  if (status === "error") {
    if (command === HubApi.RequestType.CHECKOUT) {
      return { type: "checkout-error", error: hubRpcErrorValue(result) };
    }
    const restore = command === HubApi.RequestType.SIGN_MESSAGE;
    const stage: AuthStage = restore ? "requesting-signature" : "requesting-wallet";
    const phase: AuthPhase = restore ? "restore-signature" : "restore-address";
    return { type: "error", error: wrapWalletError(stage, hubRpcErrorValue(result), phase), stage, phase };
  }
  if (command === HubApi.RequestType.CHECKOUT || isSignedHubTransaction(result)) {
    const hash = checkoutHashFromResult(result);
    if (!hash) {
      return { type: "checkout-error", error: new Error("The Hub checkout result did not include a transaction hash.") };
    }
    return { type: "checkout", hash };
  }
  if (command === HubApi.RequestType.SIGN_MESSAGE || isSignedHubMessage(result)) {
    if (!isSignedHubMessage(result)) return { type: "empty" };
    return { type: "signature", signed: result };
  }
  if (command === HubApi.RequestType.CHOOSE_ADDRESS || isChosenHubAddress(result)) {
    if (!isChosenHubAddress(result)) return { type: "empty" };
    persistSelectedHubAddress(result.address, hubAccountLabel(result));
    const label = hubAccountLabel(result);
    return label ? { type: "address", address: result.address, label } : { type: "address", address: result.address };
  }
  return { type: "empty" };
}

function replaceLocationUrl(href: string) {
  try {
    window.history.replaceState(window.history.state, "", href);
    return;
  } catch {
    // Next.js App Router history.state is often not structured-cloneable.
  }
  try {
    window.history.replaceState(null, "", href);
    return;
  } catch {
    // Keep the parsed callback in sessionStorage even if the hash remains.
  }
  try {
    const next = new URL(href, window.location.origin);
    window.location.hash = next.hash;
  } catch {
    // Hash clearing is best-effort; sessionStorage is the source of truth.
  }
}

function locationWithoutHubCallbackHash() {
  const url = new URL(window.location.href);
  const fragment = new URLSearchParams(url.hash.substring(1));
  fragment.delete("id");
  fragment.delete("status");
  fragment.delete("result");
  url.hash = fragment.toString();
  return `${url.pathname}${url.search}${url.hash}`;
}

export function captureHubRpcCallbackFromWindow(): boolean {
  if (typeof window === "undefined") return false;
  try {
    if (window.sessionStorage.getItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY)) {
      replaceLocationUrl(locationWithoutHubCallbackHash());
      return true;
    }
    const fragment = new URLSearchParams(window.location.hash.substring(1));
    const id = fragment.get("id");
    const status = fragment.get("status");
    const result = fragment.get("result");
    if (!id || !status || result == null) return false;
    const stored: StoredHubRpcCallback = {
      v: HUB_CALLBACK_STORAGE_VERSION,
      id,
      status,
      result,
      pathname: window.location.pathname,
      search: window.location.search,
    };
    window.sessionStorage.setItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY, JSON.stringify(stored));
    replaceLocationUrl(locationWithoutHubCallbackHash());
    return true;
  } catch {
    return false;
  }
}

function readStoredHubRpcCallback(): StoredHubRpcCallback | null {
  if (typeof window === "undefined") return null;
  const raw = window.sessionStorage.getItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY);
  if (!raw) return null;
  try {
    const stored = JSON.parse(raw) as StoredHubRpcCallback;
    if (stored.v !== HUB_CALLBACK_STORAGE_VERSION || !stored.id || !stored.status || stored.result == null) {
      window.sessionStorage.removeItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY);
      return null;
    }
    return stored;
  } catch {
    window.sessionStorage.removeItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY);
    return null;
  }
}

function interpretStoredHubCallback(stored: StoredHubRpcCallback): HubRedirectResult | null {
  const id = Number.parseInt(stored.id, 10);
  if (!Number.isFinite(id)) return null;
  let result: unknown;
  try {
    result = parseHubRpcJson(stored.result);
  } catch {
    return {
      type: "error",
      error: wrapWalletError(
        "requesting-wallet",
        new Error("malformed Hub callback"),
        "restore-address",
      ),
      stage: "requesting-wallet",
      phase: "restore-address",
    };
  }
  const interpreted = interpretHubRpcResult(
    commandForRpcId(id),
    stored.status === "ok" ? "ok" : "error",
    result,
  );
  return interpreted.type === "empty" ? null : interpreted;
}

function consumeStoredHubCallback(): HubRedirectResult | null {
  const stored = readStoredHubRpcCallback();
  if (!stored) return null;
  window.sessionStorage.removeItem(NIMIQ_HUB_CALLBACK_STORAGE_KEY);
  hubCallbackConsumeCount += 1;
  return interpretStoredHubCallback(stored) ?? { type: "empty" };
}

export function hubCallbackConsumeCountForTests() {
  return hubCallbackConsumeCount;
}

export function peekHubCallbackMeta(): HubCallbackMeta {
  const stored = readStoredHubRpcCallback();
  const fragment = typeof window === "undefined" ? null : new URLSearchParams(window.location.hash.substring(1));
  const hashPresent = Boolean(fragment?.get("id") && fragment.get("status") && fragment.get("result") != null);
  const idRaw = stored?.id ?? fragment?.get("id");
  const id = idRaw ? Number.parseInt(idRaw, 10) : Number.NaN;
  const command = Number.isFinite(id) ? commandForRpcId(id) : null;
  const statusRaw = stored?.status ?? fragment?.get("status");
  return {
    hashPresent,
    storagePresent: Boolean(stored),
    operation: callbackOperationFromCommand(command),
    status: statusRaw === "ok" || statusRaw === "error" ? statusRaw : null,
    pathname: typeof window !== "undefined" ? window.location.pathname : "",
  };
}

function readAndClearHubRedirectHash(): HubRedirectResult | null {
  if (typeof window === "undefined") return null;
  captureHubRpcCallbackFromWindow();
  return consumeStoredHubCallback();
}

function captureHubRedirectFromLocation() {
  if (capturedHubRedirectRead) return;
  capturedHubRedirectRead = true;
  capturedHubRedirect = readAndClearHubRedirectHash();
}

export function persistSelectedHubAddress(address: string, accountLabel?: string) {
  if (typeof window === "undefined") return;
  const identity = currentHubAuthIdentity();
  window.sessionStorage.setItem(SELECTED_HUB_ADDRESS_KEY, address);
  if (identity) window.sessionStorage.setItem(SELECTED_HUB_NETWORK_KEY, hubAuthIdentityKey(identity));
  if (accountLabel) window.sessionStorage.setItem(SELECTED_HUB_LABEL_KEY, accountLabel);
  markHubUiIntent();
}

export function readSelectedHubAddress(): string | null {
  if (typeof window === "undefined") return null;
  discardStaleHubAuthState();
  return window.sessionStorage.getItem(SELECTED_HUB_ADDRESS_KEY);
}

export function readSelectedHubAccountLabel(): string | undefined {
  if (typeof window === "undefined") return undefined;
  return window.sessionStorage.getItem(SELECTED_HUB_LABEL_KEY) || undefined;
}

export function clearSelectedHubAddress() {
  if (typeof window === "undefined") return;
  window.sessionStorage.removeItem(SELECTED_HUB_ADDRESS_KEY);
  window.sessionStorage.removeItem(SELECTED_HUB_LABEL_KEY);
  window.sessionStorage.removeItem(SELECTED_HUB_NETWORK_KEY);
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  return Boolean(value && typeof value === "object" && "error" in value);
}

function isCancellationText(value: string) {
  const normalized = value.toLowerCase();
  return (
    normalized.includes("cancel") ||
    normalized.includes("reject") ||
    normalized.includes("denied") ||
    normalized.includes("permission") ||
    normalized.includes("abort") ||
    normalized.includes("closed")
  );
}

function publicHubReturnUrl() {
  if (typeof window === "undefined") return "";
  try {
    const url = new URL(nimiqHubReturnUrl());
    for (const key of [...url.searchParams.keys()]) {
      if (/token|secret|signature|jwt|cookie|key/i.test(key)) url.searchParams.delete(key);
    }
    url.hash = "";
    return url.toString();
  } catch {
    return window.location.pathname;
  }
}

export function hubAuthFailureDiagnostics(
  stage: AuthStage,
  error: unknown,
  phase: AuthPhase,
): HubAuthFailureDiagnostics {
  const network = resolveNimiqAuthConfig();
  const original = error instanceof Error ? { name: error.name, message: error.message } : { name: "Unknown", message: String(error) };
  const meta = peekHubCallbackMeta();
  const continuation = typeof window !== "undefined" ? readHubAuthContinuation() : null;
  const cause = error instanceof NimiqAuthError && error.cause instanceof Error
    ? `${error.cause.name}: ${error.cause.message}`
    : original.name === "NimiqAuthError"
      ? original.message
      : `${original.name}: ${original.message}`;
  return {
    phase,
    name: original.name,
    message: original.message,
    cause,
    pathname: typeof window !== "undefined" ? window.location.pathname : "",
    hashPresent: meta.hashPresent,
    storagePresent: meta.storagePresent,
    callbackOperation: meta.operation,
    hubEndpoint: network.ok ? network.hubEndpoint : "unconfigured",
    network: network.ok ? network.network : "",
    environment: network.ok ? network.environment : "",
    returnPath: continuation?.returnPath || publicHubReturnUrl(),
    ...(typeof window !== "undefined" && readSelectedHubAddress()
      ? { walletAddress: redactWalletAddress(readSelectedHubAddress() as string) }
      : {}),
  };
}

function logHubAuthFailure(stage: AuthStage, error: unknown, phase: AuthPhase) {
  if (PUBLIC_NODE_ENV === "production") return;
  console.info("[auth] hub failure", hubAuthFailureDiagnostics(stage, error, phase));
}

function wrapWalletError(stage: AuthStage, error: unknown, phase: AuthPhase = phaseFromAuthStage(stage)): NimiqAuthError {
  if (error instanceof NimiqAuthError) return error;
  if (error instanceof AuthApiError) {
    const cancelled = error.status === 401 && isCancellationText(error.message);
    const code = error.code || (error.status === 410 ? "challenge_expired" : error.status === 409 ? "challenge_already_used" : "auth_api_error");
    const message = cancelled ? "" : userMessageForAuthPhase(phase, code);
    return new NimiqAuthError(stage, code, message, cancelled, error, phase);
  }
  const text = error instanceof Error ? `${error.name} ${error.message}` : String(error);
  if (isCancellationText(text)) {
    return new NimiqAuthError(stage, "wallet_cancelled", "", true, error, phase);
  }
  if (text.toLowerCase().includes("popup")) {
    return new NimiqAuthError(stage, "popup_blocked", userMessageForAuthPhase(phase, "popup_blocked"), false, error, phase);
  }
  if (text.toLowerCase().includes("invalid request")) {
    return new NimiqAuthError(
      stage,
      "hub_invalid_request",
      "Could not open the Nimiq wallet.",
      false,
      error,
      phase === "sign-challenge" || phase === "restore-signature" ? phase : "choose-address",
    );
  }
  const restorePhase = isHistoryStateCloneError(error)
    ? (phase === "sign-challenge" || phase === "restore-signature" ? "restore-signature" : "restore-address")
    : phase;
  if (isHistoryStateCloneError(error)) {
    logHubAuthFailure(stage, error, restorePhase);
    return new NimiqAuthError(stage, "hub_restore_failed", userMessageForAuthPhase(restorePhase), false, error, restorePhase);
  }
  logHubAuthFailure(stage, error, restorePhase);
  return new NimiqAuthError(stage, "wallet_unavailable", userMessageForAuthPhase(restorePhase), false, error, restorePhase);
}

export function selectNimiqAuthTransport(host: { hasNimiqPay: boolean; hasNimiqProvider: boolean; hostLanguage?: string }): NimiqAuthTransport {
  return isMiniAppHost(host) ? "mini-app" : "hub";
}

export function isMiniAppHost(host: { hasNimiqPay: boolean; hasNimiqProvider: boolean; hostLanguage?: string }) {
  return host.hasNimiqPay || host.hasNimiqProvider || host.hostLanguage !== undefined;
}

export function detectNimiqAuthTransport(): NimiqAuthTransport {
  if (typeof window === "undefined") return isNimiqHubEnabled() ? "hub" : "mini-app";
  const detected = selectNimiqAuthTransport({
    hasNimiqPay: Boolean(window.nimiqPay),
    hasNimiqProvider: Boolean(window.nimiq),
    hostLanguage: getHostLanguage(),
  });
  if (detected === "hub" && !isNimiqHubEnabled()) return "mini-app";
  return detected;
}

export async function requestMiniAppWallet(initialize: typeof init = init): Promise<MiniAppWallet> {
  try {
    console.info("[auth] transport=mini-app");
    const provider = await initialize({ timeout: 10_000 });
    console.info("[auth] provider initialized");
    const result = await provider.listAccounts();
    if (isErrorResponse(result)) {
      const text = `${result.error.type} ${result.error.message}`;
      throw new NimiqAuthError("requesting-wallet", result.error.type, result.error.message || "Nimiq Pay account access failed.", isCancellationText(text));
    }
    if (result.length === 0) throw new NimiqAuthError("requesting-wallet", "empty_account_list", "Nimiq Pay returned no accounts.");
    return { transport: "mini-app", provider, accounts: result };
  } catch (error) {
    throw wrapWalletError("requesting-wallet", error, "choose-address");
  }
}

export async function authenticateMiniApp(
  wallet: MiniAppWallet,
  address: string,
  api = { createChallenge: createNimiqChallenge, verifyChallenge: verifyNimiqChallenge },
): Promise<AuthSession> {
  const canonicalAddress = wallet.accounts.find((account) => compactAddress(account) === compactAddress(address));
  if (!canonicalAddress) throw new NimiqAuthError("selecting-account", "account_not_approved", "The selected account was not approved by Nimiq Pay.");
  console.info("[auth] account selected");
  let challenge: NimiqChallenge;
  try {
    challenge = await api.createChallenge(canonicalAddress, "mini-app");
  } catch (error) {
    throw wrapWalletError("requesting-challenge", error, "request-challenge");
  }
  console.info("[auth] challenge created");
  let result: SignatureResult | ErrorResponse;
  try {
    result = await wallet.provider.sign(challenge.message);
  } catch (error) {
    throw wrapWalletError("requesting-signature", error, "sign-challenge");
  }
  if (isErrorResponse(result)) {
    const text = `${result.error.type} ${result.error.message}`;
    throw new NimiqAuthError("requesting-signature", result.error.type, result.error.message || "The Nimiq signature could not be obtained.", isCancellationText(text));
  }
  console.info("[auth] signature received");
  return submitVerification(challenge, normalizeHex(result.publicKey, 32, "public key"), normalizeHex(result.signature, 64, "signature"), api.verifyChallenge);
}

export function persistPendingHubAuthentication(pending: PendingHubAuthentication) {
  if (typeof window === "undefined") return;
  const identity = currentHubAuthIdentity();
  if (!identity) {
    window.sessionStorage.removeItem(PENDING_HUB_KEY);
    return;
  }
  const stored: StoredPendingHubAuthentication = {
    ...pending,
    v: HUB_AUTH_STATE_VERSION,
    network: identity.network,
    environment: identity.environment,
    hubEndpoint: identity.hubEndpoint,
  };
  window.sessionStorage.setItem(PENDING_HUB_KEY, JSON.stringify(stored));
  persistHubAuthContinuation({
    phase: "sign-challenge",
    addressSelected: true,
  });
  markHubUiIntent();
}

export function readPendingHubAuthentication(): PendingHubAuthentication | null {
  if (typeof window === "undefined") return null;
  discardStaleHubAuthState();
  const stored = window.sessionStorage.getItem(PENDING_HUB_KEY);
  if (!stored) return null;
  try {
    const pending = JSON.parse(stored) as StoredPendingHubAuthentication;
    const identity = currentHubAuthIdentity();
    if (!isCurrentPendingHubAuthentication(pending, identity)) {
      window.sessionStorage.removeItem(PENDING_HUB_KEY);
      return null;
    }
    return {
      transport: "hub",
      address: pending.address,
      ...(pending.accountLabel ? { accountLabel: pending.accountLabel } : {}),
      challenge: pending.challenge,
    };
  } catch {
    window.sessionStorage.removeItem(PENDING_HUB_KEY);
    return null;
  }
}

export function clearPendingHubAuthentication() {
  if (typeof window === "undefined") return;
  window.sessionStorage.removeItem(PENDING_HUB_KEY);
  clearSelectedHubAddress();
  clearHubAuthContinuation();
}

export function persistHubAuthContinuation(patch: Partial<Pick<HubAuthContinuation, "phase" | "addressSelected" | "returnPath">> = {}) {
  if (typeof window === "undefined") return;
  const identity = currentHubAuthIdentity();
  if (!identity) {
    clearHubAuthContinuation();
    return;
  }
  const existing = readHubAuthContinuation();
  const next: HubAuthContinuation = {
    v: HUB_AUTH_CONTINUATION_VERSION,
    phase: patch.phase ?? existing?.phase ?? "idle",
    returnPath: patch.returnPath ?? existing?.returnPath ?? currentReturnPath(),
    addressSelected: patch.addressSelected ?? existing?.addressSelected ?? false,
    network: identity.network,
    environment: identity.environment,
    hubEndpoint: identity.hubEndpoint,
  };
  window.sessionStorage.setItem(NIMIQ_HUB_CONTINUATION_KEY, JSON.stringify(next));
}

export function readHubAuthContinuation(): HubAuthContinuation | null {
  if (typeof window === "undefined") return null;
  const stored = window.sessionStorage.getItem(NIMIQ_HUB_CONTINUATION_KEY);
  if (!stored) return null;
  try {
    const continuation = JSON.parse(stored) as HubAuthContinuation;
    if (
      continuation.v !== HUB_AUTH_CONTINUATION_VERSION ||
      typeof continuation.phase !== "string" ||
      typeof continuation.returnPath !== "string"
    ) {
      window.sessionStorage.removeItem(NIMIQ_HUB_CONTINUATION_KEY);
      return null;
    }
    return continuation;
  } catch {
    window.sessionStorage.removeItem(NIMIQ_HUB_CONTINUATION_KEY);
    return null;
  }
}

function currentReturnPath() {
  if (typeof window === "undefined") return "/";
  const pathname = window.location?.pathname || "/";
  const search = window.location?.search || "";
  return `${pathname}${search}`;
}

export function markHubUiIntent() {
  if (typeof window === "undefined") return;
  window.sessionStorage.setItem(HUB_UI_KEY, "open");
}

export function hasHubUiIntent() {
  return typeof window !== "undefined" && window.sessionStorage.getItem(HUB_UI_KEY) === "open";
}

export function clearHubUiIntent() {
  if (typeof window === "undefined") return;
  window.sessionStorage.removeItem(HUB_UI_KEY);
}

export async function createHubChallenge(
  address: string,
  createChallenge = createNimiqChallenge,
  accountLabel?: string,
): Promise<PendingHubAuthentication> {
  const existing = readPendingHubAuthentication();
  const label = accountLabel || readSelectedHubAccountLabel();
  if (existing && compactAddress(existing.address) === compactAddress(address)) {
    if (label && !existing.accountLabel) {
      existing.accountLabel = label;
      persistPendingHubAuthentication(existing);
    }
    return existing;
  }
  const key = compactAddress(address);
  const inFlight = challengeInFlight.get(key);
  if (inFlight) return inFlight;
  const pendingPromise = (async () => {
    try {
      const challenge = await createChallenge(address, "hub");
      const pending: PendingHubAuthentication = { transport: "hub", address, accountLabel: label, challenge };
      persistPendingHubAuthentication(pending);
      persistSelectedHubAddress(address, label);
      console.info("[auth] challenge created");
      return pending;
    } catch (error) {
      throw wrapWalletError("requesting-challenge", error, "request-challenge");
    }
  })();
  challengeInFlight.set(key, pendingPromise);
  return pendingPromise.finally(() => challengeInFlight.delete(key));
}

// Browser Hub uses RedirectRequestBehavior. Popup postMessage times out on the
// current Testnet Hub whenever window.opener is missing, which renders
// /request-error. Redirect encodes chooseAddress/signMessage in the Hub URL so
// the request exists before the page loads. Challenge retrieval stays between
// the two user gestures and is persisted in sessionStorage across the return.
export async function beginHubAuthentication(
  client: HubChooseAddressClient = getHubApi() as HubChooseAddressClient,
  createChallenge = createNimiqChallenge,
): Promise<PendingHubAuthentication | undefined> {
  const network = resolveNimiqAuthConfig();
  if (!network.ok) {
    throw new NimiqAuthError("detecting-environment", network.code, network.message);
  }
  console.info(`[auth] transport=hub network=${network.network} hub=${network.hubEndpoint}`);
  markHubUiIntent();
  persistHubAuthContinuation({ phase: "choose-address", addressSelected: false, returnPath: currentReturnPath() });
  captureHubRedirectFromLocation();
  if (capturedHubRedirect?.type === "address") {
    console.info("[auth] address selected");
    persistHubAuthContinuation({ phase: "request-challenge", addressSelected: true });
    return createHubChallenge(capturedHubRedirect.address, createChallenge, capturedHubRedirect.label);
  }
  const existing = readPendingHubAuthentication();
  if (existing) return existing;
  const behavior = typeof window === "undefined" ? undefined : hubRedirectBehavior({ phase: "choose-address" });
  let selected: { address: string; label?: string; meta?: { account?: { label?: string } } } | void;
  try {
    selected = await client.chooseAddress({ appName: APP_NAME }, behavior);
  } catch (error) {
    throw wrapWalletError("requesting-wallet", error, "choose-address");
  }
  if (!selected?.address) return undefined;
  console.info("[auth] address selected");
  persistHubAuthContinuation({ phase: "request-challenge", addressSelected: true });
  return createHubChallenge(selected.address, createChallenge, hubAccountLabel(selected));
}

export async function completeHubAuthentication(
  pending: PendingHubAuthentication,
  client: HubSignMessageClient = getHubApi() as HubSignMessageClient,
  verifyChallenge = verifyNimiqChallenge,
): Promise<AuthSession | undefined> {
  persistPendingHubAuthentication(pending);
  persistSelectedHubAddress(pending.address, pending.accountLabel);
  const behavior = typeof window === "undefined" ? undefined : hubRedirectBehavior({ phase: "sign-message" });
  if (behavior && hubSignRedirectStarted) return undefined;
  if (behavior) hubSignRedirectStarted = true;
  let signed: SignedMessage | void;
  try {
    signed = await client.signMessage(
      { appName: APP_NAME, signer: pending.address, message: pending.challenge.message },
      behavior,
    );
  } catch (error) {
    hubSignRedirectStarted = false;
    throw wrapWalletError("requesting-signature", error, "sign-challenge");
  }
  if (!signed) return undefined;
  return finishHubSignature(pending, signed, verifyChallenge);
}

export async function finishHubSignature(
  pending: PendingHubAuthentication,
  signed: SignedMessage,
  verifyChallenge = verifyNimiqChallenge,
): Promise<AuthSession> {
  if (compactAddress(signed.signer) !== compactAddress(pending.address)) {
    throw new NimiqAuthError("verifying-signature", "wallet_address_mismatch", "The Hub signed with a different wallet.");
  }
  if (verificationInFlight) return verificationInFlight;
  console.info("[auth] signature received");
  verificationInFlight = submitVerification(
    pending.challenge,
    toHex(signed.signerPublicKey, 32, "public key"),
    toHex(signed.signature, 64, "signature"),
    verifyChallenge,
    pending.accountLabel,
  ).then((session) => {
    clearPendingHubAuthentication();
    clearHubUiIntent();
    return session;
  });
  try {
    return await verificationInFlight;
  } finally {
    verificationInFlight = null;
  }
}

export function consumeHubRedirect(client: HubRedirectClient = getHubApi()): Promise<HubRedirectResult> {
  redirectWaiter ??= (async () => {
    captureHubRedirectFromLocation();
    if (capturedHubRedirect) return capturedHubRedirect;

    return new Promise<HubRedirectResult>((resolve) => {
      let settled = false;
      const finish = (result: HubRedirectResult) => {
        if (settled) return;
        settled = true;
        if (result.type === "address") persistSelectedHubAddress(result.address, result.label);
        resolve(result);
      };
      client.on(HubApi.RequestType.CHOOSE_ADDRESS, (result) => {
        finish({ type: "address", address: result.address, ...(hubAccountLabel(result) ? { label: hubAccountLabel(result) } : {}) });
      }, (error) => finish({ type: "error", error: wrapWalletError("requesting-wallet", error, "restore-address"), stage: "requesting-wallet", phase: "restore-address" }));
      client.on(HubApi.RequestType.SIGN_MESSAGE, (result) => {
        finish({ type: "signature", signed: result });
      }, (error) => finish({ type: "error", error: wrapWalletError("requesting-signature", error, "restore-signature"), stage: "requesting-signature", phase: "restore-signature" }));
      client.on(HubApi.RequestType.CHECKOUT, (result) => {
        finish(interpretHubRpcResult(HubApi.RequestType.CHECKOUT, "ok", result));
      }, (error) => finish({ type: "checkout-error", error }));
      void client.checkRedirectResponse()
        .then(() => {
          queueMicrotask(() => finish({ type: "empty" }));
        })
        .catch((error) => {
          const selected = readSelectedHubAddress();
          if (selected) {
            finish({ type: "address", address: selected, ...(readSelectedHubAccountLabel() ? { label: readSelectedHubAccountLabel() } : {}) });
            return;
          }
          const pending = readPendingHubAuthentication();
          const phase: AuthPhase = pending ? "restore-signature" : "restore-address";
          const stage: AuthStage = pending ? "requesting-signature" : "requesting-wallet";
          finish({ type: "error", error: wrapWalletError(stage, error, phase), stage, phase });
        });
    });
  })();
  return redirectWaiter;
}

export async function takeHubRedirectResult(client?: HubRedirectClient): Promise<HubRedirectResult> {
  return consumeHubRedirect(client ?? getHubApi());
}

export type HubAuthResumeDeps = {
  restoreSession?: () => Promise<AuthSession | null>;
  takeRedirect?: () => Promise<HubRedirectResult>;
  createChallenge?: typeof createNimiqChallenge;
  verifyChallenge?: typeof verifyNimiqChallenge;
  signClient?: HubSignMessageClient;
  autoStartSignature?: boolean;
};

async function continueAfterAddress(
  address: string,
  label: string | undefined,
  deps: HubAuthResumeDeps,
): Promise<HubAuthResumeResult> {
  persistHubAuthContinuation({ phase: "request-challenge", addressSelected: true });
  let pending: PendingHubAuthentication;
  try {
    pending = await createHubChallenge(address, deps.createChallenge, label);
  } catch (error) {
    return { type: "error", error: wrapWalletError("requesting-challenge", error, "request-challenge"), phase: "request-challenge" };
  }
  persistHubAuthContinuation({ phase: "sign-challenge", addressSelected: true });
  if (deps.autoStartSignature === false) {
    return { type: "awaiting-signature", pending, phase: "sign-challenge" };
  }
  try {
    const session = await completeHubAuthentication(pending, deps.signClient, deps.verifyChallenge);
    if (session) return { type: "authenticated", session, phase: "restore-session" };
    return { type: "redirecting", phase: "sign-challenge", pending };
  } catch (error) {
    return { type: "error", error: wrapWalletError("requesting-signature", error, "sign-challenge"), phase: "sign-challenge" };
  }
}

async function doResumeHubAuthFlow(deps: HubAuthResumeDeps): Promise<HubAuthResumeResult> {
  prepareNimiqHub();
  try {
    const restored = await (deps.restoreSession ?? restoreAuthSession)();
    if (restored) return { type: "authenticated", session: restored, phase: "restore-session" };
  } catch (error) {
    return { type: "error", error: wrapWalletError("restoring-session", error, "restore-session"), phase: "restore-session" };
  }

  let redirected: HubRedirectResult;
  try {
    redirected = await (deps.takeRedirect ?? takeHubRedirectResult)();
  } catch (error) {
    const selected = readSelectedHubAddress();
    const wrappedPhase: AuthPhase = readPendingHubAuthentication() ? "restore-signature" : "restore-address";
    const wrapped = wrapWalletError(
      readPendingHubAuthentication() ? "requesting-signature" : "requesting-wallet",
      error,
      wrappedPhase,
    );
    if (wrapped.cancelled) return { type: "cancelled", phase: wrapped.phase };
    if (selected && wrappedPhase === "restore-address") {
      return continueAfterAddress(selected, readSelectedHubAccountLabel(), deps);
    }
    return { type: "error", error: wrapped, phase: wrapped.phase };
  }

  if (isHubPaymentRedirect(redirected)) return { type: "payment", phase: "idle" };

  if (redirected.type === "error") {
    const error = wrapWalletError(redirected.stage, redirected.error, redirected.phase ?? phaseFromAuthStage(redirected.stage, true));
    if (error.cancelled) return { type: "cancelled", phase: error.phase };
    return { type: "error", error, phase: error.phase };
  }

  if (redirected.type === "address") {
    return continueAfterAddress(redirected.address, redirected.label, deps);
  }

  if (redirected.type === "signature") {
    const pending = readPendingHubAuthentication();
    if (!pending) {
      return {
        type: "error",
        error: new NimiqAuthError(
          "verifying-signature",
          "missing_hub_challenge",
          userMessageForAuthPhase("restore-signature"),
          false,
          undefined,
          "restore-signature",
        ),
        phase: "restore-signature",
      };
    }
    try {
      const session = await finishHubSignature(pending, redirected.signed, deps.verifyChallenge);
      return { type: "authenticated", session, phase: "restore-session" };
    } catch (error) {
      return { type: "error", error: wrapWalletError("verifying-signature", error, "verify-challenge"), phase: "verify-challenge" };
    }
  }

  const pending = readPendingHubAuthentication();
  const continuation = readHubAuthContinuation();
  if (pending && continuation?.phase === "sign-challenge" && continuation.addressSelected && deps.autoStartSignature !== false) {
    try {
      const session = await completeHubAuthentication(pending, deps.signClient, deps.verifyChallenge);
      if (session) return { type: "authenticated", session, phase: "restore-session" };
      return { type: "redirecting", phase: "sign-challenge", pending };
    } catch (error) {
      return { type: "error", error: wrapWalletError("requesting-signature", error, "sign-challenge"), phase: "sign-challenge" };
    }
  }
  if (pending) return { type: "awaiting-signature", pending, phase: "sign-challenge" };

  const selected = readSelectedHubAddress();
  if (selected) return continueAfterAddress(selected, readSelectedHubAccountLabel(), deps);

  return { type: "idle", phase: "idle" };
}

export function resumeHubAuthFlow(deps: HubAuthResumeDeps = {}): Promise<HubAuthResumeResult> {
  hubAuthResume ??= doResumeHubAuthFlow(deps);
  return hubAuthResume;
}

export function resetHubRedirectConsumption() {
  redirectWaiter = null;
  capturedHubRedirect = null;
  capturedHubRedirectRead = false;
  hubSignRedirectStarted = false;
  verificationInFlight = null;
  challengeInFlight.clear();
  hubApi = null;
  hubApiEndpoint = null;
  hubReturnRestoreClaimed = false;
  hubAuthResume = null;
  hubCallbackConsumeCount = 0;
}

export function claimHubReturnRestore() {
  if (hubReturnRestoreClaimed) return false;
  hubReturnRestoreClaimed = true;
  return true;
}

export function releaseHubReturnRestore() {
  // Restore ownership is process-scoped. Releasing on unmount allowed a second
  // NimiqConnect to consume the same Hub callback. Tests still reset via resetHubRedirectConsumption.
}

async function submitVerification(
  challenge: NimiqChallenge,
  publicKey: string,
  signature: string,
  verify: typeof verifyNimiqChallenge,
  accountLabel?: string,
) {
  try {
    const session = await verify({
      challenge_id: challenge.challenge_id,
      message: challenge.message,
      public_key: publicKey,
      signature,
      ...(accountLabel ? { account_label: accountLabel } : {}),
    });
    console.info("[auth] verification succeeded");
    return session;
  } catch (error) {
    throw wrapWalletError("verifying-signature", error, "verify-challenge");
  }
}

export function bytesToHex(value: Uint8Array) {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export function normalizeHex(value: string, byteLength: number, label: string) {
  const normalized = value.trim().toLowerCase().replace(/^0x/, "");
  if (!new RegExp(`^[0-9a-f]{${byteLength * 2}}$`).test(normalized)) {
    throw new NimiqAuthError("verifying-signature", "signature_representation_error", `Invalid ${label} representation.`);
  }
  return normalized;
}

export function toHex(value: Uint8Array | number[] | string, byteLength: number, label: string) {
  if (typeof value === "string") return normalizeHex(value, byteLength, label);
  if (value instanceof Uint8Array) return normalizeHex(bytesToHex(value), byteLength, label);
  if (Array.isArray(value)) return normalizeHex(bytesToHex(Uint8Array.from(value)), byteLength, label);
  throw new NimiqAuthError("verifying-signature", "signature_representation_error", `Invalid ${label} representation.`);
}

function compactAddress(address: string) {
  return address.replace(/\s/g, "").toUpperCase();
}
