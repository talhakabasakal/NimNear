import { apiBaseUrl } from "./events";
import type { EventRecord } from "./events";
import { withAuthSession } from "./session-request";
import { userFacingApiMessage } from "./http-error";

export type ProfileRecord = {
  id: string;
  display_name: string;
  username: string | null;
  bio: string | null;
  avatar_url: string | null;
  wallet_address: string | null;
  joined_at: string;
  organized_event_count: number;
  attended_event_count: number;
};

export type UpdateProfileInput = {
  display_name?: string | null;
  username?: string | null;
  bio?: string | null;
};

type ProfileResponse = { data: ProfileRecord };
type ProfileEventsResponse = { data: EventRecord[] };
type ProfileErrorPayload = {
  message?: string;
  error?: string;
  error_code?: string;
};

export class ProfilesApiError extends Error {
  constructor(
    public readonly status: number,
    message = "Profile API returned " + status,
    public readonly errorCode?: string,
  ) {
    super(message);
    this.name = "ProfilesApiError";
  }
}

async function throwProfileApiError(response: Response): Promise<never> {
  const payload = (await response
    .json()
    .catch(() => null)) as ProfileErrorPayload | null;
  throw new ProfilesApiError(
    response.status,
    userFacingApiMessage(
      response.status,
      payload?.message ?? payload?.error,
      "Profile could not be loaded.",
    ),
    payload?.error_code,
  );
}

export async function fetchProfile(id: string): Promise<ProfileRecord> {
  const response = await fetch(
    new URL("/api/v1/profiles/" + encodeURIComponent(id), apiBaseUrl),
    { cache: "no-store" },
  );
  if (!response.ok) await throwProfileApiError(response);
  return ((await response.json()) as ProfileResponse).data;
}

export async function fetchProfileEvents(
  id: string,
  type: "organized" | "attended",
): Promise<EventRecord[]> {
  const url = new URL(
    "/api/v1/profiles/" + encodeURIComponent(id) + "/events",
    apiBaseUrl,
  );
  url.searchParams.set("type", type);
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) await throwProfileApiError(response);
  return ((await response.json()) as ProfileEventsResponse).data;
}

export async function updateProfile(
  input: UpdateProfileInput,
  token?: string,
): Promise<ProfileRecord> {
  const response = await fetch(
    new URL("/api/v1/me/profile", apiBaseUrl),
    withAuthSession(token, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    }),
  );
  if (!response.ok) await throwProfileApiError(response);
  return ((await response.json()) as ProfileResponse).data;
}
