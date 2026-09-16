# NIMNear — Frontend Guidelines

## Location

Frontend source: `frontend/web`

## Stack

- Next.js
- App Router
- TypeScript
- Tailwind CSS
- shadcn/ui
- @nimiq/mini-app-sdk
- qrcode and html5-qrcode are installed dependencies for future/isolated ticket or check-in surfaces; no ticket or QR product flow is currently exposed.

## Primary Environment

The UI is mobile-first and should work well inside the Nimiq Mini App/WebView environment.

Desktop compatibility is useful but must not compromise the primary mobile experience.

## Recommended Structure

```text
frontend/web/
├── app/
├── components/
│   ├── ui/
│   └── ...
├── hooks/
├── lib/
├── public/
└── ...
```

Do not create folders only to match this document if the project already has a sensible structure.

## App Router

Use the Next.js App Router.

Prefer server components unless a component needs browser-only behavior or interactivity.

Use "use client" only where required.

## Current route tree

- /
- /events
- /events?view=past
- /events/[id]
- /events/create
- /places/[id]
- /calendars
- /calendars/[id]
- /profile
- /profiles/[id]
- /profile/edit


### Calendar discovery

`/calendars` is a server-rendered public calendar surface backed by `GET /api/v1/calendars`. `/calendars/[id]` uses `GET /api/v1/calendars/{id}` and renders only the associated backend events returned by that response. Public list/detail pages distinguish loading, empty, error, and not-found states and never substitute static calendar records.

The authenticated workspace reads `GET /api/v1/me/calendars` and renders real `Takvimlerim` and `Takip edilenler` collections. Calendar creation and follow/unfollow actions send the existing backend JWT. A connected Nimiq account without that JWT is shown a restrained blocked state; `listAccounts()` is never treated as backend authentication.

### Location-driven discovery

The homepage does not request location on load and does not assume Istanbul or another city. The user must explicitly choose `Konumumu kullan`. The browser geolocation result is sent only to `GET /api/v1/places/nearby` for the existing radius-bounded search and is not stored as profile data. Permission denied, unsupported WebView/browser, timeout, backend failure, and no-nearby-place results have distinct UI states; users can continue browsing `/events` without location. Successful nearby places link to `/places/{id}`, whose event list is filtered server-side by the stable place UUID.

Likely client-side areas include:
- Nimiq SDK integration;
- geolocation;
- QR scanning/camera APIs;
- interactive filters;
- modal/dialog state;
- client-side navigation state when necessary.

## Figma

Figma is the visual source of truth.

When implementing a Figma screen:
1. inspect the actual frame using Figma MCP;
2. identify reusable components;
3. inspect variables/tokens;
4. inspect typography;
5. inspect spacing;
6. inspect interaction states;
7. understand responsive behavior where provided;
8. then implement.

Do not blindly paste Figma-generated code into production. Translate the design into the existing Next.js architecture.

## shadcn/ui

shadcn/ui is the preferred UI primitive foundation.

Important rules:
- generated shadcn components are project-owned source code;
- customize them to match Figma;
- do not keep default shadcn styling when it conflicts with the design;
- do not rebuild an existing primitive unnecessarily;
- do not add many components from shadcn preemptively;
- add only what the current feature needs.

Typical primitives may include:
- Button
- Dialog / Drawer
- Sheet
- Input
- Tabs
- Badge
- Card
- Skeleton
- Separator
- Tooltip
- Dropdown Menu

These are examples, not mandatory dependencies.

## Component Strategy

Prefer reusable components.

Potential examples include:
- AppShell
- Header
- BottomNavigation
- PlaceCard
- EventCard
- SearchInput
- FilterChip
- EmptyState
- LoadingState
- ErrorState
- ActionButton

These names are illustrative. Do not create components that are not required by the actual design.

## TypeScript

Prefer strict types. Avoid `any` unless unavoidable.

