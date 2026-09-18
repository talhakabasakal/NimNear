# Release checklist

This is an operator checklist for assembling a NIMNear deployment. Completing
it does not mean the product is production-ready. Known Nimiq production work
remains outside this list.

## Backend config

- [ ] `APP_ENV=production`
- [ ] `JWT_SECRET` and `JWT_ISSUER` are explicit, unique, and not development defaults
- [ ] PostgreSQL host, user, password, database name, and `DB_SSLMODE` require TLS
- [ ] Redis is reachable; production startup fails if Redis is unavailable
- [ ] `CORS_ALLOWED_ORIGINS` lists the frontend HTTPS origin only
- [ ] `NIMNEAR_PUBLIC_ORIGIN` is the canonical HTTPS frontend origin and matches CORS
- [ ] `NIMNEAR_AUTH_COOKIE_SECURE=true`
- [ ] Cookie SameSite matches the frontend/API HTTPS layout
- [ ] `NIMNEAR_EMAIL_AUTH_ENABLED=false`
- [ ] `NIMNEAR_PLATFORM_API_ENABLED=false`
- [ ] `NIMNEAR_METRICS_PUBLIC=false`
- [ ] Nimiq auth network/domain match the intended deployment
- [ ] Payment settings are all present together, or all absent
- [ ] `NIMNEAR_MERCHANT_ADDRESS` passes Nimiq IBAN checksum when payments are enabled
- [ ] `WS_ALLOW_EMPTY_ORIGIN=false` in production

## Frontend config

- [ ] Production image or Vercel project built with the public API origin
- [ ] `NEXT_PUBLIC_NIMNEAR_API_URL` equals the browser-reachable API origin
- [ ] `NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN` equals the canonical HTTPS frontend origin
- [ ] `NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK` matches backend `NIMNEAR_AUTH_NETWORK`
- [ ] `NEXT_PUBLIC_NIMNEAR_HUB_ENABLED=false` if Mini App-only production is selected
- [ ] No JWT, database, Redis, or RPC secrets in frontend env or image

## Data

- [ ] Goose migrations applied to a fresh production database
- [ ] `nimnear_test` is not used as the application database
- [ ] Place inventory loaded through `manage-place` if discovery should be non-empty
- [ ] Backup and restore of PostgreSQL is defined before first traffic
- [ ] Rollback plan is "restore database + previous images"; there is no in-place undo for payments

## Operator paths

- [ ] `manage-place` is available to operators
- [ ] `manage-event list|get|cancel` is available for abusive/invalid public events
- [ ] Operators understand cancel preserves RSVP, purchases, and hashes and does not refund

## Nimiq known production requirements

Carry forward; do not treat as done by this checklist:

- live Nimiq Pay E2E
- merchant versus organizer payout decision
- Hub residual advisory if Hub fallback remains enabled

See `docs/VERCEL_DEPLOYMENT.md` for the primary Vercel frontend path.
The standalone frontend Docker image remains an alternative.

## Verification

- [ ] CI is green on the release commit
- [ ] Isolated PostgreSQL tests ran (`NIMNEAR_TEST_DB_ISOLATED=true`)
- [ ] `/health/live` and `/health/ready` succeed
- [ ] `/metrics` is not publicly reachable in production
- [ ] Account deletion, calendar archive, and event cancel were exercised in a staging environment
