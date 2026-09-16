# NIMNear Finalization Plan

Date: 2026-09-16

This plan converts docs/FINALIZATION_AUDIT.md into executable work lanes.
No code, migration, payment, or frontend behavior is changed by this document. A1–A9 are complete; this document records their current status and the remaining A10/B-lane work.

## Completion gate

NIMNear is final only when:

- no user-facing route renders mock, demo, fake, or silently substituted
  application records;
- dynamic product data comes from real backend state or an explicitly approved
  static UI configuration;
- every API-backed surface has loading, empty, and error behavior;
- frontend DTOs match backend JSON behavior;
- public/private data boundaries are enforced;
- production configuration fails closed for unsafe defaults;
- stale and dead product/sample code is removed or quarantined;
- Figma-confirmed surfaces have real functionality or an explicit product
  scope decision;
- Nimiq identity, organizer recipient ownership, and paid ownership remain
  blocked until the external Mini App cryptographic contract is confirmed.

## External blocker

`docs/NIMIQ_AUTH_CONTRACT.md` is an external **NO-GO** blocker. The official
Mini App material still does not establish:

1. the exact bytes or digest signed by `sign(message)`;
2. the semantics of `{ message, isHex }`;
3. native signer selection when more than one account is approved; and
4. sufficient Mini App-specific vectors proving the returned public key is the
   address-verifiable ordinary Nimiq key.

Do not infer the Mini App contract from Hub/Keyguard signing, fabricate a JWT
from `listAccounts()`, trust a frontend address claim, weaken protected
endpoints, or route production payments through an unverified profile field.

# LANE A — CAN COMPLETE NOW

Lane A must remain valid with the current legacy email/password → JWT path. It
may improve backend APIs, public discovery, data provenance, production
configuration, and UI states without implementing Nimiq cryptographic auth.

## A0. Freeze the product decisions that do not require Nimiq identity

Dependency: current audit, product docs, Figma confirmation, and explicit
product-owner decisions.

Affected domains/files: `docs/PRODUCT.md`, `docs/USER_FLOWS.md`,
`docs/ARCHITECTURE.md`, final event/calendar/city/payment operations scope.

Exact implementation objective: record decisions for:

- whether calendars and city/location discovery are implemented in this
  release or explicitly de-scoped;
- default-city behavior when location is denied or unavailable;
- whether the homepage section is called “Yaklaşan etkinlikler” or receives a
  real popularity definition;
- the supported event-creation field set;
- external image/avatar policy;
- unresolved submitted/verifying purchase expiry and reconciliation;
- whether paid attendance means purchase ownership, check-in, or both;
- one-ticket versus multiple-ticket policy;
- whether QR/check-in is part of the final release.

Backend/API work: none; this is the decision gate for later tasks.

Frontend work: none; do not add placeholder screens while decisions are open.

Tests required: documentation review against current routes and Figma-confirmed
screens.

Definition of done: each decision has one owner-approved outcome, and the
documents identify what will be implemented, what will be removed, and what
remains unconfirmed.

## A1. Harden production configuration and environment separation — COMPLETE

Dependency: A0 only. This does not depend on Nimiq identity.

Affected files/domains: `backend/internal/shared/config/config.go`,
`backend/cmd/server/main.go`, `backend/deployments/docker-compose.yml`, CORS,
JWT configuration, frontend deployment configuration, Nimiq RPC/network
runbooks.

Exact implementation objective: make production startup fail closed when the
known JWT secret or convenience database credentials are used. Keep local
Docker defaults confined to the documented local workflow.

Backend/API work:

- distinguish local development from production configuration explicitly;
- reject `change-me-in-production` and known convenience credentials in
  production;
- require explicit JWT issuer, secret, database, Redis, CORS, and payment/RPC
  settings for the intended environment;
- validate exact allowed browser origins rather than accepting an empty CORS
  configuration for a deployed frontend;
- separate testnet RPC/network/recipient settings from production settings;
- document secret rotation and deployment checks.

Frontend work:

- remove deployment dependence on `.env.local` values;
- document the production API base URL and Mini App origin without exposing
  secrets;
