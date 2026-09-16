# NIMNear Data Provenance

This document records where every user-facing dynamic value comes from and defines the boundary between application records, static interface content, and test/development tooling. It reflects the repository after finalization task A6.

## Runtime policy

- User-facing event, profile, participation, purchase, and place records must come from NIMNear backend APIs and their authoritative database state.
- An API failure must render an error state. It must never substitute a static record, a previous demo collection, or a fabricated success.
- A successful response with an empty collection renders an empty state distinct from an error.
- Loading skeletons, navigation labels, form labels/examples, design tokens, icon choices, neutral avatar/image fallbacks, and explicitly labeled editorial taxonomy are interface content, not application records.
- Test fixtures are allowed only in test files and isolated test databases. They must not be imported by runtime packages or normal startup.
- No NIMNear product seed runs during server startup, migrations, `dev.sh`, or Docker Compose startup.

## Route provenance

### `/` — Keşfet

Dynamic data: `GET /api/v1/events?limit=4` through `fetchEvents`. Event count and event-card title, date, location, price, capacity-derived status, organizer ID, and media URL all originate in the API response.

Static interface content: hero copy; category names and colors; navigation; and neutral calendar empty/error copy. Categories are a local taxonomy, not event records. Nearby place cards are not rendered until the user explicitly chooses `Konumumu kullan`; successful cards come from `GET /api/v1/places/nearby` and link to stable `/places/{id}` routes. Istanbul, Kadıköy, Galata, and Karaköy are no longer runtime place records or location fallbacks.

Loading: route-level skeleton in `app/loading.tsx`.

Empty: `EventList` renders `Henüz etkinlik yok` only after a successful empty response.

Error: a failed events request passes an explicit error marker to `EventList`; no event records are rendered. The hero reports that the event service is unavailable rather than showing a fabricated count.

The homepage calendar preview and `/calendars` surface use `GET /api/v1/calendars`; calendar cards and detail data are backend-derived. The homepage section is `Yaklaşan etkinlikler`: it displays chronological upcoming API records, not a popularity ranking.

Location behavior: the homepage requests browser geolocation only after an explicit user action. Permission denied, unsupported browser/WebView, timeout, backend error, and no-nearby-place results are distinct states. No precise location is persisted, and unavailable location never claims Istanbul or another city.

### `/calendars` and `/calendars/[id]`

Dynamic data: `GET /api/v1/calendars` supplies the public calendar collection. `GET /api/v1/calendars/{id}` supplies the active public calendar and its published/public associated events. The authenticated workspace uses `GET /api/v1/me/calendars`; create and follow/unfollow actions use the existing JWT-protected calendar endpoints.

Static interface content: headings, onboarding copy, labels, neutral missing-media artwork, and empty/error descriptions. No calendar, owner, follower, or event counts are fabricated.

Loading: route-level calendar skeletons and a client workspace skeleton.

Empty/not found: public empty arrays render `Henüz herkese açık takvim yok`; owned/followed empty arrays render `Henüz takvim yok`; an unknown/private public calendar maps to not-found.

Error: public, detail, and authenticated workspace failures render explicit error states. No stale or static calendar list is used. Native Nimiq connection without a backend JWT is presented as blocked for ownership/follow mutations.

### `/events` and `/events?view=past`

Dynamic data: `GET /api/v1/events` through `fetchEvents`, with `from` for upcoming and `to` for past. Past results are additionally checked against the API `is_past` field. Timeline groups are derived only from returned `starts_at` values.

Static interface content: page title and time-filter labels. The page has no implicit city context; event records and their city values come from the backend.

Loading: route-level event-list skeleton.

Empty: `EventTimeline` renders `Bu dönemde etkinlik yok` only for a successful empty result.

Error: `EventTimeline` renders an explicit service error before evaluating collection emptiness. No static event list is substituted.

### `/places/[id]`

Dynamic data: `GET /api/v1/places/{id}` supplies the active place record. `GET /api/v1/events?place_id={id}&from={now}` supplies upcoming events filtered server-side by the stable place UUID.

Static interface content: labels, formatting, loading skeletons, neutral missing-media artwork, and empty/error copy. No city description, venue list, event count, or image is fabricated.

Loading: route-level place skeleton.

Empty/not found: an inactive or unknown place maps to the not-found screen; a successful place with no matching upcoming events renders an empty state.

Error: place and event API failures are rendered separately. No stale static place or event collection is substituted.

Dependency: city-level taxonomy and subscriptions remain unsupported and deferred to later product decisions; public calendar discovery and legacy-JWT-protected calendar workspace behavior are implemented.

### `/events/[id]`

Dynamic data: `GET /api/v1/events/{id}` supplies all event content. When `organizer_id` exists, `GET /api/v1/profiles/{organizer_id}` supplies organizer identity. Authenticated free-event participation and paid purchase status use their typed participation/purchase APIs.

Static interface content: labels, formatting, neutral missing-media art, and generic status explanations. Neutral media/avatar fallbacks contain no fabricated person, event, price, attendee, or venue data.

Loading: route-level detail skeleton plus component-level participation and purchase skeletons.

Empty/not found: an event API 404 maps to the route not-found screen.

Error: event API failures render `Etkinlik yüklenemedi`. Organizer API failure is shown independently as `Organizatör bilgisi yüklenemedi`; it is not silently replaced with a person. Participation, purchase restore, and event-creation auth checks distinguish a 401 from service failure. Non-auth failures render retryable errors and do not expose anonymous, ready-to-buy, or other fabricated states. Purchase polling retains the last backend purchase state and displays polling errors while retrying.

