# NIMNear — Backend Guidelines

## Location

Backend: `backend`

## Stack

- Go
- NIMNear Go
- PostgreSQL
- Redis
- Kafka

## Existing Foundation

The NIMNear backend retains its existing clean/hexagonal foundation and should be extended rather than replaced.

Existing functionality includes infrastructure for:
- HTTP routing;
- authentication;
- organizations/tenants;
- workspaces;
- roles/permissions;
- API management;
- auditing;
- realtime;
- PostgreSQL;
- Redis;
- Kafka.

## Engineering Principle

NIMNear domain functionality should be added without unnecessary modification to the existing core infrastructure.

Prefer domain-specific additions over broad framework rewrites.

## Router Safety

The router uses Chi.

Chi requires middleware to be registered before routes on the same mux.

A previous router bug was fixed using a nested group to preserve middleware scope.

Current intended protected behavior must be preserved.

In particular, do not casually move middleware between parent and child groups.

## Database

PostgreSQL is the source of truth for persistent application data.

Schema changes should use the repository's existing migration mechanism.

Do not manually rely on untracked database changes.

Every persistent domain change should have a reproducible migration.

## Redis

Use Redis only where its characteristics are appropriate, such as ephemeral/cache/infrastructure workloads.

Do not duplicate PostgreSQL source-of-truth data into Redis without a clear reason.

## Kafka

Kafka is available but optional for NIMNear domain functionality.

Do not use Kafka simply because it exists.

Use asynchronous events when they provide a concrete architectural benefit.

## Configuration

Configuration must use the existing environment/config system.

Never hardcode passwords, JWT secrets, API keys, production URLs or credentials.

Local secrets belong in ignored environment files.

### Environment modes and production validation

`APP_ENV` explicitly selects `development`, `test`, or `production`; an unknown value fails configuration validation. It defaults to `development` so the existing `backend/dev.sh` and local Docker workflow continue to work.

- `development`: documented local defaults remain available, including the local PostgreSQL credentials and development JWT defaults.
- `test`: tests may use isolated explicit fixtures and do not depend on production secrets.
- `production`: startup fails before infrastructure initialization if JWT secret/issuer, database host/user/password/name, TLS mode, or CORS origins are missing or still using known development defaults. Empty or wildcard CORS origins are rejected. Database connection failure also stops production startup; it does not downgrade to an in-memory/no-database mode.

Payment configuration remains optional as before. When enabled in production, the existing `NIMNEAR_NIMIQ_NETWORK`, `NIMNEAR_MERCHANT_ADDRESS`, and `NIMNEAR_NIMIQ_RPC_URL` settings must be complete, must identify the production main network, and must not use local or obvious test-network RPC URLs. This is an environment-safety check only; payment routing is unchanged.

Validation errors contain configuration variable names and remediation guidance, never secret values or full connection strings.

## API Design

NIMNear endpoints should follow the existing backend routing conventions.

Before creating endpoints:
1. define the domain behavior;
2. define request/response contracts;
3. determine authentication requirements;
4. determine persistence requirements;
5. then implement handlers/services/repositories.

Do not make the frontend infer undocumented API shapes.

## Testing

Backend changes should be validated with:

```bash
go test ./...
go vet ./...
```

When relevant:
- `GET /health/live`
- `GET /health/ready`

Add regression tests for infrastructure bugs when practical.

## Scope

Avoid modifying IAM, tenant, API management or gateway behavior for a NIMNear feature unless the feature genuinely requires it.

Prefer the smallest correct change.
## Events Domain

The first NIMNear Events slice is backed by migration 00014_create_events.sql.
It adds a PostgreSQL events table with:

- event title, description, start/end timestamps, and lifecycle status (draft, published, or cancelled);
- canonical integer `price_lunas` storage; one NIM is exactly 100,000 Luna;
- optional capacity, attendee count, image, calendar/place relations, coordinates, address, and organizer relation;
- city, public visibility, and created/updated timestamps.

upcoming, past, sold out, and free are response-level derived values. invited is intentionally not an event-wide status; it requires a future user-specific invitation domain.

