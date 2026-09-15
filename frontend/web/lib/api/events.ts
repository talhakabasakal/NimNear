export type EventRecord = {
  id: string;
  title: string;
  description: string;
  starts_at: string;
  ends_at: string;
  status: string;
  price_nim: string;
  currency: string;
  capacity: number | null;
  attendee_count: number;
  image_url: string;
  place_id: string | null;
  latitude: number | null;
  longitude: number | null;
  address: string | null;
  city: string;
  organizer_id: string | null;
  is_free: boolean;
  is_sold_out: boolean;
  is_past: boolean;
  created_at: string;
  updated_at: string;
};

type EventsResponse = {
  data: EventRecord[];
};

type EventResponse = {
  data: EventRecord;
};

export class EventsApiError extends Error {
  constructor(public readonly status: number, message = "Events API returned " + status) {
    super(message);
    this.name = "EventsApiError";
  }
}

export function formatNimPrice(price: string) {
  const normalized = price.trim();
  if (!normalized) return price;

  const [integer, fraction] = normalized.split(".");
  if (!fraction) return normalized;

  const trimmedFraction = fraction.replace(/0+$/, "");
  return trimmedFraction ? `${integer}.${trimmedFraction}` : integer;
}

export type EventQuery = {
  city?: string;
  from?: string;
  to?: string;
  limit?: number;
};

export const apiBaseUrl = process.env.NEXT_PUBLIC_NIMNEAR_API_URL ?? process.env.NIMNEAR_API_URL ?? "http://localhost:8080";

async function throwEventsApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as { message?: string; error?: string } | null;
  throw new EventsApiError(response.status, payload?.message ?? payload?.error ?? "Events API returned " + response.status);
}

export async function fetchEvents(query: EventQuery = {}): Promise<EventRecord[]> {
  const url = new URL("/api/v1/events", apiBaseUrl);

  if (query.city) url.searchParams.set("city", query.city);
  if (query.from) url.searchParams.set("from", query.from);
  if (query.to) url.searchParams.set("to", query.to);
  if (query.limit) url.searchParams.set("limit", String(query.limit));

  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) await throwEventsApiError(response);

  const payload = (await response.json()) as EventsResponse;
  return payload.data;
}


export async function fetchEvent(id: string): Promise<EventRecord> {
  const url = new URL(`/api/v1/events/${encodeURIComponent(id)}`, apiBaseUrl);
  const response = await fetch(url, { cache: "no-store" });

  if (!response.ok) await throwEventsApiError(response);

  const payload = (await response.json()) as EventResponse;
  return payload.data;
}


export type CreateEventInput = {
  title: string;
  description: string;
  starts_at: string;
  ends_at: string;
  price_nim: string;
  currency: string;
  capacity?: number;
  image_url?: string;
  place_id?: string;
  latitude?: number;
  longitude?: number;
  address?: string;
  city: string;
};

export async function createEvent(input: CreateEventInput, token: string): Promise<EventRecord> {
  const response = await fetch(new URL("/api/v1/events", apiBaseUrl), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(input),
  });

  if (!response.ok) await throwEventsApiError(response);

  const payload = (await response.json()) as EventResponse;
  return payload.data;
}