- keep `allowedDevOrigins` limited to local development.

Tests required:

- production startup rejects missing/default JWT and database secrets;
- local development still starts with the documented workflow;
- browser preflight succeeds only for approved origins;
- frontend build has no secret exposed through `NEXT_PUBLIC_*`;
- testnet and production configuration cannot be accidentally mixed.

Definition of done: a production-like startup cannot run with known defaults,
the deployed origin/CORS/API/RPC settings are documented, and local development
still works.

## A2. Make API optional-field and media contracts explicit — COMPLETE

Dependency: A0 media decision. No Nimiq identity dependency.

Affected files/domains: `backend/internal/application/event/dto`, profile and
event media validation, `frontend/web/lib/api/events.ts`, profile/event image
components, response-schema tests.

Exact implementation objective: eliminate ambiguity between omitted optional
JSON fields and frontend `null` types, and enforce a safe external-media policy.

Backend/API work:

- choose one contract for optional fields: explicit `null` or omission;
- apply that choice consistently to `address`, `place_id`, coordinates,
  `organizer_id`, image URLs, and profile fields;
- add response contract tests for absent values;
- validate image/avatar URLs by approved scheme and, if applicable, host
  allowlist;
- reject `javascript:`, data URLs, unsupported schemes, malformed URLs, and
  values exceeding policy limits;
- document whether first-party object storage is required later.

Frontend work:

- type optional fields to match the actual contract rather than relying on
  truthiness to tolerate `undefined`;
- retain only neutral missing-image presentation, never a stock/application
  record;
- handle remote-image failure and missing image consistently;
- configure image/CSP policy only for approved origins.

Tests required: response-schema tests, malformed/non-HTTP URL tests, remote
failure tests, missing-field rendering tests, and CSP/image-origin review.

Definition of done: API responses and TypeScript types agree, invalid media is
rejected, and missing media renders only a clearly neutral UI state.

## A3. Remove dead sample infrastructure and audit data provenance — COMPLETE

Dependency: A0. No Nimiq identity dependency.

Affected files/domains: `backend/internal/gateway/handlers/example_handler.go`,
gateway registration/registry, Postman examples, `backend/scripts/seed.go`,
frontend discovery data arrays/components.

Exact implementation objective: guarantee that no user-facing route can reach
generic sample records or silently fall back to them.

Backend/API work:

- prove the generic example handler is not registered or dynamically exposed;
- remove it from production builds or quarantine it as explicitly non-product
  example infrastructure;
- keep Postman credentials and role/permission seeds clearly tooling-only;
- confirm seed scripts do not create events, profiles, places, attendees,
  purchases, tickets, or payment success records.

Frontend work:

- trace every route's dynamic data source;
- retain only legitimate static labels, icons, design tokens, and empty/error
  copy;
- replace static category/city application records with backend-backed data in
  A5, or remove those cards from the final product scope;
- ensure failed API requests never populate demo records.

Tests required: route/API reachability audit, source search for mock/demo/fake
records, gateway tests, and failure-path tests asserting no static product data
appears after API failure.

Definition of done: no reachable sample handler returns application-looking
records, no user-facing API failure displays fake records, and each remaining
static value is classified as UI configuration or product data.

## A4. Correct the homepage label and discovery semantics — COMPLETE

Dependency: A0 popularity decision. No Nimiq identity dependency.

Affected files/domains: `frontend/web/app/page.tsx`,
`frontend/web/lib/api/events.ts`, homepage discovery components,
`docs/FRONTEND.md`.

Exact implementation objective: stop presenting chronological upcoming events
as a popularity ranking.

Backend/API work:

- if the decision is chronological discovery, document the existing `limit: 4`
  upcoming query as the canonical behavior;
- if “popular” is retained, define a server-owned metric, API contract, and
  privacy/abuse policy in a later product-approved task.

Frontend work:

- rename the section to a truthful label such as “Yaklaşan etkinlikler” or
  “Yakındaki etkinlikler” when no ranking exists;
