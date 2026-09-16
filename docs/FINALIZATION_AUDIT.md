# NIMNear Finalization Audit

> Historical snapshot: this audit records the repository state and findings at the time it was run. A1–A8 were subsequently completed. Use the current implementation and docs/FINALIZATION_PLAN.md for present status; do not read the pre-A5/A8 findings below as current runtime facts.

Date: 2026-09-16

Scope: repository-wide completion, data provenance, API contract, Nimiq Mini App, payment, privacy, and production-readiness audit. The audit read the repository guidance and product documents, inspected all current frontend routes and backend domain slices, searched for mock/demo/fallback patterns, and ran the existing static validation. No application code, migrations, backend infrastructure, or existing worktree changes were modified by this audit.

## Executive status

NIMNear has real backend-backed event, profile, RSVP, and payment-verification slices. It does not yet satisfy the final product requirement for production use. Native Nimiq Pay account connection is available through `init()` and `listAccounts()`, but it is not authentication and cannot produce the JWT required by protected APIs. Paid payments still go to the single `NIMNEAR_MERCHANT_ADDRESS`, not to a verified organizer account. Confirmed purchases do not yet create a ticket/entitlement or enter attended-event history.

Loading, empty, unavailable-image, and neutral identity fallbacks are UI states, not fake application records. No user-facing static event, profile, attendee, price, payment, or venue record was found. The homepage does contain static application catalog/location content that is not sourced from an API; this is a data-source gap and is tracked below.

## Findings

### P0 — Native Nimiq identity is not connected to backend authentication

- Affected files: `frontend/web/components/auth/nimiq-connect.tsx`, `frontend/web/lib/api/auth.ts`, `backend/internal/infrastructure/http/router/router.go`, `backend/internal/shared/middleware/auth.go`, `backend/internal/infrastructure/auth/jwt_service.go`.
- Current behavior: the frontend explicitly calls `init({ timeout: 10000 })` and `listAccounts()` and displays the first returned address. No `sign()` challenge flow, Nimiq identity binding, account-to-user lookup, or JWT issuance exists. Protected event creation, free RSVP, profile editing, and purchase endpoints still require the legacy email/password-issued JWT. A fresh Mini App user can connect natively but cannot perform those protected operations.
- Desired final behavior: a cryptographically verified Nimiq identity maps to one internal `users.id`, then receives the existing server-issued JWT/session. `listAccounts()` alone must never be treated as authentication.
- Backend/API dependency: challenge issuance and single-use storage, exact SDK signing contract, backend public-key/signature/address verification, account binding, and a JWT issuance endpoint.
- Recommended fix: implement the previously decided challenge/signature bridge as a separate authenticated identity domain while preserving existing JWT validation and existing user/event/purchase foreign keys during migration.
- Validation required: valid signature, wrong address/key, expired challenge, replay, cross-origin/domain mismatch, wallet/account switch, JWT claims, and protected endpoint authorization tests.

### P0 — Paid-event recipient is still a global merchant address

- Affected files: `backend/internal/shared/config/config.go`, `backend/cmd/server/main.go`, `backend/internal/application/eventpurchase/usecase/purchase.go`, `backend/internal/infrastructure/nimiq/rpc/client.go`, migrations `00014_create_events.sql` and `00019_create_event_purchases.sql`.
- Current behavior: payment instructions and RPC verification use `NIMNEAR_MERCHANT_ADDRESS`. The purchase verifier compares the transaction recipient to that configured address. The event's `organizer_id` is not resolved to a verified Nimiq account, and the purchase does not snapshot an organizer recipient.
- Desired final behavior: attendee account → event organizer's verified Nimiq account. A recipient must be bound to a verified identity, not an arbitrary editable profile field, and the recipient used for a purchase must remain stable after purchase creation.
- Backend/API dependency: verified Nimiq identity storage, organizer eligibility, event-recipient snapshot, payout/refund policy, and updated payment-instruction/verifier contracts.
- Recommended fix: add the identity/recipient domain only after the native authentication contract is implemented; make purchase creation snapshot the eligible organizer recipient and remove the global merchant address from the product payment path. Keep `NIMNEAR_MERCHANT_ADDRESS` only as an explicitly non-production/dev fallback if product approves it.
- Validation required: organizer verification lifecycle, recipient rotation after purchase creation, wrong-recipient rejection, event ownership, concurrency, and testnet end-to-end verification.