Domain objects should have shared, explicit types rather than anonymous object shapes repeated across components.

## Styling

Use Tailwind CSS.

Prefer design tokens/variables for repeated values.

Use CSS variables where appropriate for shadcn/theme tokens and Figma-derived design tokens.

Avoid accumulating arbitrary one-off values when a shared design token exists.

Do not introduce another styling framework unless explicitly requested.

## Responsive Behavior

Start from the mobile layout.

Support small mobile screens, modern phone sizes, safe spacing, touch-friendly controls and WebView constraints.

Avoid relying exclusively on hover interactions.

## States

Interactive screens should consider loading, empty, error, permission denied, offline/network failure and success states.

Only implement states relevant to the feature being built.

## Location

Location access must be treated as permission-based.

Never assume permission will be granted.

The UI should support an appropriate fallback when location is unavailable or denied.

Exact location data should only be requested when necessary for the feature.

## API Layer

Do not scatter raw `fetch()` calls across presentation components.

Use a dedicated API/client layer once backend endpoints are introduced.

If server-side data fetching is used, keep backend base URLs and secrets server-safe.

Do not expose backend secrets through `NEXT_PUBLIC_*`.

Frontend components should consume typed application-level functions.

## Environment Variables

Only environment variables that are safe to expose to the browser may use `NEXT_PUBLIC_*`.

Never place secrets, private API keys or backend credentials in public frontend environment variables.

## Nimiq

Use `@nimiq/mini-app-sdk` for Mini App functionality.

Keep Nimiq integration isolated from generic presentation components where practical.

Nimiq browser/WebView integrations should be placed in client components or client-side hooks/modules.

Do not implement custom private-key storage.

The current legacy JWT compatibility session is stored in browser sessionStorage by the existing auth API module. This is technical debt retained for protected operations; it is not a native Nimiq identity and must not be treated as one. Native listAccounts() success does not populate this session.

## Quality

Before considering a frontend task complete:
- TypeScript should compile;
- existing lint checks should pass;
- production build should pass;
- the result should be compared against the referenced Figma design.

Run:

```bash
npm run lint
npm run build
```

Run a dedicated typecheck script too if the project defines one.


## Event creation

`/events/create` remains protected by the existing backend JWT. Native Nimiq
`listAccounts()` connection is not treated as authentication; users without a
valid JWT see the blocked native-connect state and cannot submit the form.

The form loads the authenticated user's owned calendars from
`GET /api/v1/me/calendars` and offers only those real active records. Nearby
place selection is explicit and uses `GET /api/v1/places/nearby` after browser
geolocation; no static place options are rendered. A selected place is sent as
`place_id` and cannot be combined with custom address/coordinates. Without a
place, custom address and a complete coordinate pair are optional.

Supported inputs are title, description, local date/time controls serialized
as RFC3339 UTC by the browser, city, optional HTTP(S) image URL, optional
capacity, optional owned calendar, optional active place or custom location,
and free/paid `price_nim`. NIM input is normalized without floating-point
rounding: one NIM is exactly 100,000 Luna and more than five fractional
places are rejected. Empty capacity means unlimited. Successful creation
navigates to the backend-returned event ID; validation, dependency, auth, and
service errors remain visible.

Figma-only landscape, theme, and approval controls are intentionally omitted
until the backend has a persisted contract for them.

## Free event participation

The event detail page uses the typed participation API layer for:

- GET /api/v1/events/{id}/rsvp
- POST /api/v1/events/{id}/rsvp
- DELETE /api/v1/events/{id}/rsvp

The page restores the existing legacy JWT session through /api/v1/me before loading user-specific RSVP state. Users without that JWT see native Nimiq connection plus an explicit blocked protected-action state; listAccounts() is not authentication. JWT-authenticated users can join or cancel a free upcoming event, with a pending state that prevents duplicate clicks. The participation response updates the attendee count, sold-out state, and CTA presentation.

