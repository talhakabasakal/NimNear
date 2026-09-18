# Nimiq RPC production contract

This is an operational contract, not a vendor recommendation. NIMNear does not
ship, hardcode, or endorse a third-party MainAlbatross RPC provider.

## When payments are enabled

Payments are enabled only when all three public settings are present and valid:

- `NIMNEAR_NIMIQ_NETWORK`
- `NIMNEAR_MERCHANT_ADDRESS`
- `NIMNEAR_NIMIQ_RPC_URL`

Partial configuration fails startup. No private key is accepted.

## Network

- Production payments require `MainAlbatross`.
- `TestAlbatross` is development/testnet only.
- Public Nimiqwatch TestAlbatross RPC is a development convenience. It is not a
  production dependency and has no claimed SLA.
- The configured RPC consensus network must match `NIMNEAR_NIMIQ_NETWORK`.
- Auth and payment network settings must agree (P0 fail-closed).

Choosing a dedicated, managed, or self-operated MainAlbatross RPC is a
deployment decision. NIMNear does not select that operator.

## Authentication

If the RPC requires HTTP basic auth, put credentials in the URL userinfo.
The client strips userinfo from the stored endpoint and never logs it. Do not
put RPC passwords in application logs, metrics labels, or client JSON.

## Timeouts

JSON-RPC calls use a 10-second HTTP timeout. Readiness probes use a 3-second
timeout and cache `getBatchNumber` for 20 seconds so `/health/ready` does not
hammer RPC. Do not add per-request RPC probes on normal API traffic.

## Health monitoring

`GET /health/live` is process liveness.

`GET /health/ready` is core process readiness: PostgreSQL and Redis.

When payments are enabled, `/health/ready` also reports `services.nimiq_rpc` as
`healthy` or `unhealthy`. **An RPC outage does not fail `/ready`.** Taking the
process out of rotation would block free-event, auth, and other non-payment
traffic. Payments already fail closed when the verifier/RPC is unavailable.

Operators must alert on `services.nimiq_rpc` independently of the HTTP status.

`GET /health/payments` is payment readiness. It exposes only non-secret fields:
`payment_configured`, canonical `network` (`MainAlbatross` or `TestAlbatross`),
`rpc_configured`, `merchant_address_configured`, `websocket_enabled`, and the
same service map as `/health/ready`. It never returns the merchant address
value, RPC URL, credentials, tokens, cookies, JWTs, or private keys.

- `200` + `status=ready` when payments are configured and RPC is healthy
- `200` + `status=not_configured` when the payment trio is unset
- `503` + `status=not_ready` when PostgreSQL/Redis are down, or payments are
  configured but RPC is unhealthy

Unlike `/health/ready`, this endpoint does fail when a configured RPC is
unreachable, because it is specifically a payment smoke-test probe.

Metrics (low-cardinality labels `method` and `outcome` only):

- `nimnear_nimiq_rpc_requests_total`
- `nimnear_nimiq_rpc_request_duration_seconds`

Never label metrics with RPC URL, username/password, transaction hash, wallet
address, or user ID.

## Rate limits

RPC providers enforce their own rate limits. NIMNear application limits
(EventPurchase create/submit, PaymentRequest, Nimiq auth) are separate and do
not replace provider quotas. Size the RPC capacity for verification, wallet
reads, reconciliation, and occasional expired-paid recovery. Do not use the
RPC to broadcast application-created transactions; wallets sign and broadcast.

## Incident behavior

If RPC is down or not final:

- do not mark a payment paid/confirmed
- keep submitted/verifying rows retryable until the reconciliation deadline
- after the deadline, unresolved payments expire
- consumed transaction hashes remain reserved for that domain row

Fail-closed payment APIs return `payment_not_configured` or transient
verification outcomes. They never invent a paid state from a client hash.

## Reconciliation and recovery

The reconciliation worker retries submitted/verifying payments until the
deadline. Expired payments with a reserved hash can be re-verified through:

- `POST /api/v1/purchases/{id}/reverify` (purchase owner)
- `POST /api/v1/payment-requests/{public_id}/reverify` (creator or payer)
- `go run ./cmd/recover-payment <event-purchase|payment-request> <id>`

Recovery becomes paid/confirmed only when the same full verifier succeeds
(transaction exists, execution true, sender belongs to the payer, recipient
exact, amount exact, network exact, included, Albatross-final, and the consumed
hash still belongs to that same domain row). Recovery is idempotent. It cannot
attach a new hash to an expired row.

## Trusted reverse proxies

`NIMNEAR_TRUSTED_PROXY_CIDRS` is an explicit IPv4/IPv6 address or CIDR
allowlist of immediate peers allowed to supply `X-Forwarded-For` / `X-Real-IP`.
Empty (the development default) means forwarded headers are ignored. Invalid
values fail startup. Private ranges are not trusted automatically.

## Isolated PostgreSQL tests

Destructive repository tests require both:

- `NIMNEAR_TEST_DATABASE_URL` pointing at a database whose name ends with `_test`
- `NIMNEAR_TEST_DB_ISOLATED=true`

Docker Compose creates `nimnear_test` on first Postgres volume init. Existing
volumes need `CREATE DATABASE nimnear_test;` once. Run:

```
NIMNEAR_TEST_DATABASE_URL=postgres://nimnear:nimnear@localhost:5432/nimnear_test?sslmode=disable \
  make test-postgres
```
