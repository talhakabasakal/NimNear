import { apiBaseUrl } from "./events";

export type AuthUser = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  status: string;
  created_at: string;
};

type LoginResponse = {
  token: string;
  user: AuthUser;
};

type RegisterInput = {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
};

type LoginInput = {
  email: string;
  password: string;
};

export type AuthSession = {
  token: string;
  user: AuthUser;
};

export class AuthApiError extends Error {
  constructor(public readonly status: number, message = "Authentication request failed") {
    super(message);
    this.name = "AuthApiError";
  }
}

const sessionKey = "nimnear.auth.session";

async function throwAuthApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as { message?: string; error?: string } | null;
  throw new AuthApiError(response.status, payload?.message ?? payload?.error ?? "Authentication request failed");
}

export async function login(input: LoginInput): Promise<AuthSession> {
  const response = await fetch(new URL("/api/v1/auth/login", apiBaseUrl), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });

  if (!response.ok) await throwAuthApiError(response);
  return (await response.json()) as LoginResponse;
}

export async function register(input: RegisterInput): Promise<AuthUser> {
  const response = await fetch(new URL("/api/v1/auth/register", apiBaseUrl), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });

  if (!response.ok) await throwAuthApiError(response);
  return (await response.json()) as AuthUser;
}

export async function fetchCurrentUser(token: string): Promise<AuthUser> {
  const response = await fetch(new URL("/api/v1/me", apiBaseUrl), {
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!response.ok) await throwAuthApiError(response);
  return (await response.json()) as AuthUser;
}

export function readAuthSession(): AuthSession | null {
  if (typeof window === "undefined") return null;

  const stored = window.sessionStorage.getItem(sessionKey);
  if (!stored) return null;

  try {
    const session = JSON.parse(stored) as Partial<AuthSession>;
    if (!session.token || !session.user) return null;
    return session as AuthSession;
  } catch {
    window.sessionStorage.removeItem(sessionKey);
    return null;
  }
}

export function writeAuthSession(session: AuthSession) {
  window.sessionStorage.setItem(sessionKey, JSON.stringify(session));
}

export function clearAuthSession() {
  window.sessionStorage.removeItem(sessionKey);
}