### P0 — Production can run with known default authentication/database credentials

- Affected files: `backend/internal/shared/config/config.go:175-178`, `backend/cmd/server/main.go:90-92`, `backend/deployments/docker-compose.yml:1-11`.
- Current behavior: the backend uses `change-me-in-production` as the JWT default and only logs a warning. Database defaults are also convenience credentials. The compose file is documented as local-only, but the server configuration does not fail closed if production operators omit the JWT secret.
- Desired final behavior: production startup rejects known/default secrets and requires explicit production database credentials; local defaults remain confined to the documented local workflow.
- Backend/API dependency: deployment environment and secret-management policy.
- Recommended fix: add environment validation at the deployment boundary and fail startup for production when defaults are present. Do not use local compose credentials for shared or public environments.
- Validation required: startup rejection tests, deployment configuration review, secret rotation, and confirmation that local development still works.

### P1 — Paid confirmation has no entitlement, ticket, or paid attendance projection

- Affected files: `backend/internal/application/eventpurchase`, `backend/internal/infrastructure/postgres/eventpurchase`, migrations `00019_create_event_purchases.sql` and `00020_payment_verification_states.sql`, `backend/internal/infrastructure/postgres/event/event_repository.go`, `frontend/web/components/events/event-purchase.tsx`, `frontend/web/components/profile/profile-screen.tsx`.
- Current behavior: a finalized transfer becomes a confirmed purchase and the UI stops at “Ödeme doğrulandı.” There is no entitlement/ticket record, ticket delivery, ticket ownership API, or QR payload. Attended profile events are sourced only from `event_participants`, so a confirmed paid purchase is not included in attended-event history.
- Desired final behavior: one confirmed purchase creates a durable, owner-scoped entitlement/ticket and the attended history reflects the product's chosen definition of attendance.
- Backend/API dependency: ticket/entitlement domain, idempotent confirmation hook or worker, ownership model, and product decision on one ticket versus multiple tickets.
- Recommended fix: define the entitlement contract before implementing ticket UI; make confirmation-to-entitlement creation idempotent and server-owned.
- Validation required: exactly-once entitlement creation under repeated polling/webhooks, owner isolation, capacity accounting, and confirmed/failed/reorg policy tests.

### P1 — Confirmed Figma calendar and city experiences are not implemented

- Affected files: `frontend/web/components/app/app-header.tsx:10-28`, `frontend/web/app`, `frontend/web/app/page.tsx`, `frontend/web/components/discover/city-card.tsx`, `docs/USER_FLOWS.md:59-67`.
- Current behavior: `Takvimler` is a disabled “Yakında” navigation label and there is no calendar route, calendar API, create/follow model, or calendar result flow. The Figma-confirmed Istanbul city detail has no dedicated city route. City cards are non-linking presentation blocks.
- Desired final behavior: either implement the confirmed calendar and city-detail contracts, or explicitly remove/de-scope their navigation and catalog presentation after a product decision.
- Backend/API dependency: calendar/follow domain and city/place discovery/detail API, or an approved scope reduction.
- Recommended fix: decide the scope first; do not add placeholder routes or fabricated calendar/city data.
- Validation required: route reachability, real empty/error states, public/private rules, and mobile navigation tests.

### P1 — Location-aware discovery is not connected to the places backend

- Affected files: `frontend/web/app/page.tsx:43-85`, `frontend/web/app/events/page.tsx:36-43`, `frontend/web/components/discover/discover-hero.tsx:23-28`, `frontend/web/components/discover/category-card.tsx:9-15`, `frontend/web/components/discover/city-card.tsx:9-20`, `frontend/web/lib/api`, `backend/internal/infrastructure/http/handler/place/handler.go`, `backend/internal/application/place/usecase/nearby_places.go`.
- Current behavior: the homepage and events page display “İstanbul” unconditionally. The homepage fetches events without a city or coordinate filter. The places nearby API exists, but no frontend route consumes it and no permission/denied/unavailable location flow is implemented. Static cards list İstanbul, Kadıköy, Galata, and Karaköy without backend records or navigation.
- Desired final behavior: discovery context must be derived from an approved location/city source, and unavailable or denied location must have an explicit empty/error/fallback UX. A city or place card must either navigate to a real backend-backed result or be clearly static UI configuration.
- Backend/API dependency: browser geolocation permission flow, nearby-place/event query contract, city-detail contract, and product decision on default city behavior.
- Recommended fix: implement location/context and API wiring before claiming location-based discovery complete; avoid silently presenting İstanbul as the user's location.
- Validation required: permission granted/denied/unavailable, LAN/WebView behavior, coordinate validation, city filtering, no horizontal scroll, and real empty/error responses.