### Public event discovery

GET /api/v1/events returns a { "data": [...] } envelope containing only public, published events. Results are sorted by starts_at ASC in PostgreSQL.

Supported query parameters:

- city: case-insensitive city match;
- place_id: optional stable public place UUID;
- from: optional RFC3339 lower bound for starts_at;
- to: optional RFC3339 upper bound for starts_at;
- limit: optional integer from 1 to 100, default 20.

When neither from nor to is supplied, the lower bound defaults to the current UTC time, producing the default upcoming-event view.

GET /api/v1/events/{id} returns one public, published event or 404.


### Public calendar discovery

The calendar domain is backed by migrations `00021_create_calendars.sql` and `00022_add_event_calendar.sql`. `GET /api/v1/calendars` returns only active, public calendars in deterministic `created_at ASC, id ASC` order. `GET /api/v1/calendars/{id}` returns one active public calendar and its published, public events in chronological `starts_at ASC, id ASC` order. Empty collections are returned as empty arrays; no calendar seed data is created.

Calendar ownership and follows are protected by the existing JWT middleware. `GET /api/v1/me/calendars` returns the authenticated user’s owned and followed calendars. `POST /api/v1/calendars`, `POST /api/v1/calendars/{id}/follow`, and `DELETE /api/v1/calendars/{id}/follow` require the legacy backend JWT. Follow insertion is idempotent and unfollow is safely repeatable. Private and archived calendars are excluded from public reads.

Calendar DTOs expose stable IDs, name, visibility, timestamps, and optional description/image fields as explicit JSON `null` when absent. Owner IDs and follower data are not exposed through public calendar reads. Nimiq `listAccounts()` does not authorize calendar mutations; native Nimiq authentication remains blocked pending the verified signature-to-JWT contract.

### Public place discovery

`GET /api/v1/places/nearby?lat={latitude}&lng={longitude}&radius={meters}` returns a `{ "data": [...] }` envelope containing only active places. Latitude and longitude are validated, radius defaults to 5,000 meters and is capped at 50,000 meters, and results use Haversine distance with nearest-first ordering. An empty result is a successful empty `data` array.

`GET /api/v1/places/{id}` returns one active place by its stable UUID or 404. The public DTO contains only place identity, description, coordinates, address, category, and image URL; inactive places are not exposed.

Public event discovery accepts `place_id` and applies that filter in PostgreSQL before chronological ordering. The frontend does not fetch all events and filter them locally. No city taxonomy or place seed data is created by migrations or startup; city-level aggregation remains unsupported until a product decision defines it.

### Event creation

POST /api/v1/events is inside the existing JWT-protected route group. The authenticated JWT user becomes organizer_id; the endpoint creates a published public event. It validates title, timestamps, time ordering, decimal price, capacity, coordinate pairing/ranges, city, address, and image URL length.

The request uses the existing string `price_nim` edge field, for example `"12.5"`, so clients do not pass money through a floating-point type. The use case converts it exactly to integer Luna; more than five fractional decimal places are rejected rather than rounded. Empty or `"0"` means a free event. `calendar_id` may reference only an active calendar owned by the authenticated organizer. `place_id` must reference an active place; it cannot be combined with custom address or coordinates. With no `place_id`, custom address and a complete coordinate pair are optional.

### Optional response fields and external media

Public event and profile responses use explicit JSON `null` for absent optional fields. Fields are not omitted. For events this applies to `capacity`, `image_url`, `place_id`, `latitude`, `longitude`, `address`, and `organizer_id`. For profiles this applies to `username`, `bio`, and `avatar_url`. Required fields such as event `city`, profile `display_name`, timestamps, status, and counts remain present with their documented non-null types. The database keeps its existing nullable and empty-string storage semantics; response mapping provides the stable API contract. RSVP responses also return `capacity: null` when capacity is unlimited.

