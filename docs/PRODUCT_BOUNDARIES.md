# NIMNear product boundaries

NIMNear is a Nimiq Mini App for nearby place discovery, events, calendars,
profiles, RSVP, paid event purchases, wallets, and peer payment requests.

The Go backend still contains leftover MasterFabric multi-tenant platform
code. That platform is not the NIMNear product.

## Active NIMNear product surfaces

- Health: `GET /health/live`, `GET /health/ready`
- Nimiq auth: `POST /api/v1/auth/nimiq/challenges`, `POST /api/v1/auth/nimiq/verify`, `POST /api/v1/auth/logout`
- Session identity: `GET /api/v1/me`, `PATCH /api/v1/me/profile`, `DELETE /api/v1/me`
- Places: `GET /api/v1/places/nearby`, `GET /api/v1/places/{id}`
- Events: public list/detail and authenticated create/update/cancel
- Calendars: public list/detail and authenticated create/update/archive/follow
- RSVP: `GET|POST|DELETE /api/v1/events/{id}/rsvp`
- Event purchases and payment instructions
- Payment requests
- Wallet reads
- WebSocket `/api/v1/ws`

Place writes are operator CLI, not a public API. See `PLACES_OPERATIONS.md`.
Event takedown is operator CLI `manage-event`, not a dashboard. See `EVENT_OPERATIONS.md`.
Account deletion is documented in `ACCOUNT_DELETION.md`. Session cookie vs Bearer behavior is in `SESSION_POLICY.md`.

## Legacy MasterFabric platform

Organizations, apps, managed endpoints, the dynamic SQL gateway, platform user
listing, role assignment, and audit-log listing exist only as inherited
platform functionality. The Mini App does not use them.

Production default: `NIMNEAR_PLATFORM_API_ENABLED=false`.

Development may leave the platform enabled. Enabling it in production also
requires an explicit `NIMNEAR_GATEWAY_TABLE_ALLOWLIST`. The allowlist is empty
by default and the gateway fails closed. Security, payment, and NIMNear
product tables are denied even if listed.

## Email/password auth

`POST /api/v1/auth/register`, `/login`, and `/token` are legacy IAM for API
compatibility. The Mini App signs in with Nimiq.

- Development default: enabled
- Production default: disabled (`NIMNEAR_EMAIL_AUTH_ENABLED=false`)
- When disabled, those three routes are not registered and handlers return 404
- Nimiq auth is unaffected
- When enabled, the endpoints are rate-limited by trusted client IP plus a
  hashed normalized email identity and return the existing 429 envelope

## Production defaults

| Flag | Development | Production unless explicitly enabled |
| --- | --- | --- |
| `NIMNEAR_EMAIL_AUTH_ENABLED` | true | false |
| `NIMNEAR_PLATFORM_API_ENABLED` | true | false |
| `NIMNEAR_METRICS_ENABLED` | true | true |
| `NIMNEAR_METRICS_PUBLIC` | true | false |
| `NIMNEAR_GATEWAY_TABLE_ALLOWLIST` | empty | empty (fail closed) |

## Nimiq freeze

Nimiq integration is closed. Do not change verifier sender binding, recipient
verification, Luna conversion, network verification, finality,
`consumed_nimiq_transactions`, payment recovery, or Mini App transaction
semantics as part of NIMNear product work.

Event payments still go to `NIMNEAR_MERCHANT_ADDRESS`. Changing that to the
event organizer is a financial product decision covering organizer identity,
payout ownership, refunds, fees, disputes, event edits, and recipient
snapshots. It is not a quick patch.

Known pre-production Nimiq items, not a new Nimiq phase:

- production MainAlbatross RPC
- merchant IBAN checksum
- HTTPS pay/QR enforcement
- Hub production policy
- Redis production policy
- live Nimiq Pay E2E
- event merchant-versus-organizer payout product decision
