# NIMNear Backend

The backend is the Go API for the NIMNear Mini App. It is a modular monolith
with domain code layered over the repository's clean/hexagonal foundation.

## Runtime stack

- Go
- Chi HTTP router
- PostgreSQL via pgx
- Redis for infrastructure/cache/ephemeral concerns
- Kafka as optional event infrastructure
- goose-compatible SQL migrations
- bcrypt and JWT for the current legacy authentication boundary
- Prometheus metrics and structured logging

The backend also retains tenant, workspace, RBAC, API-management, audit, and
WebSocket infrastructure from the original platform. Those systems are not
NIMNear product records. In production they are disabled unless
`NIMNEAR_PLATFORM_API_ENABLED=true`. See `docs/PRODUCT_BOUNDARIES.md`.

## Local development

    cd backend
    ./dev.sh          # Docker infrastructure, migrations, and development server
    ./dev.sh infra    # infrastructure only
    ./dev.sh migrate  # migrations only
    ./dev.sh server   # server only
    ./dev.sh down     # stop local services

The normal local API is http://localhost:8080.

    curl http://localhost:8080/health/live
    curl http://localhost:8080/health/ready

The readiness response checks PostgreSQL and Redis. Local Docker services and
their credentials are development-only. make seed is an explicit RBAC
bootstrap helper; `make seed-dev-places` is an explicit fictional
development-place helper. Startup and migrations do not create application
events, places, calendars, profiles, purchases, tickets, or demo records.

The latest migration is
backend/internal/infrastructure/postgres/migrations/00023_payment_reconciliation.sql.
Migrations through 00023 are the current schema, including places, events,
participants, profiles, canonical Luna prices, purchases, calendars, event
calendar association, and payment reconciliation state.

## Environment and fail-closed rules

APP_ENV is development, test, or production and defaults to development for
the existing local workflow.

- development keeps the documented local PostgreSQL and JWT defaults.
- test uses isolated test configuration/fixtures and must not depend on
  production secrets.
- production rejects missing/default JWT secret or issuer, missing/default
  database host/user/password/name, non-TLS PostgreSQL mode, invalid ports,
  empty/wildcard CORS, and unknown environment values before infrastructure
  initialization.

Payment configuration is optional when all three public settings are absent.
If enabled, these variables must be supplied together:

- NIMNEAR_NIMIQ_NETWORK
- NIMNEAR_MERCHANT_ADDRESS
- NIMNEAR_NIMIQ_RPC_URL

Local `./dev.sh` defaults those three to TestAlbatross development values,
including the public rate-limited endpoint `https://rpc.testnet.nimiqwatch.com`.
`https://rpc.nimiqwatch.com` serves MainAlbatross and must not be used with the
test-albatross development network. That public endpoint is not production RPC
infrastructure.

Production payment validation rejects obvious test/local network mixing,
loopback/local RPC hosts, malformed addresses, and ambiguous network/RPC
identifiers. No private key is accepted by configuration. Payment routing
currently remains the global merchant-address design.

The main settings loaded by backend/internal/shared/config/config.go are:

