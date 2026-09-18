import HubApi, { type SignedMessage } from "@nimiq/hub-api";
import {
  getHostLanguage,
  init,
  type ErrorResponse,
  type NimiqProvider,
  type SignatureResult,
} from "@nimiq/mini-app-sdk";

import {
  AuthApiError,
  createNimiqChallenge,
  verifyNimiqChallenge,
  type AuthSession,
  type NimiqChallenge,
} from "@/lib/api/auth";
import { isNimiqHubEnabled, resolveNimiqAuthConfig } from "@/lib/auth/nimiq-network";

export {
  isNimiqHubEnabled,
  requireNimiqAuthConfig,
  resolveNimiqAuthConfig,
} from "@/lib/auth/nimiq-network";
export const NIMIQ_HUB_PUBLIC_METHODS = ["chooseAddress", "signMessage"] as const;
const APP_NAME = "NIMNear";
const PENDING_HUB_KEY = "nimnear.auth.hub.pending";
const SELECTED_HUB_ADDRESS_KEY = "nimnear.auth.hub.selectedAddress";
const SELECTED_HUB_LABEL_KEY = "nimnear.auth.hub.accountLabel";
const SELECTED_HUB_NETWORK_KEY = "nimnear.auth.hub.network";
const HUB_UI_KEY = "nimnear.auth.hub.ui";
const RPC_REQUESTS_KEY = "rpcRequests";
const HUB_AUTH_STATE_VERSION = 2;

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
export type AuthStage =
  | "idle"
  | "detecting-environment"
  | "requesting-wallet"
  | "selecting-account"
  | "requesting-challenge"
  | "awaiting-signature"
  | "requesting-signature"
  | "verifying-signature"
  | "creating-session"
  | "restoring-session"
  | "authenticated";

export class NimiqAuthError extends Error {
  constructor(
    public readonly stage: AuthStage,
    public readonly code: string,
    message: string,
    public readonly cancelled = false,
  ) {
    super(message);
    this.name = "NimiqAuthError";
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
  | { type: "error"; error: unknown; stage: AuthStage }
  | { type: "empty" };

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
  return new HubApi.RedirectRequestBehavior(hubReturnUrl(), localState);
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

function hubAccountLabel(value: { label?: string; meta?: { account?: { label?: string } } } | null | undefined): string | undefined {
  const label = value?.meta?.account?.label?.trim() || value?.label?.trim();
  return label || undefined;
}

function interpretHubRpcResult(command: string | null, status: "ok" | "error", result: unknown): HubRedirectResult {
  if (status === "error") {
    const stage: AuthStage = command === HubApi.RequestType.SIGN_MESSAGE ? "requesting-signature" : "requesting-wallet";
    const errorValue = result && typeof result === "object" && "message" in result
      ? new Error(String((result as { message: unknown }).message))
      : result;
    return { type: "error", error: wrapWalletError(stage, errorValue instanceof Error ? errorValue : new Error(typeof result === "string" ? result : "The Hub request was cancelled.")), stage };
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

function readAndClearHubRedirectHash(): HubRedirectResult | null {
  if (typeof window === "undefined") return null;
  const fragment = new URLSearchParams(window.location.hash.substring(1));
  const idRaw = fragment.get("id");
  const statusRaw = fragment.get("status");
  const resultRaw = fragment.get("result");
  if (!idRaw || !statusRaw || resultRaw == null) return null;
  const id = Number.parseInt(idRaw, 10);
  if (!Number.isFinite(id)) return null;
  let result: unknown;
  try {
    result = parseHubRpcJson(resultRaw);
  } catch {
    return null;
  }
  const interpreted = interpretHubRpcResult(
    commandForRpcId(id),
    statusRaw === "ok" ? "ok" : "error",
    result,
  );
  fragment.delete("id");
  fragment.delete("status");
  fragment.delete("result");
  const url = new URL(window.location.href);
  url.hash = fragment.toString();
  window.history.replaceState(window.history.state, "", url.href);
  return interpreted.type === "empty" ? null : interpreted;
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
  return normalized.includes("cancel") || normalized.includes("reject") || normalized.includes("denied") || normalized.includes("permission");
}

function configuredHubLabel() {
  const network = resolveNimiqAuthConfig();
  return network.ok ? network.hubLabel : "Nimiq Hub";
}

function wrapWalletError(stage: AuthStage, error: unknown): NimiqAuthError {
  if (error instanceof NimiqAuthError) return error;
  if (error instanceof AuthApiError) {
    const cancelled = error.status === 401 && isCancellationText(error.message);
    const code = error.code || (error.status === 410 ? "challenge_expired" : error.status === 409 ? "challenge_already_used" : "auth_api_error");
    return new NimiqAuthError(stage, code, error.message || "Authentication request failed.", cancelled);
  }
  const text = error instanceof Error ? `${error.name} ${error.message}` : String(error);
  if (isCancellationText(text)) return new NimiqAuthError(stage, "wallet_cancelled", "The wallet transaction was cancelled.", true);
  if (text.toLowerCase().includes("popup")) {
    return new NimiqAuthError(stage, "popup_blocked", "The browser blocked the Hub window. Allow popups and try again.");
  }
  if (text.toLowerCase().includes("invalid request")) {
    return new NimiqAuthError(stage, "hub_invalid_request", `The Nimiq Hub request is invalid. Try the ${configuredHubLabel()} redirect again.`);
  }
  return new NimiqAuthError(stage, "wallet_unavailable", "The Nimiq wallet could not be reached.");
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
    throw wrapWalletError("requesting-wallet", error);
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
    throw wrapWalletError("requesting-challenge", error);
  }
  console.info("[auth] challenge created");
  let result: SignatureResult | ErrorResponse;
  try {
    result = await wallet.provider.sign(challenge.message);
  } catch (error) {
    throw wrapWalletError("requesting-signature", error);
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
      throw wrapWalletError("requesting-challenge", error);
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
  captureHubRedirectFromLocation();
  if (capturedHubRedirect?.type === "address") {
    console.info("[auth] address selected");
    return createHubChallenge(capturedHubRedirect.address, createChallenge, capturedHubRedirect.label);
  }
  const existing = readPendingHubAuthentication();
  if (existing) return existing;
  const behavior = typeof window === "undefined" ? undefined : hubRedirectBehavior({ phase: "choose-address" });
  let selected: { address: string; label?: string; meta?: { account?: { label?: string } } } | void;
  try {
    selected = await client.chooseAddress({ appName: APP_NAME }, behavior);
  } catch (error) {
    throw wrapWalletError("requesting-wallet", error);
  }
  if (!selected?.address) return undefined;
  console.info("[auth] address selected");
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
    throw wrapWalletError("requesting-signature", error);
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
      }, (error) => finish({ type: "error", error: wrapWalletError("requesting-wallet", error), stage: "requesting-wallet" }));
      client.on(HubApi.RequestType.SIGN_MESSAGE, (result) => {
        finish({ type: "signature", signed: result });
      }, (error) => finish({ type: "error", error: wrapWalletError("requesting-signature", error), stage: "requesting-signature" }));
      void client.checkRedirectResponse()
        .then(() => {
          queueMicrotask(() => finish({ type: "empty" }));
        })
        .catch((error) => finish({ type: "error", error: wrapWalletError("requesting-wallet", error), stage: "requesting-wallet" }));
    });
  })();
  return redirectWaiter;
}

export async function takeHubRedirectResult(client?: HubRedirectClient): Promise<HubRedirectResult> {
  return consumeHubRedirect(client ?? getHubApi());
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
    throw wrapWalletError("verifying-signature", error);
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