- keep event records, count, price, status, date, image, and location API-backed;
- preserve loading, empty, and error states.

Tests required: ordering and label acceptance test; API failure must not render
the previous event list as a fake fallback.

Definition of done: visible copy describes the actual query, with no implied
popularity ranking that the backend does not calculate.

## A5. Implement real city/place/location discovery or remove static catalog — COMPLETE

Status: COMPLETE. Runtime place discovery now uses active backend records after explicit geolocation; the static city catalog and implicit Istanbul claim were removed.

Dependency: A0 default-city/location decision, A2 field/media contract.
No Nimiq identity dependency for public read paths.

Affected files/domains: `backend/internal/application/place`, place schema and
repository, event query filters, `GET /api/v1/places/nearby`, frontend home and
events discovery components, city route, geolocation client component.

Exact implementation objective: eliminate the unconditional “İstanbul” claim
and replace static İstanbul/Kadıköy/Galata/Karaköy records with real backend
place/city data, or remove those surfaces if the product decision de-scopes
them.

Backend/API work:

- define the minimal public city/place read model and stable IDs;
- expose nearby place results using validated latitude, longitude, radius, and
  active-place filtering;
- define city detail and city event query contracts if the Istanbul Figma page
  remains in scope;
- make event discovery filterable by the approved location context;
- provide explicit empty/error responses for no nearby places or no events;
- do not insert demo places/events into production migrations.

Frontend work:

- request geolocation only through an explicit, understandable UX;
- handle granted, denied, unavailable, timeout, and WebView/LAN cases;
- show the backend-derived city/place context, not a hardcoded user location;
- make city cards link to a real city/detail/result route;
- render a real Istanbul/city detail page only if its API contract exists;
- keep no silent fallback to İstanbul unless A0 explicitly defines it as static
  product configuration rather than the user's location.

Tests required: permission states, invalid coordinates, radius bounds, empty
nearby results, API errors, city filtering, route reachability, and real-device
Mini App behavior.

Definition of done: every visible city/place/event discovery record is backend
state, or the corresponding static card is removed; location failure has an
explicit state; no route silently claims an unverified location.

## A6. Implement the public calendar surface without pretending native auth — COMPLETE

Status: COMPLETE. Public calendar discovery/detail and legacy-JWT-protected ownership/follow actions are implemented; native Nimiq connection remains non-authoritative for mutations.

Dependency: A0 calendar decision; public discovery contracts from A5 if
calendars are location-scoped. Public calendar reads do not require Nimiq auth.

Affected files/domains: calendar schema/domain/API, `frontend/web/app` calendar
route/components, shared navigation, `docs/USER_FLOWS.md`.

Exact implementation objective: replace the disabled “Yakında” calendar navigation with real public list/detail behavior and real authenticated workspace states.

Backend/API work:

- define public calendar list/detail DTOs and ownership/privacy rules;
- implement real `Takvimlerim` and followed-calendar reads;
- implement create/follow/unfollow with the existing legacy JWT because the committed Figma surface includes these actions;
- keep writes protected by the current legacy JWT until the Nimiq JWT bridge
  exists; do not treat `listAccounts()` as authorization;
- return real empty states when no calendars exist.

Frontend work:

- add the real `/calendars` and `/calendars/[id]` routes and navigation;
- render backend-backed lists and empty/error states;
- clearly disable or explain create/follow actions for a native-connected user
  who has no legacy JWT;
- do not create a custom auth workaround or fake calendar records.

Tests required: public/private read authorization, empty/error states, owner
isolation, create/follow idempotency, and native-connected-without-JWT behavior.

Definition of done: calendar navigation, public reads, empty/error states, and legacy-JWT-protected workspace actions are backend-backed; no “Yakında” placeholder represents calendars.

## A7. Align event creation with the final supported contract — COMPLETE

Status: COMPLETE. The create form now submits only persisted backend fields, and association authorization is enforced server-side.

Dependency: A0 supported-field decision, A2 optional/media contract, A5/A6 if
location or calendar fields remain in the form.