| Area | Variables |
| --- | --- |
| Application/server | APP_ENV, SERVER_HOST, SERVER_PORT, SERVER_READ_TIMEOUT_SECONDS, SERVER_WRITE_TIMEOUT_SECONDS, SERVER_IDLE_TIMEOUT_SECONDS, CORS_ALLOWED_ORIGINS, MAX_BODY_BYTES, NIMNEAR_TRUSTED_PROXY_CIDRS |
| Product/legacy surfaces | NIMNEAR_EMAIL_AUTH_ENABLED, NIMNEAR_PLATFORM_API_ENABLED, NIMNEAR_GATEWAY_TABLE_ALLOWLIST |
| Payments | NIMNEAR_PURCHASE_HOLD_MINUTES, NIMNEAR_PURCHASE_RECONCILIATION_INTERVAL_SECONDS, NIMNEAR_PURCHASE_RECONCILIATION_DEADLINE_MINUTES, NIMNEAR_PURCHASE_RECONCILIATION_BATCH_SIZE, NIMNEAR_PURCHASE_CREATE_LIMIT, NIMNEAR_PURCHASE_SUBMIT_LIMIT, NIMNEAR_PURCHASE_RATE_LIMIT_WINDOW_SECONDS, plus the three Nimiq variables above |
| PostgreSQL | DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE, DB_MAX_CONNS, DB_MIN_CONNS |
| Redis | REDIS_HOST, REDIS_PORT, REDIS_PASSWORD, REDIS_DB |
| JWT | JWT_SECRET, JWT_EXPIRATION_HOURS, JWT_ISSUER |
| Kafka | KAFKA_BROKERS, KAFKA_GROUP_ID, KAFKA_ENABLED, KAFKA_NUM_PARTITIONS, KAFKA_REPLICATION_FACTOR |
| WebSocket | WS_ENABLED, WS_MAX_CONNECTIONS, WS_PING_INTERVAL_SECONDS, WS_READ_BUFFER_SIZE, WS_WRITE_BUFFER_SIZE |
| Nimiq auth | NIMNEAR_AUTH_NETWORK, NIMNEAR_AUTH_ENVIRONMENT, NIMNEAR_AUTH_DOMAIN, NIMNEAR_AUTH_CHALLENGE_TTL_SECONDS, NIMNEAR_AUTH_COOKIE_NAME, NIMNEAR_AUTH_COOKIE_SECURE, NIMNEAR_AUTH_COOKIE_SAME_SITE, NIMNEAR_AUTH_CHALLENGE_IP_LIMIT, NIMNEAR_AUTH_VERIFY_IP_LIMIT, NIMNEAR_AUTH_VERIFY_ABUSE_LIMIT, NIMNEAR_AUTH_RATE_LIMIT_WINDOW_SECONDS |
| Logging | LOG_LEVEL, LOG_FORMAT |

Validation errors identify configuration variable names only; they do not print
secret values or full connection strings.

## NIMNear API

### Public

- GET /health/live
- GET /health/ready
- GET /metrics — registered only when `NIMNEAR_METRICS_ENABLED=true` and `NIMNEAR_METRICS_PUBLIC=true`. Production default: not public.
- POST /api/v1/auth/register — legacy email/password registration. Production default: disabled.
- POST /api/v1/auth/login — browser cookie session; JSON is `{ "user": ... }` only. Production default: disabled.
- POST /api/v1/auth/token — explicit Bearer JWT issuance for non-browser API clients. Production default: disabled.
- POST /api/v1/auth/logout — clears the HttpOnly session cookie.
- POST /api/v1/auth/nimiq/challenges — Nimiq AUTH_LOGIN challenge.
- POST /api/v1/auth/nimiq/verify — verifies the wallet signature and sets the session cookie.
- GET /api/v1/places/nearby?lat={lat}&lng={lng}&radius={meters}
- GET /api/v1/places/{id}
- GET /api/v1/events
- GET /api/v1/events/{id}
- GET /api/v1/calendars
- GET /api/v1/calendars/{id}
- GET /api/v1/profiles/{id}
- GET /api/v1/profiles/{id}/events?type=organized|attended

GET /api/v1/events returns published public events ordered by starts_at ASC,
id ASC. With no from or to, the default lower bound is current UTC time.
limit is 1–100 and defaults to 20. place_id and case-insensitive city filters
are applied by the API.

Nearby places require latitude and longitude, default to a 5 km radius, cap the
radius at 50 km, and return active places nearest-first.

### JWT-protected NIMNear operations

Protected NIMNear routes accept the HttpOnly session cookie or a Bearer JWT.
The Mini App uses the cookie.

