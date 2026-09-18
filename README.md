# NIMNear

NIMNear is a Nimiq Mini App for discovering and creating local events. The
current application has a Next.js frontend and a Go API backed by PostgreSQL,
with Redis, Kafka, tenant/workspace infrastructure, and the legacy JWT
boundary retained from the original backend foundation.

## Repository entry points

- frontend/web — Next.js 16 App Router application.
- backend — Go HTTP API and domain services.
- docs — product, architecture, API, design, provenance, and finalization
  documentation.
- AGENTS.md — repository instructions for coding agents.

Figma is the visual source of truth for the NIMNear interface.

## Current frontend routes

- / — discovery and chronological upcoming events.
- /events — upcoming events.
- /events?view=past — past events.
- /events/[id] — event detail, participation, and paid-event payment state.
- /events/create — authenticated event creation.
- /places/[id] — active place detail and its upcoming events.
- /calendars — public calendars and the authenticated workspace.
- /calendars/[id] — public calendar detail and published events.
- /profile — self profile, inline edit, logout, and account deletion when a session exists.
- /profiles/[id] — public profile.

The frontend renders dynamic records from the API. Loading skeletons, empty and
error states, neutral missing-media/identity fallbacks, navigation labels, and
the local category taxonomy are interface behavior, not application records.
There are no runtime demo event, profile, place, payment, or ticket records.
Development place inventory is loaded only by the explicit operator command
`make seed-dev-places`. See docs/PLACES_OPERATIONS.md and
docs/PRODUCT_BOUNDARIES.md.

## Current API boundary

Public reads include health, places, events, calendars, and public profiles.
JWT-protected operations include /api/v1/me, profile editing, event creation,
calendar ownership/follows, RSVP, and purchase operations. The exact route list
and DTO rules are in docs/BACKEND.md.

Nimiq wallet authentication is implemented for Testnet. `NimiqConnect` requests
a backend AUTH_LOGIN challenge, signs it with Nimiq Pay or the Testnet Hub,
and the API sets an HttpOnly session cookie. Protected NIMNear routes accept
that cookie (`credentials: include`). Legacy email/password login still exists on the API for non-browser clients
and is rate-limited. Production disables it unless
`NIMNEAR_EMAIL_AUTH_ENABLED=true`. The Mini App does not store a JWT in
sessionStorage or localStorage. The leftover platform/gateway APIs are
disabled in production unless `NIMNEAR_PLATFORM_API_ENABLED=true`.

Paid events currently use the configured global merchant recipient. The backend
authoritatively derives integer Luna amounts, verifies submitted transaction
hashes against the configured Nimiq RPC, binds the RPC sender to the
authenticated user's verified Nimiq identity, and confirms only after the
configured macro-block finality rule. Entitlements, tickets, QR/check-in,
refunds, payouts, and organizer-recipient routing are not implemented.

## Local development

From the repository root:

    ./dev.sh

This is the monorepo runner based on `backend/dev.sh`. It starts Docker
infrastructure, applies migrations, then runs the Go API on
http://localhost:8080 and the Next.js app on http://localhost:3000.

    ./dev.sh          # Docker infrastructure, migrations, API, and frontend
    ./dev.sh server   # API + frontend only
    ./dev.sh infra    # infrastructure only
    ./dev.sh migrate  # migrations only
    ./dev.sh down     # stop local services
    ./dev.sh logs     # tail Docker logs
    ./dev.sh clean    # stop infra, remove volumes, clean artifacts

The frontend runs on http://localhost:3000. API base URL resolution is:

1. NEXT_PUBLIC_NIMNEAR_API_URL in browser-visible frontend configuration;
2. NIMNEAR_API_URL for server-side use;
3. http://localhost:8080 as the local fallback.

Nimiq wallet login uses NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK (`test-albatross` or
`main-albatross`). Development may omit it. Production must set it explicitly.

For a phone/WebView on a LAN, use a reachable LAN API URL and configure backend
CORS for the actual frontend origin. Do not put backend secrets in NEXT_PUBLIC_*
variables.

Validation:

    cd backend && go test ./... && go vet ./...
    cd frontend/web && npm run lint && npx tsc --noEmit && npm run build

See docs/FINALIZATION_PLAN.md for the remaining work. A1 through A9 are
complete; A10 is the remaining Lane A release validation, followed by the
blocked Nimiq identity and paid-ownership work.