External event media accepts either an empty value or an absolute `http://` or `https://` URL no longer than 2,048 bytes. Malformed URLs, URL credentials, whitespace/control characters, and `javascript:`, `data:`, `file:`, and other schemes are rejected at event creation. Public response mapping suppresses invalid legacy event/profile media values as `null`. Profile avatar mutation is not currently exposed, so there is no avatar write boundary in this release.

There is no approved media host allowlist or first-party object storage provider. Host allowlisting, upload validation, object lifecycle management, and a matching restrictive browser `img-src` policy remain future hardening work once that infrastructure is selected.

Payments, tickets, QR codes, notifications, subscriptions, and user-specific invitations are not implemented by this domain slice. Public calendars and their optional event association are implemented by the calendar domain.


## Event participation / RSVP

Free event participation is implemented as an authenticated user-to-event relationship.

Protected routes:

- GET /api/v1/events/{id}/rsvp
- POST /api/v1/events/{id}/rsvp
- DELETE /api/v1/events/{id}/rsvp

The response is user-specific and contains event_id, attending, attendee_count, capacity, and is_sold_out. It is intentionally not included in the public event response.

RSVP is allowed only for authenticated users and public, published, upcoming, free events. Paid events return an explicit unsupported-operation error. Past and sold-out events are rejected. A NULL capacity preserves unlimited-capacity semantics.

POST is idempotent for an existing participation. DELETE cancels only the authenticated user's own participation and is safe to repeat.

The event_participants join table has a unique (event_id, user_id) constraint and indexes for event and user lookup. RSVP operations lock the event row inside a PostgreSQL transaction, check the actual join-row count, insert or delete the participation, and synchronize events.attendee_count in the same transaction. This prevents concurrent RSVPs from exceeding capacity while retaining the existing denormalized count for public discovery.

Invitations, payments, tickets, QR codes, notifications, and waitlists remain unsupported. Calendar creation/follow mutations use the existing legacy JWT until native Nimiq authentication is available.



## Public Profile Read Model

The first profile slice reuses the existing authenticated user record and exposes a privacy-safe public read model:

- GET /api/v1/profiles/{id}
- GET /api/v1/profiles/{id}/events?type=organized
- GET /api/v1/profiles/{id}/events?type=attended

The response contains the user's effective public display name, optional username, bio, avatar_url, joined_at, and public/published organized and attended event counts. The effective display name uses the nullable `display_name` field when present, then the existing first/last name fields. Email, password/authentication fields, JWT data, and tenant metadata are not exposed.

Profile event lists reuse the public EventInfo mapping and include only public, published events. Organized and attended counts are SQL aggregates. Migration `00016_add_profile_fields.sql` provides username, bio, and avatar read fields; migration `00017_add_profile_display_name.sql` adds the dedicated nullable display name field.

Authenticated self-profile editing is available at `PATCH /api/v1/me/profile`. The JWT subject identifies the target user; clients cannot update another user or arbitrary user-table fields. The request accepts only `display_name`, `username`, and `bio`. Omitted fields are unchanged, `null` clears a field, and an empty object is rejected. The response is the same privacy-safe `PublicProfileResponse` envelope.

Display names are trimmed and limited to 100 Unicode characters without line breaks or control characters. Usernames are optional, normalized to lowercase, restricted to the documented ASCII format, checked against reserved route names, and protected by the database partial unique index for concurrency-safe uniqueness. Bios are optional plain text, normalized to LF line endings, and limited to 280 Unicode characters. Avatar mutation, email/password changes, username routing, payments, and social graph operations are not part of this endpoint.

Validation errors use stable `error_code` values: `invalid_display_name`, `invalid_username`, `reserved_username`, `username_taken`, `invalid_bio`, `bio_too_long`, and `empty_profile_update`. Validation returns 422, username conflicts return 409, missing JWT authentication returns 401, and persistence failures return a generic 500 response.

The existing event detail response still returns organizer_id. The frontend resolves that identifier through the public profile read model; missing profile data is handled as unavailable rather than fabricated.

## Nimiq paid-event payment verification

The paid-event flow stops at a confirmed purchase. It does not issue tickets or QR codes.

