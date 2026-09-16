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
- /profile — self profile and organized/attended history when a legacy JWT
  session is available.
- /profiles/[id] — public profile.
- /profile/edit — authenticated profile editing.

The frontend renders dynamic records from the API. Loading skeletons, empty and
error states, neutral missing-media/identity fallbacks, navigation labels, and
the local category taxonomy are interface behavior, not application records.
There are no runtime demo event, profile, place, payment, or ticket records.

## Current API boundary

Public reads include health, places, events, calendars, and public profiles.
JWT-protected operations include /api/v1/me, profile editing, event creation,
calendar ownership/follows, RSVP, and purchase operations. The exact route list
and DTO rules are in docs/BACKEND.md.

Native Nimiq Pay account connection is a frontend permission flow using
init({ timeout: 10000 }) and listAccounts() after explicit user action. It is
not backend authentication and does not issue a JWT. The cryptographic
Nimiq signature-to-JWT contract remains NO-GO; protected operations retain the
legacy JWT requirement until that contract is established and implemented.

Paid events currently use the configured global merchant recipient. The backend
authoritatively derives integer Luna amounts, verifies submitted transaction
hashes against the configured Nimiq RPC, and confirms only after the configured
macro-block finality rule. Entitlements, tickets, QR/check-in, refunds,
payouts, and organizer-recipient routing are not implemented.

## Local development

Backend:

    cd backend
    ./dev.sh

The existing workflow starts local Docker infrastructure, applies migrations,
and runs the server on http://localhost:8080. The current migration ceiling is
00023_payment_reconciliation.sql.

Frontend:

    cd frontend/web
    npm install
    npm run dev

The frontend runs on http://localhost:3000. API base URL resolution is:

1. NEXT_PUBLIC_NIMNEAR_API_URL in browser-visible frontend configuration;
2. NIMNEAR_API_URL for server-side use;
3. http://localhost:8080 as the local fallback.

For a phone/WebView on a LAN, use a reachable LAN API URL and configure backend
CORS for the actual frontend origin. Do not put backend secrets in NEXT_PUBLIC_*
variables.

Validation:

    cd backend && go test ./... && go vet ./...
    cd frontend/web && npm run lint && npx tsc --noEmit && npm run build

See docs/FINALIZATION_PLAN.md for the remaining work. A1 through A9 are
complete; A10 is the remaining Lane A release validation, followed by the
blocked Nimiq identity and paid-ownership work.
