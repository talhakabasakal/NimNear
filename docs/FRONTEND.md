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
- qrcode
- html5-qrcode

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

Use `"use client"` only where required.

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


## Free event participation

The event detail page uses the typed participation API layer for:

- GET /api/v1/events/{id}/rsvp
- POST /api/v1/events/{id}/rsvp
- DELETE /api/v1/events/{id}/rsvp

The page restores the existing session through /api/v1/me before loading user-specific RSVP state. Anonymous users see the existing login/register panel. Authenticated users can join or cancel a free upcoming event, with a pending state that prevents duplicate clicks. The participation response updates the attendee count, sold-out state, and CTA presentation.

Paid events do not show an RSVP action; they use the separate authenticated Nimiq Pay purchase component. Past events do not show an RSVP action, and sold-out events render a disabled state unless the current user is already attending and needs to cancel.

RSVP state is not requested through the public event fetch and is never presented as a global attendance claim. Payment state is user-specific and handled by the separate purchase component; ticket, QR, invitation, calendar, and notification UI is not implemented.



## Profile and organizer integration

The frontend now includes:

- /profile for the current signed-in user's public profile;
- /profiles/[id] for a public profile route;
- typed profile API functions in lib/api/profiles.ts;
- organizer identity on event detail when organizer_id resolves to the public profile API.

The Profile screen uses the confirmed Figma hierarchy: avatar fallback, display name, optional handle/bio, join date, organized/attended counts, tabs, real event cards, loading, error, and empty states. The authenticated self profile also links to the inferred `/profile/edit` route. That route uses the typed `PATCH /api/v1/me/profile` API for display name, username, and plain-text bio only; avatar and account identity fields remain read-only. Public profiles do not expose the edit control.

The profile API is public and privacy-safe. The frontend does not display email as profile identity, and it does not fabricate organizer names, avatars, venue metadata, payment state, tickets, QR codes, or invitation state.

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
- Refresh/reopen recovery uses the authenticated current-purchase endpoint and persisted backend state. Polling is bounded and has no background worker.
- Confirmed UI stops at the purchase confirmation message. Ticket, QR, checkout, refund, and booking actions are not implemented.

The paid component is client-side because the Mini App provider is browser/WebView-only. It does not handle keys, secrets, or production funds.