Affected files/domains: `frontend/web/components/events/create-event-form.tsx`,
event DTO/use case/repository, event migration/domain, image/media and
calendar associations.

Exact implementation objective: ensure every visible Figma-supported creation
control is persisted and returned, or remove unsupported controls from the
committed scope.

Backend/API work:

- implement only approved fields such as image/media, landscape/theme,
  calendar association, approval, and capacity behavior;
- persist fields with validation and authorization;
- return the same values in event list/detail responses;
- reject or remove controls whose values would be discarded;
- retain server-owned organizer ownership and price/Luna conversion.

Frontend work:

- align the form to the supported API, including any approved Figma controls;
- show field-level validation and backend errors;
- render the created event from the returned API record;
- keep the page blocked when no legacy JWT exists; do not claim native account
  connection is sufficient for creation.

Tests required: DTO/request contract tests, persistence round trip, field
validation, authorization, create-to-detail rendering, and unsupported-field
absence tests.

Definition of done: no visible creation input is silently ignored, and a
successful create is a real backend event reachable through its returned ID.

Implemented A7 contract: title, description, RFC3339 start/end timestamps,
city, optional external image URL, optional positive capacity, exact string
`price_nim` conversion to integer Luna, optional active place association or
custom address/coordinate pair, and optional active calendar owned by the
JWT organizer. Figma-only landscape, theme, and approval controls remain
omitted because the current backend does not persist them.

## A8. Define and implement unresolved-payment reconciliation — COMPLETE

Dependency: A0 payment operations decision. This implementation improves the
current merchant-address flow but does not claim the final organizer-recipient
design.

Affected files/domains: event purchase use case/repository, payment state
migration `00023_payment_reconciliation.sql`, reconciliation worker, config,
and `docs/BACKEND.md`.

Implemented policy: submitted/verifying purchases are retried only after a
bounded interval and are selected in deterministic bounded batches. Database
claim timestamps prevent concurrent frontend reads and workers from verifying
the same purchase at once. Temporary RPC errors, propagation/not-found, and
non-final results remain unresolved before the deadline. At the deadline the
backend makes one authoritative verifier attempt; confirmed is reserved for a
valid finalized transfer, invalid becomes failed, and any still-uncertain
result becomes expired. Expired purchases release capacity and cannot be
resurrected by later reads. Existing rows without a deadline use updated_at
plus the configured deadline as a compatibility fallback.

Frontend work was intentionally unchanged: existing owner-scoped polling and
refresh recovery consume the backend state, including expired, without
suggesting ticket or entitlement ownership.

Configuration: `NIMNEAR_PURCHASE_RECONCILIATION_INTERVAL_SECONDS` (default
30), `NIMNEAR_PURCHASE_RECONCILIATION_DEADLINE_MINUTES` (default 60), and
`NIMNEAR_PURCHASE_RECONCILIATION_BATCH_SIZE` (default 50).

Definition of done: every submitted/verifying purchase has a bounded
server-owned reconciliation policy, capacity behavior is documented, duplicate
hashes remain database-protected, and no UI implies final ownership before
confirmation.

## A9. Update stale documentation and close the unblocked validation loop — COMPLETE

Dependency: A1–A8 decisions and completed behavior. No Nimiq cryptographic
dependency except documenting the external blocker accurately.

Status: COMPLETE. Current root, backend, frontend, architecture, API, flow,
provenance, gateway, product, and finalization documents now match the
implemented routes and explicitly preserve the native-auth NO-GO boundary.

Affected files/domains: root README, backend README/security/test/Postman
documentation, gateway handler guide, frontend README, docs/ARCHITECTURE.md,
docs/BACKEND.md, docs/FRONTEND.md, docs/PRODUCT.md, docs/USER_FLOWS.md,
docs/DATA_PROVENANCE.md, and finalization status references.

Exact implementation objective: make documentation describe native account
connection as distinct from authentication, record the current blocked
protected actions, and remove references to deleted login/register panels or
temporary migration checks.

Backend/API work: document final public API contracts, production settings,
unresolved payment policy, calendar/city scope, and legacy JWT boundaries.

