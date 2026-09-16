import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

import { resolveCollectionState } from "../lib/collection-state.ts";
import { locationStateForError, nearbyResultState } from "../lib/location-state.ts";

test("an API error never exposes supplied stale or fabricated records", () => {
  const state = resolveCollectionState([{ id: "stale-record" }], "api");

  assert.equal(state.kind, "error");
  assert.deepEqual(state.records, []);
});

test("a successful empty API response remains an empty state", () => {
  const state = resolveCollectionState([]);

  assert.equal(state.kind, "empty");
  assert.deepEqual(state.records, []);
});

test("a successful API response preserves only its returned records", () => {
  const records = [{ id: "backend-record" }];
  const state = resolveCollectionState(records);

  assert.equal(state.kind, "content");
  assert.equal(state.records, records);
});

test("the homepage labels the backend feed as upcoming, not popular", () => {
  const page = fs.readFileSync(new URL("../app/page.tsx", import.meta.url), "utf8");

  assert.match(page, /Yaklaşan etkinlikler/);
  assert.doesNotMatch(page, /Popüler etkinlikler|popular|popularity|trending/i);
});

test("geolocation errors stay distinct", () => {
  assert.equal(locationStateForError(1), "denied");
  assert.equal(locationStateForError(2), "unavailable");
  assert.equal(locationStateForError(3), "timeout");
});

test("a granted location with real nearby records becomes success", () => {
  assert.equal(nearbyResultState([{ id: "backend-place" }]), "success");
});

test("nearby API errors and empty results remain distinct", () => {
  assert.equal(nearbyResultState([], true), "error");
  assert.equal(nearbyResultState([]), "empty");
});

test("calendar navigation and surface are backend-backed, not a placeholder", () => {
  const header = fs.readFileSync(new URL("../components/app/app-header.tsx", import.meta.url), "utf8");
  const page = fs.readFileSync(new URL("../app/calendars/page.tsx", import.meta.url), "utf8");

  assert.match(header, /href: "\/calendars", label: "Takvimler"/);
  assert.match(page, /fetchCalendars/);
  assert.match(page, /Henüz herkese açık takvim yok/);
  assert.doesNotMatch(page, /Takvimler yakında burada|calendar mock|coming soon/i);
});

test("calendar mutation UI keeps native connection separate from backend authentication", () => {
  const workspace = fs.readFileSync(new URL("../components/calendars/calendar-workspace.tsx", import.meta.url), "utf8");

  assert.match(workspace, /NimiqConnect/);
  assert.match(workspace, /backend oturumu oluşturmaz/);
  assert.match(workspace, /createCalendar/);
  assert.doesNotMatch(workspace, /demo|mock|fake calendar/i);
});


test("event creation exposes only persisted backend fields", () => {
  const form = fs.readFileSync(new URL("../components/events/create-event-form.tsx", import.meta.url), "utf8");

  assert.match(form, /fetchMyCalendars/);
  assert.match(form, /fetchNearbyPlaces/);
  assert.match(form, /calendar_id/);
  assert.match(form, /place_id/);
  assert.match(form, /image_url/);
  assert.doesNotMatch(form, /organizer_id/);
  assert.doesNotMatch(form, /Tema|Manzara|Onay/);
});

test("NIM price handling is exact and rejects unsupported precision", () => {
  const eventsApi = fs.readFileSync(new URL("../lib/api/events.ts", import.meta.url), "utf8");

  assert.match(eventsApi, /function nimToLunas/);
  assert.match(eventsApi, /BigInt\("100000"\)/);
  assert.match(eventsApi, /fractionPart.length > 5/);
  assert.match(eventsApi, /function normalizeNimPrice/);
});
