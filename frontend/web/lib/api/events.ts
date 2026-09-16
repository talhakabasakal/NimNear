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
  image_url: string | null;
  calendar_id: string | null;
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

const LUNAS_PER_NIM = BigInt("100000");
const MAX_INT64 = BigInt("9223372036854775807");

/** Converts a decimal NIM string to exact Luna without using floating point. */
export function nimToLunas(value: string): bigint | null {
  const normalized = value.trim();
  if (!normalized || !/^[0-9]+(?:\.[0-9]+)?$/.test(normalized)) return null;

  const [integerPart, fractionPart = ""] = normalized.split(".");
  if (fractionPart.length > 5) return null;

  const fraction = BigInt((fractionPart + "00000").slice(0, 5));
  const lunas = BigInt(integerPart) * LUNAS_PER_NIM + fraction;
  return lunas <= MAX_INT64 ? lunas : null;
}

/** Returns the canonical exact decimal string accepted by POST /events. */
export function normalizeNimPrice(value: string): string | null {
  const lunas = nimToLunas(value);
  if (lunas === null) return null;
  if (lunas === BigInt("0")) return "0";

  const integer = lunas / LUNAS_PER_NIM;
  const fraction = (lunas % LUNAS_PER_NIM).toString().padStart(5, "0").replace(/0+$/, "");
  return fraction ? `${integer}.${fraction}` : integer.toString();
}

export function formatNimPrice(price: string) {
  return normalizeNimPrice(price) ?? price.trim();
}

export type EventQuery = {
  city?: string;
  place_id?: string;
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
  if (query.place_id) url.searchParams.set("place_id", query.place_id);
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
  calendar_id?: string;
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