Frontend work: document routes, data provenance, loading/empty/error states,
and any intentionally unavailable native-connected actions.

Tests required: documentation review against route tree, API handlers, and
protected middleware; repository search for stale claims.

Definition of done: no documentation tells users or implementers that
`listAccounts()` authenticates them, and all unblocked behavior matches the
actual repository.

## A10. Lane A validation and release checklist — NOT STARTED

Dependency: all selected A0–A9 tasks.

Affected domains: complete repository, local infrastructure, deployed-like
configuration, real backend data.

Exact implementation objective: prove that unblocked product surfaces contain
no mock/fake data and work against real API state.

Backend/API work: run tests, vet, API contract tests, health checks, CORS and
configuration checks without seeding application demo records.

Frontend work: run lint, typecheck, build, and real-device Mini App smoke tests
for every implemented route.

Tests required:

```text
go test ./...
go vet ./...
npm run lint
npx tsc --noEmit
npm run build
GET /health/live
GET /health/ready
```

Also verify `/`, `/events`, `/events?view=past`, `/events/[id]`, selected
calendar/city routes, `/events/create`, `/profile`, `/profiles/[id]`, and
`/profile/edit` against live backend data and failure responses.

Definition of done: the final route/data-provenance audit finds no user-facing
mock records, no silent fake fallback, no unsafe production defaults, and no
unverified auth claim.

# LANE A — LEGACY AUTH COUPLING

Lane A is executable without the Nimiq contract, but these areas still depend
on the existing email/password-issued JWT until Lane B is complete:

- `POST /api/v1/events` and `/events/create`;
- free RSVP and cancellation;
- profile self-read/edit and `/profile/edit`;
- purchase creation, payment instructions, transaction submission, and current
  purchase reads;
- calendar create/follow mutations, if implemented in A6;
- any internal `/me` or owner-scoped API;
- user-specific organized/attended views when the public endpoint needs a
  current user identity.

The native Nimiq connection UI may remain visible, but it must show a clear
blocked or legacy-session state for these operations. It must not fabricate a
JWT, silently call legacy login, or remove backend authorization.

# LANE B — BLOCKED BY NIMIQ AUTH CONTRACT

Lane B cannot begin implementation until the external Mini App `sign()` and
multiple-account contract is confirmed with authoritative documentation/source
or official test vectors. The following work depends on a verified identity,
not merely a connected account list.

## B0. Close the external Nimiq cryptographic contract

Dependency: Nimiq documentation/source or official maintainer-confirmed
vectors; specifically the blockers in `docs/NIMIQ_AUTH_CONTRACT.md`.

Affected files/domains: external provider contract, SDK version policy,
backend verification design, authentication test vectors.

Exact implementation objective: establish signed bytes/digest, `isHex`
semantics, result/error behavior, ordinary public-key/address equivalence, and
native signer selection for multiple approved accounts.

Backend/API work: none until the contract is authoritative; prepare no
cryptographic workaround.

Frontend work: none; do not add signing or authentication behavior.

Tests required: official positive/negative vectors, two-account signer tests,
message mutation tests, and network behavior confirmation.

Definition of done: `docs/NIMIQ_AUTH_CONTRACT.md` can be changed from NO-GO to
GO without assumptions.

## B1. Implement Nimiq challenge authentication and identity mapping

Dependency: B0 GO verdict.

Affected files/domains: new auth challenge/identity domain, router,
repositories/migrations, `users`, JWT service/use case, frontend API/auth
session layer.

Exact implementation objective: backend-generated one-time challenge → native
sign → cryptographic verification → verified Nimiq identity → internal
`users.id` → existing JWT.

Backend/API work:

- add challenge creation and verification endpoints;
- store nonce, exact message, domain, audience, network, claimed address,
  expiry, and consumed state;
- verify strict public-key/signature encodings, signed bytes, Ed25519
  signature, and public-key-derived address;
- consume challenges atomically and prevent replay;
- add unique network/address/public-key identity constraints;
- preserve existing JWT subject and protected middleware;
- resolve/create users without fabricated email, username, or device identity.