Dependency: protected participation and purchase actions still depend on a legacy backend JWT. Native Nimiq account permission does not create that JWT; the verified Nimiq authentication bridge remains externally blocked.

### `/events/create`

Dynamic data: `GET /api/v1/me` validates an existing backend session; `GET /api/v1/me/calendars` supplies only the authenticated organizer's eligible owned calendars; the explicit nearby-place action reads active records from `GET /api/v1/places/nearby`; and `POST /api/v1/events` creates the record and returns the authoritative event ID used for navigation.

Static interface content: form labels, validation copy, and neutral input examples. No default city, calendar, place, image, attendee, or event record is synthesized. Figma-only theme, landscape, and approval controls are omitted because the current backend does not persist them.

Loading: authentication-check skeleton, calendar dependency state, explicit nearby-place loading state, and submit pending state. Calendar and place failures remain visible; the organizer can continue without an optional association when appropriate.

Empty: no owned calendar and no nearby place are explicit optional states.

Error: expired/invalid JWT transitions to the blocked Nimiq connection state. Other auth-service failures render a retryable error. Validation, dependency, and create failures remain visible and no event is synthesized. A successful response navigates to the returned real event ID.

Contract: `price_nim` is normalized exactly to integer Luna semantics (1 NIM = 100,000 Luna; no rounding). An owned active `calendar_id` or active `place_id` is persisted; a selected place cannot be combined with custom address/coordinates. Empty capacity means unlimited. Creation remains coupled to legacy JWT authentication until the verified Nimiq identity bridge exists.

### `/profile`

Dynamic data: the internal user ID is read from the existing JWT session; `GET /api/v1/profiles/{id}` plus organized and attended profile-event APIs supply every profile field, count, and event card.

Static interface content: tab labels, field labels, neutral avatar fallback, and empty-state copy.

Loading: profile skeleton.

Empty: organized and attended tabs render explicit empty states only when their API collections are empty.

Error: any profile/history request failure renders a profile error; no static profile or event history is substituted.

Dependency: self-profile resolution remains coupled to the legacy JWT session. `listAccounts()` alone is not treated as authentication.

### `/profiles/[id]`

Dynamic data: the route ID selects `GET /api/v1/profiles/{id}` and the organized/attended event APIs. The server-fetched profile may initialize the client screen, but it is still API data.

Static interface content: labels, neutral avatar fallback, tabs, and empty-state copy.

Loading: public-profile route skeleton.

Empty: empty event histories are explicit successful states.

Error/not found: profile 404 maps to not-found; other profile or history failures render an error. No person or history is fabricated.

### `/profile/edit`

Dynamic data: `GET /api/v1/profiles/{current-user-id}` initializes editable values; `PATCH /api/v1/me/profile` persists display name, username, and bio.

Static interface content: field labels, validation guidance, neutral avatar fallback, and username example placeholder.

Loading: profile-form skeleton and submit pending state.

Empty: optional username and bio may legitimately be empty; they are not fallback records.

Error: profile load and mutation failures are explicit. The form never substitutes a static profile.

Dependency: current-user ID and mutation authorization remain coupled to the legacy JWT session until verified Nimiq authentication is available.

## Backend and tooling boundaries

### Runtime handlers

The removed `internal/gateway/handlers/example_handler.go` returned fabricated product records (`Product 1`, `Product 2`, and `new-id`). It was not registered by startup, but retaining it created an unsafe path for accidental reuse. It has been deleted, and a router regression test proves `/api/v1/products` is not reachable. No custom sample backend handler is registered by default.

A configured gateway endpoint without a usable backend now fails closed with HTTP 502 instead of returning endpoint metadata as a successful data-looking response.

### Migrations and normal startup

Repository migrations define schema and required authorization metadata; they contain no inserts for events, places, profiles, event participants, purchases, tickets, or calendars. Server startup, `dev.sh`, and Docker Compose do not invoke the manual seed command.

### Manual RBAC bootstrap

`make seed` invokes `backend/scripts/seed.go` only when a developer/operator explicitly requests it. The command inserts predefined RBAC roles and role-permission relationships. It does not create NIMNear product or user-facing records. A focused test records every statement and rejects writes outside the RBAC tables.

### Postman

Postman collections and local environments are manual API-development tooling. Their example credentials, tenant values, and request bodies do not execute during startup and are not frontend fallbacks. Sending those requests can intentionally mutate a selected development database, so they must never be run against production without review.

### Tests

Mocks, fakes, fixtures, example addresses, and deterministic records inside `_test.go` files are test-only. They validate repositories, Nimiq RPC verification, DTO mapping, and configuration without becoming runtime application data.

## Remaining static product surfaces

The following are intentionally retained and are not represented as live records:

- homepage category taxonomy;
- input examples and neutral media/avatar fallbacks;
- the chronological event feed under the truthful `Yaklaşan etkinlikler` label;
- explicit location instructions and neutral unavailable-location copy.

None of these surfaces supplies fake event, profile, attendee, payment, ticket, or location API records.

## A3 verification contract

A3 is complete when all of the following continue to hold:

1. `/api/v1/products` returns 404 and cannot expose the removed sample products.
2. Normal startup and migrations do not create product records.
3. The explicit RBAC bootstrap writes only `roles` and `role_permissions`.
4. Collection components evaluate error before empty and never receive static record fallbacks.
5. Authenticated component restore failures distinguish 401 from service failure.
6. Type checking, lint, production build, Go tests, Go vet, and `git diff --check` pass.