Paid events do not show an RSVP action; they use the separate authenticated Nimiq Pay purchase component. Past events do not show an RSVP action, and sold-out events render a disabled state unless the current user is already attending and needs to cancel.

RSVP state is not requested through the public event fetch and is never presented as a global attendance claim. Payment state is user-specific and handled by the separate purchase component; ticket, QR, invitation, and notification UI is not implemented. Public calendar discovery and legacy-JWT-protected calendar workspace actions are implemented separately.



## Profile and organizer integration

The frontend now includes:

- /profile for the current signed-in user's public profile;
- /profiles/[id] for a public profile route;
- typed profile API functions in lib/api/profiles.ts;
- organizer identity on event detail when organizer_id resolves to the public profile API.

The Profile screen uses the confirmed Figma hierarchy: avatar fallback, display name, optional handle/bio, join date, organized/attended counts, tabs, real event cards, loading, error, and empty states. The authenticated self profile also links to the inferred `/profile/edit` route. That route uses the typed `PATCH /api/v1/me/profile` API for display name, username, and plain-text bio only; avatar and account identity fields remain read-only. Public profiles do not expose the edit control.

The profile API is public and privacy-safe. The frontend does not display email as profile identity, and it does not fabricate organizer names, avatars, venue metadata, payment state, tickets, QR codes, or invitation state.

## Optional fields and media rendering

Typed event and profile records mirror the backend's explicit-null response contract. Optional event fields (`capacity`, `image_url`, `place_id`, coordinates, `address`, and `organizer_id`) and optional profile fields (`username`, `bio`, and `avatar_url`) are `T | null`, never optional properties. Components compare or coalesce these values deliberately rather than depending on an omitted property being falsy.

Event and profile images share a client-side external-media renderer. It accepts only absolute HTTP(S) URLs within the backend's 2,048-byte limit, sends no referrer, and switches to the established neutral artwork/avatar state when media is absent, invalid, or fails to load. It never substitutes a stock event image or a profile record. The neutral `UserRound` avatar and event gradient are presentation states, not claims that uploaded media exists.

The project currently uses browser `<img>` loading for validated external URLs, so Next.js remote image patterns are not widened. No media host allowlist exists yet. A future first-party object-storage or approved-host decision should add matching backend host validation and browser CSP/image configuration.

## Paid-event payment integration

Paid future events use the existing event-detail sidebar and a restrained EventPurchase client component.

- Authenticated users create or reuse a purchase through the typed API layer.
- Payment instructions come from GET /api/v1/purchases/{id}/payment-instructions; the client does not choose the recipient, event price, or network.
- The component initializes @nimiq/mini-app-sdk with init() and calls sendBasicTransaction({ recipient, value }) with the backend's exact integer Luna amount.
- Before passing the amount to the SDK, the component validates its decimal-string format and checks Number.MAX_SAFE_INTEGER.
- The installed provider getNetwork() value is the Nimiq provider namespace, not a reliable TestAlbatross/MainAlbatross identifier, so it is not used for consensus-network matching. Testnet selection is enforced by the Nimiq Pay runtime and the backend network/RPC configuration.
- For phone-based local validation, the frontend API URL must be reachable from the device (for example, a LAN address rather than localhost), and the backend CORS configuration must allow the Mini App origin.
- A recognizable wallet rejection is rendered as a normal retryable state and never submitted as payment.
- The returned transaction hash is sent to POST /api/v1/purchases/{id}/transaction. The UI shows submitted/verifying until the backend returns confirmed.
- Refresh/reopen recovery uses the authenticated current-purchase endpoint and persisted backend state. Polling is bounded, and the backend reconciliation worker retries submitted/verifying purchases independently of the browser.
- Confirmed UI stops at the purchase confirmation message. Ticket, QR, checkout, refund, and booking actions are not implemented.

The paid component is client-side because the Mini App provider is browser/WebView-only. It does not handle keys, secrets, or production funds.