Frontend work:

- request the backend challenge after explicit native account interaction;
- pass the exact backend message to `sign()`;
- submit only the returned proof to the backend;
- persist/use the server-issued JWT/session;
- handle cancellation, provider errors, address mismatch, expiry, and retry.

Tests required: cryptographic vectors, wrong key/address/signature, expiry,
replay, race, domain/network mismatch, account switch, JWT claims, user
creation/linking, and protected-route authorization.

Definition of done: a native proof alone cannot impersonate another address,
one challenge yields at most one session, and protected APIs authorize the
internal user rather than a frontend address.

## B2. Migrate legacy users and remove normal-user legacy login dependency

Dependency: B1; explicit migration policy for current users.

Affected files/domains: users schema/credentials, profile identity mapping,
JWT DTOs, frontend auth panels and protected-route guards.

Exact implementation objective: preserve existing `users.id`, profiles,
organizer IDs, RSVP records, purchases, and roles while allowing a verified
Nimiq identity to link to or create an internal user.

Backend/API work:

- make legacy email/password fields compatible with native-only users or move
  credentials to a separate table;
- keep legacy login for controlled migration/admin use until retirement is
  approved;
- never match accounts by display name, guessed email, or device ID;
- define account-linking, unlinking, and recovery policy.

Frontend work:

- make native Nimiq connection the normal user entry point;
- replace blocked legacy-only prompts with the verified native session flow;
- retain a clearly scoped migration path where required;
- never display a connected address as authenticated before B1 succeeds.

Tests required: legacy login compatibility, linking, duplicate identity,
unlink/recovery, role preservation, session expiry, and anonymous access.

Definition of done: normal users can use the native identity flow without a
separate email/password product experience, while existing internal user and
ownership relationships remain stable.

## B3. Add verified organizer identity and recipient snapshotting

Dependency: B1/B2; organizer product policy and payment network configuration.

Affected files/domains: organizer profile/event ownership, verified identity
repository, event creation, purchase creation, payment instructions and RPC
verifier, `NIMNEAR_MERCHANT_ADDRESS` configuration.

Exact implementation objective: make every paid event resolve an eligible,
verified organizer Nimiq recipient and snapshot that recipient into the
purchase before wallet payment.

Backend/API work:

- define organizer verification/activation/revocation states;
- require an eligible verified identity for paid-event creation or publication;
- snapshot the recipient and network at purchase creation;
- make payment instructions and verification use the snapshot;
- reject recipient rotation from changing existing purchases;
- remove the global merchant address from the final product path, retaining it
  only as an explicitly non-production fallback if approved.

Frontend work:

- show organizers whether their payment identity is verified and eligible;
- prevent paid-event creation/publication when no recipient exists;
- render backend-authoritative recipient/payment instructions only;
- never expose an arbitrary profile address as a verified recipient.

Tests required: verification lifecycle, wrong recipient, revoked identity,
recipient rotation, event ownership, concurrency, network mismatch, and
testnet end-to-end payment verification.

Definition of done: a paid purchase can only target the event organizer's
verified identity, and the recipient cannot change underneath an existing
purchase.

## B4. Complete attendee-to-organizer paid ownership and entitlement

Dependency: B3 and the A8 payment reconciliation policy.

Affected files/domains: purchases, capacity, entitlement/ticket tables and
repositories, confirmation worker/use case, profile attended-event queries,
event detail and profile frontend components.

Exact implementation objective: finalized payment creates one server-owned,
owner-scoped paid entitlement and projects it into the chosen attendance
history without trusting frontend payment state.

Backend/API work:

- create an idempotent entitlement on confirmed purchase;
- define one-ticket/multiple-ticket, transfer, cancellation, refund, and reorg
  policy;
- enforce purchase/entitlement ownership and capacity consistency;
- include confirmed paid attendance in the approved profile history model;
- expose only safe owner-scoped entitlement state.

Frontend work:

- show entitlement only after server confirmation;
- display paid attendance from backend state;
- keep pending/verifying/failed/expired states distinct from ownership;
- do not add ticket or QR UI before the backend contract exists.

