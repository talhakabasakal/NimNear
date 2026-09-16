import { apiBaseUrl } from "./events";
import type { EventRecord } from "./events";

export type CalendarRecord = {
  id: string;
  name: string;
  description: string | null;
  image_url: string | null;
  visibility: "public" | "private";
  created_at: string;
  updated_at: string;
};

export type CalendarDetailRecord = {
  data: CalendarRecord;
  events: EventRecord[];
};

export type MyCalendarsRecord = {
  owned: CalendarRecord[];
  followed: CalendarRecord[];
};

type CalendarsResponse = { data: CalendarRecord[] };

type CalendarErrorPayload = { message?: string; error?: string };

export class CalendarsApiError extends Error {
  constructor(public readonly status: number, message = "Calendars API returned " + status) {
    super(message);
    this.name = "CalendarsApiError";
  }
}

async function throwCalendarsApiError(response: Response): Promise<never> {
  const payload = (await response.json().catch(() => null)) as CalendarErrorPayload | null;
  throw new CalendarsApiError(response.status, payload?.message ?? payload?.error ?? "Calendars API returned " + response.status);
}

export async function fetchCalendars(limit = 20): Promise<CalendarRecord[]> {
  const url = new URL("/api/v1/calendars", apiBaseUrl);
  url.searchParams.set("limit", String(limit));
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) await throwCalendarsApiError(response);
  return ((await response.json()) as CalendarsResponse).data;
}

export async function fetchCalendar(id: string): Promise<CalendarDetailRecord> {
  const response = await fetch(new URL("/api/v1/calendars/" + encodeURIComponent(id), apiBaseUrl), { cache: "no-store" });
  if (!response.ok) await throwCalendarsApiError(response);
  return (await response.json()) as CalendarDetailRecord;
}

export async function fetchMyCalendars(token: string): Promise<MyCalendarsRecord> {
  const response = await fetch(new URL("/api/v1/me/calendars", apiBaseUrl), {
    cache: "no-store",
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) await throwCalendarsApiError(response);
  return (await response.json()) as MyCalendarsRecord;
}

export type CreateCalendarInput = {
  name: string;
  description?: string;
  image_url?: string;
  visibility: "public" | "private";
};

export async function createCalendar(input: CreateCalendarInput, token: string): Promise<CalendarDetailRecord> {
  const response = await fetch(new URL("/api/v1/calendars", apiBaseUrl), {
    method: "POST",
    cache: "no-store",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify(input),
  });
  if (!response.ok) await throwCalendarsApiError(response);
  return (await response.json()) as CalendarDetailRecord;
}

async function mutateFollow(path: string, method: "POST" | "DELETE", token: string) {
  const response = await fetch(new URL(path, apiBaseUrl), {
    method,
    cache: "no-store",
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) await throwCalendarsApiError(response);
}

export function followCalendar(id: string, token: string) {
  return mutateFollow(`/api/v1/calendars/${encodeURIComponent(id)}/follow`, "POST", token);
}

export function unfollowCalendar(id: string, token: string) {
  return mutateFollow(`/api/v1/calendars/${encodeURIComponent(id)}/follow`, "DELETE", token);
}