### P1 — Event creation is narrower than the confirmed Figma creation flow

- Affected files: `frontend/web/components/events/create-event-form.tsx`, `backend/internal/application/event/dto/event_dto.go`, `backend/internal/application/event/usecase/events.go`, migration `00014_create_events.sql`.
- Current behavior: the real form/API supports title, description, start/end, city, address, price, capacity, and optional coordinates. It does not provide backend-backed image selection, theme/landscape choices, calendar selection, approval settings, or other creation controls shown in the inspected Figma flow. The form itself acknowledges this limitation.
- Desired final behavior: align the visible creation UX with the finalized backend contract, either by implementing each supported field or by removing unsupported controls from the committed product scope.
- Backend/API dependency: event media, calendar, moderation/approval, and presentation metadata domains, or an approved reduced Figma scope.
- Recommended fix: choose the supported MVP contract before adding UI fields; never collect values that are silently discarded.
- Validation required: request/response contract tests, authorization, field persistence, and create-to-detail rendering.

### P1 — Submitted/verifying purchases can remain unresolved indefinitely

- Affected files: `backend/internal/application/eventpurchase/usecase/purchase.go`, `backend/internal/infrastructure/postgres/eventpurchase/purchase_repository.go`, `frontend/web/components/events/event-purchase.tsx`, `docs/BACKEND.md:207-210`.
- Current behavior: once a transaction hash is accepted, the capacity hold expiry is cleared and submitted/verifying purchases remain active. Frontend polling is bounded at 40 attempts, but no backend worker, timeout, manual resolution, or reorg policy exists. A stuck transaction can reserve capacity indefinitely.
- Desired final behavior: define an operational terminal policy for permanently missing, invalidated, or unrecoverable transactions while preserving finalized-payment safety.
- Backend/API dependency: verifier retry policy, worker/scheduler or operator tooling, reorg/finality policy, and capacity-release rules.
- Recommended fix: decide and implement a server-owned reconciliation policy before production ticketing or high-demand events.
- Validation required: RPC outage, never-found hash, invalid transaction, delayed finality, duplicate submission, capacity release, and restart/refresh recovery.

### P1 — Production browser origin and Nimiq network configuration are deployment dependencies

- Affected files: `backend/internal/shared/config/config.go`, `backend/internal/shared/middleware/cors.go`, `frontend/web/next.config.ts`, `frontend/web/.env.local`, `docs/BACKEND.md:209`, `docs/FRONTEND.md:243-244`.
- Current behavior: local frontend configuration points to `http://192.168.1.11:8080` and Next dev allows `192.168.1.11`. Backend CORS is empty unless `CORS_ALLOWED_ORIGINS` is explicitly configured. Payment verification is only enabled when network, merchant, and RPC settings are all present, and the SDK does not provide a reliable consensus-network identifier for automatic matching.
- Desired final behavior: deployed frontend origin, backend CORS, Nimiq Pay runtime network, RPC endpoint, and payment recipient policy must be explicitly configured and operationally checked as one environment.
- Backend/API dependency: deployment environment, testnet/mainnet release policy, RPC provider, and final organizer-recipient design.
- Recommended fix: provide environment-specific configuration validation/runbooks; keep testnet and production settings impossible to mix by accident.
- Validation required: browser preflight, Mini App WebView request, testnet RPC responses, wrong-network transaction rejection, and production configuration review.

### P2 — Nullable JSON fields are represented as nullable TypeScript fields but are omitted by the backend

