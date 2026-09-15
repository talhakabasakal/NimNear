import { apiBaseUrl } from "./events";

export type ParticipationState = {
  event_id: string;
  attending: boolean;
  attendee_count: number;
  capacity: number | null;
  is_sold_out: boolean;
};

type ParticipationResponse = {
  data: ParticipationState;
};

export class ParticipationApiError extends Error {
  constructor(public readonly status: number, message = "Participation request failed") {
    super(message);
    this.name = "ParticipationApiError";
  }
}

async function requestParticipation(path: string, token: string, init?: RequestInit): Promise<ParticipationState> {
  const response = await fetch(new URL(path, apiBaseUrl), {
    ...init,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(init?.headers ?? {}),
    },
  });
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { message?: string; error?: string } | null;
    throw new ParticipationApiError(response.status, payload?.message ?? payload?.error ?? "Participation request failed");
  }
  const payload = (await response.json()) as ParticipationResponse;
  return payload.data;
}

export function fetchParticipation(eventID: string, token: string) {
  return requestParticipation(`/api/v1/events/${encodeURIComponent(eventID)}/rsvp`, token, { cache: "no-store" });
}

export function createParticipation(eventID: string, token: string) {
  return requestParticipation(`/api/v1/events/${encodeURIComponent(eventID)}/rsvp`, token, { method: "POST" });
}

export function cancelParticipation(eventID: string, token: string) {
  return requestParticipation(`/api/v1/events/${encodeURIComponent(eventID)}/rsvp`, token, { method: "DELETE" });
}