- events.price_lunas is the canonical integer price. One NIM is exactly 100,000 Luna.
- POST /api/v1/events/{id}/purchases is authenticated and derives the amount from the locked published event. The client cannot supply amount, recipient, hash, or confirmation.
- GET /api/v1/events/{id}/purchases/current returns the authenticated user's active purchase for refresh/recovery.
- GET /api/v1/purchases/{id} is authenticated and owner-scoped. Submitted and verifying reads trigger a safe, idempotent verification attempt.
- GET /api/v1/purchases/{id}/payment-instructions returns only backend-authoritative purchase_id, recipient, amount_lunas, network, and hold expiry.
- POST /api/v1/purchases/{id}/transaction accepts only a transaction_hash. Hashes are normalized, format-checked, and unique at the database level.
- Purchase states are pending, submitted, verifying, confirmed, failed, expired, and cancelled. Hash submission is never trusted by itself; the server verifier may confirm immediately only when the transfer is already valid and final.
- Verification uses getTransactionByHash, then getBlockByNumber for the containing block and getBatchNumber for finality. Recipient, exact Luna amount, network, basic-transfer fields, execution result, inclusion, and finality are server-checked.
- A transaction in a micro block remains submitted/verifying until its batch has been followed by a later batch, which is the macro-block finality rule used here.
- The frontend uses @nimiq/mini-app-sdk and sendBasicTransaction only after retrieving payment instructions. The wallet owns user confirmation and private keys; NIMNear never receives or stores private keys.
- The frontend polls the owner-scoped purchase state during verification and can recover it by reopening the event. A server-side reconciliation worker also runs when payment verification is enabled; it processes a bounded deterministic batch on a configurable interval and exits cleanly with the server.
- Once a hash is accepted, the capacity hold is pinned by clearing its expiry and counting submitted/verifying purchases as active. An invalid verified transaction moves to failed and releases capacity. A temporary RPC outage, propagation delay, or not-found response does not fail or confirm a purchase before the deadline. At the configured reconciliation deadline, the worker performs one authoritative verifier attempt: a valid finalized transfer becomes confirmed, a conclusively invalid transfer becomes failed, and an unresolved result becomes expired. Expired unresolved payments release capacity and are never silently confirmed later; refunds are not implemented.
- Reconciliation uses database claim timestamps to prevent concurrent frontend reads and workers from issuing duplicate verifier attempts. Candidate selection is bounded and deterministic. The worker logs purchase/event IDs, state transitions, and outcome classifications without private credentials or wallet secrets.
- Reconciliation settings are loaded from `NIMNEAR_PURCHASE_RECONCILIATION_INTERVAL_SECONDS` (default 30), `NIMNEAR_PURCHASE_RECONCILIATION_DEADLINE_MINUTES` (default 60), and `NIMNEAR_PURCHASE_RECONCILIATION_BATCH_SIZE` (default 50). Existing submitted/verifying rows without a dedicated deadline use their last update time plus the configured deadline as a compatibility fallback.
- Configuration is optional in local environments and becomes active only when all three public settings are supplied: NIMNEAR_NIMIQ_NETWORK, NIMNEAR_MERCHANT_ADDRESS, and NIMNEAR_NIMIQ_RPC_URL. Partial or malformed configuration fails startup. No private key is accepted. The Mini App provider does not expose a reliable consensus-network identifier; testnet safety therefore depends on matching the configured backend RPC/network and the Nimiq Pay runtime testnet selection.
- Tickets, QR codes, check-in, refunds, organizer payouts, notifications, and multiple tickets remain unsupported.

## Data bootstrap boundary

Normal server startup, `dev.sh`, Docker Compose, and migrations do not create event, place, profile, participant, purchase, ticket, or calendar records; calendar migrations create schema only. `make seed` is an explicitly invoked development/operations helper that bootstraps only RBAC roles and role permissions. Test fixtures remain isolated to test files and test databases. Postman collections are request tooling and never run as part of application startup. See `DATA_PROVENANCE.md` for the complete provenance audit.