- Affected files: `backend/internal/application/event/dto/event_dto.go:46-54`, `frontend/web/lib/api/events.ts:1-24`.
- Current behavior: `address`, `place_id`, coordinates, and `organizer_id` use `omitempty` on pointer DTO fields, so absent values may be omitted rather than serialized as `null`. The frontend types declare them as `string | null`/`number | null`, and runtime rendering currently tolerates `undefined` through truthiness checks.
- Desired final behavior: make the JSON contract explicit and keep generated/manual types accurate.
- Backend/API dependency: choice of `null` versus omitted optional fields and contract documentation.
- Recommended fix: standardize the contract and add response-schema tests; do not rely on assertions that bypass runtime validation.
- Validation required: absent address/image/organizer/place fields on list and detail responses.

### P2 — Public profile image and event image URLs have weak URL policy

- Affected files: `backend/internal/application/event/usecase/events.go:156-158`, `backend/internal/infrastructure/postgres/migrations/00016_add_profile_fields.sql`, `frontend/web/components/events/event-card.tsx`, `frontend/web/components/events/event-detail-hero.tsx`, `frontend/web/components/profile/profile-screen.tsx`.
- Current behavior: image URLs are length-limited but not scheme/host-validated. The browser renders them through plain `<img>` and uses a visual fallback when missing or invalid.
- Desired final behavior: external media policy should be explicit, with safe URL validation/allowlisting or first-party storage, and browser policy/CSP appropriate to that choice.
- Backend/API dependency: avatar/event-media strategy and allowed origins.
- Recommended fix: decide the media policy before production; the current missing-image visual fallback may remain.
- Validation required: malformed URL, `javascript:`/non-HTTP input rejection, remote failure, CSP, and tracking/privacy review.

### P2 — “Popular events” is a temporary chronological slice, not popularity ranking

- Affected files: `frontend/web/app/page.tsx:16-18,43-46`, `frontend/web/lib/api/events.ts:66-79`, `docs/FRONTEND.md:237-248`.
- Current behavior: the homepage requests the first four upcoming events in backend chronological order while labeling the section “Popüler etkinlikler.” No popularity signal or ranking API exists.
- Desired final behavior: either label this as upcoming/nearby or add a product-approved popularity definition and API later.
- Backend/API dependency: popularity metric and ranking policy, if retained.
- Recommended fix: keep the current limitation documented and avoid implying a nonexistent ranking.
- Validation required: ordering/label acceptance test.

### P2 — Repository contains dead generic sample/mock handler code

- Affected files: `backend/internal/gateway/handlers/example_handler.go:57-61,83-88,121-126`.
- Current behavior: the generic example handler returns `Product 1`, `Product 2`, and `new-id` mock data. It is not registered by `backend/cmd/server/main.go` and no NIMNear frontend route calls it; it is therefore not confirmed user-facing application data.
- Desired final behavior: production repository should not retain reachable generic mock handlers, or they should be isolated as explicitly non-product examples.
- Backend/API dependency: gateway example/documentation ownership.
- Recommended fix: remove or quarantine during unrelated infrastructure cleanup only; do not mix this audit with product-domain changes.
- Validation required: confirm no endpoint registry or dynamic handler can expose it, then run gateway tests.

### P2 — Documentation has stale pre-native-auth statements

- Affected files: `docs/FRONTEND.md:214`, `docs/USER_FLOWS.md:132,159`, `frontend/web/README.md:19`.
- Current behavior: docs still refer to the old login/register panel and the frontend README calls the current page a temporary migration check. The current frontend uses NimiqConnect and intentionally blocks protected actions pending the signature-to-JWT bridge.
- Desired final behavior: documents accurately describe native account connection versus authentication and the current implementation status.
- Backend/API dependency: final auth bridge decision.
- Recommended fix: update documentation in the auth implementation pass, not by changing product behavior in this audit.
- Validation required: documentation review against routes and protected endpoint behavior.

## Route and data provenance audit

### `/`

- Backend data: `fetchEvents({ limit: 4 })` from `GET /api/v1/events`; event cards, price/status, count, date, location, and image come from the response.
- Local/static data: Figma-derived hero copy, category catalog/icons, İstanbul labels, four city-card labels/descriptions, and the calendars “Yakında” empty state.
- Classification: copy, icons, and design configuration are legitimate static UI. Categories and city names are static application catalog/location data, not confirmed fake records, and are not backend-driven.
- Loading/empty/error: root loading skeleton exists; event API failure shows an error state; an empty event response shows an empty state; there is no fake-event fallback.
- Silent fallback: none found. API failure returns `events: []` plus an error marker.

