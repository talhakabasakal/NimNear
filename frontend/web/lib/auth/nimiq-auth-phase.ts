export const NIMIQ_HUB_CALLBACK_STORAGE_KEY = "nimnear.auth.hub.rpcCallback";
export const NIMIQ_HUB_CONTINUATION_KEY = "nimnear.auth.hub.continuation";
export const HUB_AUTH_CONTINUATION_VERSION = 1;

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

export type AuthPhase =
  | "idle"
  | "choose-address"
  | "restore-address"
  | "request-challenge"
  | "sign-challenge"
  | "restore-signature"
  | "verify-challenge"
  | "restore-session";

export type HubCallbackOperation = "choose-address" | "sign-message" | "checkout" | "unknown";

export type HubAuthContinuation = {
  v: number;
  phase: AuthPhase;
  returnPath: string;
  addressSelected: boolean;
  network: string;
  environment: string;
  hubEndpoint: string;
};

export type StoredHubRpcCallback = {
  v: number;
  id: string;
  status: string;
  result: string;
  pathname: string;
  search: string;
};

export type HubCallbackMeta = {
  hashPresent: boolean;
  storagePresent: boolean;
  operation: HubCallbackOperation | null;
  status: "ok" | "error" | null;
  pathname: string;
};

export type HubAuthFailureDiagnostics = {
  phase: AuthPhase;
  name: string;
  message: string;
  cause: string;
  pathname: string;
  hashPresent: boolean;
  storagePresent: boolean;
  callbackOperation: HubCallbackOperation | null;
  hubEndpoint: string;
  network: string;
  environment: string;
  returnPath: string;
  walletAddress?: string;
};

export function phaseFromAuthStage(stage: AuthStage, restore = false): AuthPhase {
  switch (stage) {
    case "requesting-wallet":
    case "selecting-account":
      return restore ? "restore-address" : "choose-address";
    case "requesting-challenge":
      return "request-challenge";
    case "awaiting-signature":
    case "requesting-signature":
      return restore ? "restore-signature" : "sign-challenge";
    case "verifying-signature":
    case "creating-session":
      return "verify-challenge";
    case "restoring-session":
      return "restore-session";
    default:
      return "idle";
  }
}

export function userMessageForAuthPhase(phase: AuthPhase, code = ""): string {
  if (code === "popup_blocked") {
    return "The browser blocked the Hub window. Allow popups and try again.";
  }
  if (code === "hub_disabled") {
    return "Open NIMNear in Nimiq Pay to sign in. Browser Hub fallback is disabled.";
  }
  if (code === "browser_required") {
    return "Nimiq Hub is only available in a browser.";
  }
  switch (phase) {
    case "choose-address":
      return "Could not open the Nimiq wallet.";
    case "restore-address":
    case "restore-signature":
      return "Sign-in could not be resumed. Please try again.";
    case "sign-challenge":
      return "Wallet signature could not be completed.";
    case "request-challenge":
    case "verify-challenge":
    case "restore-session":
      return "Sign-in could not be verified.";
    default:
      return "Nimiq sign-in could not be completed.";
  }
}

export function redactWalletAddress(address: string): string {
  const compact = address.replace(/\s/g, "");
  if (compact.length <= 10) return "…";
  return `${compact.slice(0, 6)}…${compact.slice(-4)}`;
}

export function isHistoryStateCloneError(error: unknown): boolean {
  const text = error instanceof Error ? `${error.name} ${error.message}` : String(error);
  return /dataclone|could not be cloned|htmlobjectelement|history\.state/i.test(text);
}

export function callbackOperationFromCommand(command: string | null): HubCallbackOperation | null {
  if (command === "choose-address") return "choose-address";
  if (command === "sign-message") return "sign-message";
  if (command === "checkout") return "checkout";
  if (!command) return null;
  return "unknown";
}