Tests required: exactly-once confirmation, duplicate polling, owner isolation,
capacity, payment failure/reorg, refresh recovery, and attended-history
projection.

Definition of done: a confirmed purchase has a durable, server-owned paid
entitlement and the UI cannot manufacture ownership.

## B5. Implement ticket issuance and QR/check-in only if product scope confirms

Dependency: B4 plus A0 ticket/QR decision; Figma states must be inspected or
the UI must be explicitly marked inferred.

Affected files/domains: ticket/entitlement API, ticket presentation, QR
payload/signing, organizer check-in, event attendance records.

Exact implementation objective: provide a server-owned ticket and a secure
check-in flow only after paid ownership and scope are complete.

Backend/API work:

- issue a non-forgeable, owner-scoped ticket after confirmed entitlement;
- define QR payload, expiry/replay protection, and check-in authorization;
- persist check-in state and audit events;
- prevent QR data from granting access without backend validation;
- define cancellation/refund/reissue behavior.

Frontend work:

- display tickets only for the current owner;
- render QR only from backend-issued ticket data;
- provide organizer check-in only to authorized organizers;
- show expired/used/invalid/error states without fake success.

Tests required: QR replay, wrong event, wrong owner, expired/used ticket,
offline/online behavior, organizer authorization, and exactly-once check-in.

Definition of done: ticket and QR/check-in behavior is server-authoritative,
owner-scoped, replay-safe, and explicitly supported by the product scope.

## B6. Retire blocked legacy assumptions after migration proves safe

Dependency: B1–B5, successful migration, and explicit product approval.

Affected files/domains: legacy auth UI/API, protected feature guards, payment
configuration, stale documentation, operational runbooks.

Exact implementation objective: remove or restrict legacy login and global
merchant assumptions only after native identity and organizer ownership are
operationally proven.

Backend/API work: deprecate legacy endpoints only with migration telemetry,
account recovery, and rollback policy; remove global recipient fallback from
production configuration.

Frontend work: remove normal-user email/password entry, update blocked-state
copy, and ensure every protected action uses the server-issued native JWT.

Tests required: migration cohort, rollback, session expiry, protected routes,
payment recipient, and no-regression public discovery tests.

Definition of done: no normal-user path relies on unverified listAccounts data,
no production purchase uses a global merchant fallback, and legacy behavior is
retired only after approved migration evidence.

# Recommended execution order

1. A0 — freeze the unblocked product and scope decisions.
2. A1 — fail closed on production secrets, database credentials, CORS, and
   environment mixing.
3. A2 — align nullable DTOs and external media policy.
4. A3 — remove/quarantine dead sample infrastructure and complete provenance
   checks.
5. A4 — correct the misleading popular-events label.
6. A5 — implement real city/place/location discovery, or remove static catalog
   surfaces according to A0.
7. A6 — COMPLETE: implement public calendar reads and legacy-JWT-protected mutations.
8. A7 — align event creation with only fields the backend stores.
9. A8 — implement unresolved-payment reconciliation while keeping the current
   recipient explicitly temporary.
10. A9 — COMPLETE: align current documentation with implemented behavior and
    preserve historical audit findings as historical.
11. A10 — run the complete no-mock/data-provenance and static/runtime release
    checklist.
12. B0 — obtain the external Nimiq contract and change the auth decision to GO.
13. B1 — implement verified Nimiq identity → internal user → JWT.
14. B2 — migrate legacy users and make native identity the normal entry point.
15. B3 — bind paid-event recipients to verified organizers.
16. B4 — issue paid entitlements and project paid attendance.
17. B5 — implement ticket/QR/check-in only if confirmed in A0.
18. B6 — retire legacy authentication and merchant fallbacks after migration.

This order keeps public discovery, truthful data, contract hygiene, and
production safety moving now while preventing payment ownership or protected
authorization from being built on an unverified Nimiq assumption.

NEXT_TASK=A10 — Run the complete no-mock/data-provenance and static/runtime release checklist.