### `/events`

- Backend data: upcoming or past event list from `GET /api/v1/events` with RFC3339 `from`/`to`; chronological order is supplied by PostgreSQL.
- Local/static data: Istanbul header label, tab labels, timeline grouping/formatting, and empty/error copy.
- Classification: legitimate UI/configuration except the unverified Istanbul context.
- Loading/empty/error: route loading skeleton; API error state; period-specific empty state.
- Silent fallback: none found. `/events?view=past` additionally filters returned records by backend `is_past`.

### `/events/[id]`

- Backend data: event detail from `GET /api/v1/events/{id}` and organizer public profile from `GET /api/v1/profiles/{organizer_id}` when available. Event participation and payment status are fetched only for legacy JWT-authenticated users.
- Local/static data: formatting, status labels, neutral missing-image artwork, neutral missing-organizer omission, and action copy.
- Classification: legitimate presentation/fallback UI; no fabricated organizer identity, venue metadata, price, attendee count, or payment success state was found.
- Loading/empty/error: detail skeleton; 404 maps to `not-found`; non-404 event API failure shows error; organizer lookup failure omits the organizer block rather than inventing one.
- Silent fallback: none found for event data.

### `/events/create`

- Backend data: authenticated user refresh through `GET /api/v1/me`; successful submission through `POST /api/v1/events`; redirect uses the returned event ID.
- Local/static data: form labels, examples/placeholders, default free state, currency `NIM`, client validation, and unsupported-field notice.
- Classification: legitimate form configuration/examples, not demo records. The form is blocked for fresh native-only users because it still needs a JWT.
- Loading/empty/error: route and auth skeletons; validation errors; API errors; expired JWT returns to the native connection blocker. No fake created event is shown.
- Silent fallback: none found.

### `/profile`

- Backend data: self profile and organized/attended event lists through the public profile endpoints when an existing JWT session supplies the user ID.
- Local/static data: neutral avatar/initial fallback, join-date fallback text, tab labels, empty-state copy, and Nimiq connection presentation.
- Classification: legitimate presentation/unavailable states. The native account address is displayed only as a shortened provider result and is not treated as a profile identity.
- Loading/empty/error: profile skeleton; empty organized/attended states; service error; anonymous users see native Nimiq connection plus an explicit blocked message.
- Silent fallback: none found.

### `/profiles/[id]`

- Backend data: public profile and public organized/attended events.
- Local/static data: the same neutral avatar, date, empty, and error presentation.
- Classification: legitimate UI fallback; no email, password, JWT, tenant metadata, or fabricated profile identity is rendered.
- Loading/empty/error: route skeleton; 404 maps to `not-found` through the page's API error handling; other profile errors show an error message; event-list failure currently takes the whole profile screen to its error state.
- Silent fallback: none found.

### `/profile/edit`

- Backend data: profile read and `PATCH /api/v1/me/profile` when a legacy JWT exists.
- Local/static data: labels, validation copy, character counter, avatar/account read-only notice, and neutral fallback avatar.
- Classification: legitimate form/UI data. It does not fabricate a Nimiq-authenticated session.
- Loading/empty/error: client loading skeleton, native connection blocker for no legacy session, profile API error, field-specific validation messages, and generic API failure.
- Silent fallback: none found.

## Confirmed mock/fake/demo-data locations

The following are the only explicit mock/demo locations found:

- `backend/internal/gateway/handlers/example_handler.go` returns generic product mock records and success payloads. It is dead/unregistered sample infrastructure, not confirmed user-facing NIMNear data.
- `backend/postman/nimnear-api.postman_collection.json` and `backend/postman/nimnear-api-local.postman_environment.json` contain example tenant/user credentials and names. They are tooling examples, not runtime application data.
- `backend/scripts/seed.go` seeds roles and permissions only. It does not seed events, profiles, places, attendees, purchases, or payment records.
- `frontend/web/app/page.tsx` and discovery components contain hardcoded category/city presentation data. These are not proven fake/demo records, but they are static application data and are not backed by the places/events API.

