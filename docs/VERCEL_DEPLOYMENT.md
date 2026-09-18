# Vercel frontend deployment

NIMNear’s primary production frontend target is Vercel. The Next.js app lives in
`frontend/web` and talks to a separately deployed Go API. The standalone
frontend Docker image remains an alternative self-hosted path and is not
removed.

This document does not mean the product has been deployed.

## 1. Create or import the Vercel project

Import the GitHub repository that contains this monorepo.

## 2. Repository

Use the existing NIMNear repository. Do not move `frontend/web` to the
repository root to simplify Vercel.

## 3. Root Directory

Set **Root Directory** to `frontend/web`.

## 4. Framework

Framework Preset: **Next.js**.

Vercel infers install (`npm ci` / `npm install`) and build (`npm run build`).
Do not add a `vercel.json` unless a future constraint requires it.

`next.config.ts` sets `output: "standalone"` only when `VERCEL` is unset, so
Docker/self-hosted builds keep standalone output and Vercel uses its own
output.

## 5. Production environment variables

Set these on the Vercel Production environment. They are public and baked into
the browser bundle.

| Variable | Production value |
| --- | --- |
| `NEXT_PUBLIC_NIMNEAR_API_URL` | `https://api.example.com` (the public HTTPS Go API origin) |
| `NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK` | `main-albatross` when MainAlbatross is activated |
| `NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN` | `https://app.example.com` (canonical frontend origin) |
| `NEXT_PUBLIC_NIMNEAR_HUB_ENABLED` | `false` for Mini App-only production; `true` if Hub browser fallback remains enabled |

Never set through `NEXT_PUBLIC_*`:

- JWT secret
- PostgreSQL DSN or password
- Redis credentials
- Nimiq RPC credentials
- operator secrets

Those belong only on the Go API host.

## 6. Preview environment policy

Vercel preview URLs change (`*.vercel.app`) and must not be added as
`CORS_ALLOWED_ORIGINS=*` or as a wildcard `*.vercel.app` allowlist.

Selected policy: **B**. Authenticated backend features are not considered valid
end-to-end on arbitrary preview URLs. Preview deploys may render public pages
against a staging API only when that staging API lists one explicit preview or
staging origin. Production CORS stays explicit and fail-closed.

Do not use a Vercel preview URL as `NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN` or as
`NIMNEAR_PUBLIC_ORIGIN`. Those values become persistent payment share/QR links.

## 7. Custom domain

Attach the production frontend hostname (for example `app.example.com`) to the
Vercel project. This origin must match:

- backend `CORS_ALLOWED_ORIGINS`
- backend `NIMNEAR_PUBLIC_ORIGIN`
- frontend `NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN`

## 8. Backend API domain

Deploy the Go API on its own HTTPS origin (for example `https://api.example.com`).
The Vercel frontend is not the WebSocket or payment-verification host.

## 9. CORS configuration

Production backend:

```text
CORS_ALLOWED_ORIGINS=https://app.example.com
```

Credentialed requests from the browser continue to use `credentials: include`.
The API sends `Access-Control-Allow-Credentials: true` only with an explicit
origin. Wildcard credentialed CORS is rejected at startup.

## 10. Cookie and CSRF requirements

Cross-origin Vercel → API uses:

- `NIMNEAR_AUTH_COOKIE_SECURE=true`
- `NIMNEAR_AUTH_COOKIE_SAME_SITE=none`

Cookie-authenticated mutations still require `Origin` or `Referer` to match
`CORS_ALLOWED_ORIGINS`. Foreign origins receive 403. Missing Origin/Referer on
cookie mutations is rejected. Bearer clients are unchanged.

## 11. WebSocket endpoint

The browser connects to the Go API:

```text
wss://api.example.com/api/v1/ws
```

The URL is derived from `NEXT_PUBLIC_NIMNEAR_API_URL`, not `window.location`.
Authentication uses the HttpOnly session cookie. Query-string JWTs are not
accepted.

## 12. Canonical payment/share origin

Payment request links and QR codes must resolve to:

```text
https://app.example.com/pay/{public_id}
```

They are built from `NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN` / backend
`NIMNEAR_PUBLIC_ORIGIN`. Production rejects HTTP and `*.vercel.app` origins.

## 13. Nimiq production network

When MainAlbatross is activated:

- frontend `NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK=main-albatross`
- backend `NIMNEAR_AUTH_NETWORK` / `NIMNEAR_NIMIQ_NETWORK` identify main
- Hub production URL is used only if Hub fallback remains enabled

No real NIM transaction is required to complete this documentation.

## 14. Deployment verification

After a production Vercel deploy, verify:

- `https://app.example.com` loads
- `/health` on the frontend responds
- public event/place/profile/calendar/pay routes render
- sign-in cookie is set against the API origin
- a cookie mutation from the Vercel origin succeeds
- a cookie mutation from a foreign origin is 403
- WebSocket connects over WSS to the API
- payment share URLs are HTTPS on the canonical origin

GitHub CI remains authoritative for Go, PostgreSQL, migrations, frontend tests,
typecheck, lint, and build. Vercel must not replace backend CI.

## 15. Rollback procedure

In the Vercel dashboard, promote the previous successful production deployment.
If the API also changed, roll the API image/process back to the matching
release. There is no in-place undo for confirmed payments.