- GET /api/v1/me
- PATCH /api/v1/me/profile
- DELETE /api/v1/me — anonymize/deactivate the current user; see docs/ACCOUNT_DELETION.md
- POST /api/v1/events
- GET /api/v1/me/calendars
- POST /api/v1/calendars
- PATCH /api/v1/calendars/{id}
- POST /api/v1/calendars/{id}/archive
- POST /api/v1/calendars/{id}/follow
- DELETE /api/v1/calendars/{id}/follow
- GET /api/v1/events/{id}/rsvp
- POST /api/v1/events/{id}/rsvp
- DELETE /api/v1/events/{id}/rsvp
- GET /api/v1/events/{id}/purchases/current
- POST /api/v1/events/{id}/purchases
- GET /api/v1/purchases/{id}
- GET /api/v1/purchases/{id}/payment-instructions
- POST /api/v1/purchases/{id}/transaction
- POST /api/v1/purchases/{id}/reverify
- POST /api/v1/payment-requests/{public_id}/reverify
- GET /api/v1/ws?token=<jwt>

The existing tenant/RBAC/API-management routes are leftover MasterFabric
platform APIs. They are disabled in production by default and are separate
from the NIMNear product. Place inventory is operator-managed; see
`docs/PLACES_OPERATIONS.md` and `docs/PRODUCT_BOUNDARIES.md`.

## Domain contract

Events store price_lunas as an integer. One NIM equals 100,000 Luna.
price_nim is an edge/request or presentation field and is converted exactly;
the API rejects excess precision rather than rounding. Optional event fields
(capacity, image_url, place_id, latitude, longitude, address, organizer_id)
and optional public profile fields (username, bio, avatar_url) are serialized
as JSON null when absent.

Event creation is JWT-protected. The authenticated subject becomes organizer_id.
An active owned calendar or active place may be attached; an active place cannot
be combined with custom address/coordinates. Empty capacity means unlimited.
External media is an absolute HTTP(S) URL subject to the current
length/scheme/control-character validation; no upload/object-storage provider
or approved-host allowlist exists yet.

Profiles expose only the public profile read model and organized/attended public
event history. Email, password, JWT claims, and tenant metadata are private.
PATCH /api/v1/me/profile accepts only display name, username, and plain-text bio
with stable validation/conflict errors. Avatar mutation is not exposed.

## Payment verification

The current paid flow is:

    JWT-authenticated purchase
      -> backend derives amount_lunas and global merchant recipient
      -> frontend receives payment instructions
      -> Nimiq Pay sendBasicTransaction
      -> frontend submits only transaction_hash
      -> backend RPC verifier checks the transfer
      -> submitted/verifying
      -> macro-block finality
      -> confirmed

The verifier checks transaction lookup, exact recipient and amount, RPC sender
against the purchaser's verified Nimiq identities, basic transfer fields,
execution result, containing block/network, inclusion, and later-batch
finality. A reconciliation worker retries submitted/verifying purchases with a
bounded deterministic batch and deadline. The same owner/hash submission is
idempotent and a unique database index prevents cross-purchase hash replay.
Submitted/verifying purchases remain capacity-protected.

The current recipient is NIMNEAR_MERCHANT_ADDRESS. Verified organizer identity,
organizer recipient snapshots, entitlements, tickets, QR/check-in, refunds,
payouts, and payment-to-organizer routing are future work. NIMNear does not
handle private keys.

## Authentication boundary

Nimiq wallet authentication is implemented for Testnet. Challenge/verify
endpoints issue an HttpOnly session cookie and return `{ "user": ... }` only.
Mini App requests use `credentials: include` and do not persist a JWT.
`POST /api/v1/auth/token` issues a Bearer JWT for non-browser API clients.
See docs/NIMIQ_AUTH_IMPLEMENTATION.md and docs/NIMIQ_RPC.md. Device identifiers,
RPC WebSocket listeners, generic payments, transaction history, and deep links
are not implemented.

## Testing

    go test ./...
    go vet ./...

Destructive PostgreSQL tests require an isolated `*_test` database:

    NIMNEAR_TEST_DATABASE_URL=postgres://nimnear:nimnear@localhost:5432/nimnear_test?sslmode=disable \
      make test-postgres

For frontend validation, see frontend/web/README.md. Manual Postman
collections are request tooling only; they are not startup seeds.