The following are explicitly not mock application data: loading skeletons, empty/error cards, missing-image gradient artwork, neutral `NIMNear kullanıcısı`/initial fallbacks, form placeholders, navigation labels, design tokens, Lucide icons, and actual values rendered from event/profile/purchase API responses.

No static event array, static profile record, static attendee count, static price, static payment confirmation, fake session, or fake ticket state was found in the user-facing frontend.

## Backend domain audit

- Places: PostgreSQL schema, repository, use case, handler, and `GET /api/v1/places/nearby` exist. No frontend caller or place detail/list UI exists.
- Events: public list/detail and authenticated create exist; published/public filtering and chronological ordering are server-backed; edit, cancellation, moderation, media upload, calendar association, and richer Figma creation metadata do not exist.
- RSVP/event participants: free-event authenticated RSVP is transactional, idempotent, owner-scoped, and synchronizes `events.attendee_count`. It does not cover paid entitlements.
- Profiles: public profile DTOs intentionally omit email/password/JWT/tenant fields; organized/attended public event queries are real SQL aggregates/lists. Self edit exists only through the legacy JWT path. Native identity binding is absent.
- Profile editing: display name, optional username, and plain-text bio mutation are constrained and use a partial unique username index. Avatar mutation is not implemented. Username routing is not implemented.
- Purchases: pending/submitted/verifying/confirmed/failed/expired/cancelled persistence and owner checks exist. Amount is server-derived integer Luna and transaction hashes are normalized/unique. Confirmed purchase does not issue entitlement/ticket.
- Nimiq verification: RPC lookup, exact recipient/amount, basic-transfer fields, execution result, containing micro-block, configured network, and later-batch finality are checked. The recipient is still the global merchant address; no organizer verification exists. No background reconciler exists.
- Auth/session: email/password registration/login and HS256 JWT infrastructure remain. Native Nimiq listAccounts is presentation/account connection only. Default JWT secret handling is not production-safe.

## API contract and security audit

- Frontend event types match the main field names and Luna/presentation split, but nullable pointer fields may be omitted by JSON while TypeScript declares `null`.
- Event list/detail/create and profile endpoints use the typed frontend API layer; raw fetch calls are not scattered through event/profile presentation components. Payment, RSVP, and auth each have dedicated API modules.
- `NIMNEAR_API_URL` is local-only configuration in `frontend/web/.env.local`; no backend secret is exposed through `NEXT_PUBLIC_*`.
- Public profile reads do not expose email, password, JWT, auth claims, or tenant metadata. The protected IAM `/users` endpoints do return email and should remain strictly permission-scoped as internal data.
- Luna handling is integer/string-safe at the purchase boundary. Backend event creation converts decimal `price_nim` exactly using `big.Int` and rejects excess precision; frontend payment uses backend `amount_lunas` and checks safe numeric range before the SDK call.
- Purchase ownership is checked by `user_id`, hash uniqueness is database-enforced, and duplicate same-owner submissions are idempotent. Final organizer ownership and recipient binding are not enforced because that domain does not exist.
- Capacity protection includes free participants and pending/submitted/verifying/confirmed paid purchases, but unresolved submitted/verifying purchases have no release policy.
- No client-supplied amount, recipient, confirmation, or payment success is trusted by the backend.
- Nimiq Pay provider calls are explicit user actions and no automatic `listAccounts()` call is made on page load. No private-key or seed-phrase handling was found. Native provider availability/cancellation/error states are handled in the visible connection component.
- Network safety is operational/backend-enforced rather than provider-identified: the SDK does not expose a reliable consensus-network identifier in the documented implementation. The backend rejects transactions whose containing block does not match configured network, but deployment must still match Nimiq Pay runtime and RPC.

## Nimiq Mini App checklist

- PASS: `@nimiq/mini-app-sdk` is installed and `init({ timeout: 10000 })` is used.
- PASS: provider initialization and unavailable-provider handling are isolated in `NimiqConnect` and the payment component.
- PASS: account access and transaction approval are explicit user actions; no approval dialog is triggered on page load.
- PASS: no private keys, seed phrases, or wallet-internal state are accessed.
- PASS: user cancellation, ErrorResponse, empty accounts, and provider-unavailable states are handled for the visible account connection.
- PASS: local Mini App development uses a LAN-accessible frontend origin and Next `allowedDevOrigins` configuration.
- PARTIAL: native account connection works, but sign-in/authentication is absent.
- PARTIAL: payment code is wired to backend-authoritative Luna/recipient/network instructions, but final product recipient architecture is not implemented.
- PARTIAL: mobile-first layouts and touch-sized controls are present; a real-device final pass is still required for every final route.
- SKIP: Ethereum/token/chain-switch checklist items do not apply to the Nimiq-only flow.

