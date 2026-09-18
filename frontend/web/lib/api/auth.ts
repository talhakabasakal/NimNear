import { apiBaseUrl } from "./events";
import { resolveNimiqAuthConfig } from "@/lib/auth/nimiq-network";

export type AuthUser = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  display_name?: string;
  wallet_address?: string;
  status: string;
  created_at: string;
};

export type AuthSession = { token?: string; user: AuthUser };
export type NimiqTransport = "mini-app" | "hub";
export type NimiqChallenge = {
  challenge_id: string;
  message: string;
  wallet_address: string;
  network: string;
  environment: string;
  purpose: "AUTH_LOGIN";
  transport: NimiqTransport;
  issued_at: string;
  expires_at: string;
};

type VerifyNimiqInput = { challenge_id: string; message: string; public_key: string; signature: string; account_label?: string };

const AUTH_NETWORK_DETAIL_KEYS = [
  "requested_network",
  "requested_environment",
  "expected_network",
  "expected_environment",
] as const;

type AuthErrorPayload = {
  message?: string;
  error?: string;
  error_code?: string;
  details?: Partial<Record<(typeof AUTH_NETWORK_DETAIL_KEYS)[number], string>>;
};

export class AuthApiError extends Error {
  constructor(public readonly status: number, message = "Authentication request failed", public readonly code = "") {
    super(message); this.name = "AuthApiError";
  }
}

export function isLostSessionStatus(status: number) {
  return status === 401 || status === 404;
}

const sessionKey = "nimnear.auth.session";

async function throwAuthApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as AuthErrorPayload | null;
  throw new AuthApiError(response.status, formatAuthApiErrorMessage(payload), payload?.error_code ?? "");
}

function formatAuthApiErrorMessage(payload: AuthErrorPayload | null) {
  const base = payload?.message ?? payload?.error ?? "Authentication request failed";
  const extras = AUTH_NETWORK_DETAIL_KEYS
    .map((key) => {
      const value = payload?.details?.[key]?.trim();
      return value ? `${key}=${value}` : "";
    })
    .filter(Boolean);
  if (extras.length === 0 || extras.every((part) => base.includes(part))) return base;
  return `${base} (${extras.join(" ")})`;
}

export async function createNimiqChallenge(walletAddress: string, transport: NimiqTransport): Promise<NimiqChallenge> {
  const network = resolveNimiqAuthConfig();
  if (!network.ok) {
    throw new AuthApiError(500, network.message, network.code);
  }
  const response = await fetch(new URL("/api/v1/auth/nimiq/challenges", apiBaseUrl), {
    method: "POST", credentials: "include", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ wallet_address: walletAddress, network: network.network, environment: network.environment, purpose: "AUTH_LOGIN", transport }),
  });
  if (!response.ok) await throwAuthApiError(response);
  const challenge = (await response.json()) as NimiqChallenge;
  if (!challenge.challenge_id || !challenge.message || challenge.wallet_address !== walletAddress && challenge.wallet_address.replace(/\s/g, "") !== walletAddress.replace(/\s/g, "")) {
    throw new AuthApiError(502, "The server returned an invalid authentication message.", "malformed_challenge_response");
  }
  if (challenge.network !== network.network || challenge.environment !== network.environment) {
    throw new AuthApiError(
      502,
      `The server returned a Nimiq network that does not match this client (requested_network=${network.network} requested_environment=${network.environment} returned_network=${challenge.network} returned_environment=${challenge.environment}).`,
      "nimiq_network_mismatch",
    );
  }
  return challenge;
}

export async function verifyNimiqChallenge(input: VerifyNimiqInput): Promise<AuthSession> {
  const response = await fetch(new URL("/api/v1/auth/nimiq/verify", apiBaseUrl), { method: "POST", credentials: "include", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) });
  if (!response.ok) await throwAuthApiError(response);
  const session = (await response.json()) as AuthSession;
  return { user: session.user };
}

export async function fetchCurrentUser(token?: string): Promise<AuthUser> {
  const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {};
  const response = await fetch(new URL("/api/v1/me", apiBaseUrl), { headers, credentials: "include", cache: "no-store" });
  if (!response.ok) await throwAuthApiError(response);
  return (await response.json()) as AuthUser;
}

export async function restoreAuthSession(): Promise<AuthSession | null> {
  try {
    const user = await fetchCurrentUser();
    const session = { user };
    writeAuthSession(session);
    return session;
  } catch (error) {
    if (error instanceof AuthApiError && isLostSessionStatus(error.status)) clearAuthSession();
    return null;
  }
}

export async function logout() {
  const response = await fetch(new URL("/api/v1/auth/logout", apiBaseUrl), { method: "POST", credentials: "include" });
  if (!response.ok) await throwAuthApiError(response);
}

export async function deleteAccount() {
  const response = await fetch(new URL("/api/v1/me", apiBaseUrl), { method: "DELETE", credentials: "include" });
  if (!response.ok) await throwAuthApiError(response);
}

export function readAuthSession(): AuthSession | null {
  if (typeof window === "undefined") return null;
  const stored = window.sessionStorage.getItem(sessionKey);
  if (!stored) return null;
  try {
    const session = JSON.parse(stored) as Partial<AuthSession>;
    if (!session.user) return null;
    return { user: session.user };
  } catch { window.sessionStorage.removeItem(sessionKey); return null; }
}
export function writeAuthSession(session: AuthSession) {
  window.sessionStorage.setItem(sessionKey, JSON.stringify({ user: session.user }));
  window.dispatchEvent(new Event("nimnear-auth-changed"));
}
export function clearAuthSession() {
  window.sessionStorage.removeItem(sessionKey);
  window.dispatchEvent(new Event("nimnear-auth-changed"));
}
export function accountDisplayName(user: Pick<AuthUser, "email" | "first_name" | "last_name"> & { display_name?: string; wallet_address?: string }) {
  const combinedName = `${user.first_name ?? ""} ${user.last_name ?? ""}`.trim();
  return [user.display_name, combinedName, user.wallet_address, user.email].map((value) => value?.trim()).find(Boolean) || "NIMNear user";
}