## Frontend routes already backend-driven

Fully backend-driven dynamic content exists for `/events`, `/events?view=past`, and `/profiles/[id]` event/profile reads, subject to their documented static labels and UI formatting. `/events/[id]` is backend-driven for event/detail/organizer data, but user-specific RSVP/payment actions require the legacy JWT. `/profile`, `/profile/edit`, and `/events/create` use real backend data when an existing JWT session exists, but are not usable for a fresh native-only user until the Nimiq authentication bridge exists. `/` is only partially backend-driven because its event section is live but its discovery catalog/city content is static.

## Missing final-product capabilities

- Nimiq challenge/signature authentication and identity-to-`users.id` binding.
- Verified organizer Nimiq recipient identity and event-recipient snapshotting.
- Durable paid entitlement/ticket issuance and owner API.
- Paid attendance-history projection, if attended history includes paid entry.
- QR ticket display and organizer check-in, if required by final product scope.
- Calendar domain, calendar route, and follow/create behavior.
- City/place detail and location-aware discovery wiring.
- Final event creation contract for Figma-visible metadata, or explicit scope reduction.
- Long-running payment reconciliation/reorg/expiry policy.
- Production secret enforcement and environment separation.
- Final privacy/media policy for internal user endpoints and external image URLs.

## Finalization order

1. Freeze product decisions: native-auth-only normal-user model, public/private profile fields, calendars/cities, paid attendance semantics, one/multiple ticket policy, QR/check-in requirement, refunds, and unresolved-payment/reorg policy.
2. Implement the cryptographic Nimiq challenge flow and map verified identity to existing `users.id`; issue the existing JWT without fabricating sessions from `listAccounts()`.
3. Migrate/gate protected frontend operations to the verified Nimiq session while preserving existing user IDs, organizer IDs, profile records, RSVP rows, and purchase ownership. Keep legacy login only as an explicitly controlled migration/admin path until retirement is approved.
4. Add verified organizer account identity and make event/purchase recipient resolution server-owned and immutable per purchase. Remove the global merchant recipient from the final product flow.
5. Reconcile events, RSVP, capacity, and paid entitlements; implement idempotent confirmed-purchase entitlement creation and the chosen attended-history projection.
6. Define and implement ticket delivery, QR, and check-in only if the product decision confirms them; keep payment confirmation separate from ticket presentation.
7. Implement calendar and city/place/location domains or remove their unimplemented navigation/catalog surfaces. Wire the existing nearby-place API only after the permission/default-city contract is decided.
8. Align event creation UI and API to the final supported field set; never retain controls whose values are discarded.
9. Add long-running payment reconciliation and operational monitoring, then validate testnet RPC/network/finality behavior under delay, outage, replay, duplicate, and wrong-recipient cases.
10. Harden deployment configuration: reject known JWT/database defaults, set exact CORS origins, separate testnet/production RPC and recipients, define media/CSP policy, and verify no example handler is reachable.
11. Remove or quarantine dead sample infrastructure and update stale docs/README after the final architecture is locked.
12. Run full backend/frontend static validation plus real-device Mini App smoke tests for all implemented final routes and real backend-data provenance.

## Validation status

All commands below were run without seeding, migrations, server startup, transaction broadcast, or application-data mutation:

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `npm run lint`: PASS with existing non-fatal warnings in `app/layout.tsx`, `components/ui/button.tsx`, `components/events/event-purchase.tsx`, `components/events/create-event-form.tsx`, and `components/events/event-participation.tsx`.
- `npx tsc --noEmit`: PASS.
- `npm run build`: PASS; Next.js 16.3.5 compiled and generated the current routes successfully.

The build confirms these implemented routes: `/`, `/events`, `/events/[id]`, `/events/create`, `/profile`, `/profile/edit`, and `/profiles/[id]`. There is no `/calendars` or dedicated city-detail route in the current app tree.
