# NIMNear Nimiq wallet authentication

Implemented: 2026-09-17. The development deployment is pinned to Nimiq Testnet.

## HTTP contract

`POST /api/v1/auth/nimiq/challenges` accepts:

```json
{
  "wallet_address": "NQ...",
  "network": "test-albatross",
  "environment": "testnet",
  "purpose": "AUTH_LOGIN",
  "transport": "mini-app"
}
```

`transport` is either `mini-app` or `hub`. The response contains
`challenge_id`, the exact server-generated `message`, the canonical wallet
address, network, environment, purpose, transport, and issue/expiry times. The
message binds domain, audience, environment, network, address, purpose, a
32-byte random nonce, and server timestamps. The TTL defaults to five minutes.

`POST /api/v1/auth/nimiq/verify` accepts the challenge ID, the exact message, a
32-byte public key as hex, and a 64-byte signature as hex. It does not trust a
client-supplied address during verification.

The server derives the Nimiq address from the public key with Blake2b-256 and
Nimiq user-friendly address encoding, compares it with the stored challenge,
verifies Ed25519, atomically consumes the challenge, resolves or creates the
wallet identity, and mints an HttpOnly session cookie. Browser login and Nimiq
verify JSON is `{ "user": ... }` only; the JWT is not included. The Mini App
does not store a JWT in sessionStorage or localStorage. Non-browser API clients
that need a Bearer token must call `POST /api/v1/auth/token` explicitly.

## Signature transports

- Nimiq Pay: `init()`, `listAccounts()`, then `sign(message)`. The Mini App
  provider returns public key and signature hex. Verification uses the UTF-8
  bytes supplied to the provider.
- Browser: `HubApi` uses the official Hub for the configured Nimiq network (`https://hub.nimiq-testnet.com` for TestAlbatross, `https://hub.nimiq.com` for MainAlbatross). Address
  selection uses `chooseAddress()`. Signing uses `signMessage()` with the
  selected address. Hub signatures are verified over SHA-256 of the official
  `\x16Nimiq Signed Message:\n<length><message>` payload. Hub byte arrays are
  normalized to lower-case hex at the wallet adapter boundary.

The browser flow uses Hub `RedirectRequestBehavior`, not popups. The current
Testnet Hub popup path opens a window, waits one second for a `postMessage`
handshake, and renders `/request-error` ("Invalid request") if `window.opener`
is missing. Redirect encodes `choose-address` / `sign-message` in the Hub URL
hash, so Hub receives a valid request on load. Address selection is one user
gesture. After Hub returns, NIMNear fetches the wallet-bound AUTH_LOGIN
challenge. A second user gesture starts `signMessage()` as another redirect.
Pending challenge state is stored in `sessionStorage` across the return. This
avoids popup activation and opener loss without weakening wallet binding.

## Session and local development

The cookie defaults to `Path=/`, `HttpOnly`, `SameSite=Lax`, and `Secure=false`
for HTTP development. Production defaults to `Secure=true` and `SameSite=None`
because the Vercel frontend and API host are cross-site; operators can still
set `NIMNEAR_AUTH_COOKIE_SAME_SITE=lax` when frontend and API share a site.
Production configuration rejects `Secure=false`. `SameSite=None` also requires
`Secure=true`. LAN frontend and backend ports on the same IP are same-site;
fetch requests use `credentials: include`, and the backend allows credentials
only for explicit CORS origins.

Challenge and verify endpoints are rate-limited with the existing Redis
fixed-window counter (in-memory fallback when Redis is absent). Challenge is
limited per IP, with the claimed address used only as an abuse key. Verify is
limited per IP plus server-stored challenge/address abuse keys. Exceeding a
limit returns HTTP 429 `authentication_rate_limited`.

The browser derives the API hostname from the hostname used to open the app and
uses port 8080. Server rendering uses `NIMNEAR_API_URL` or localhost. `dev.sh`
automatically adds the machine's current LAN IPv4 address to CORS origins.

Configuration variables:

- `NIMNEAR_AUTH_NETWORK` (default `test-albatross`; production must be `main-albatross`)
- `NIMNEAR_AUTH_ENVIRONMENT` (must match the auth network: `testnet` or `mainnet`)
- `NIMNEAR_NIMIQ_NETWORK` (when payments are enabled, must be the same Albatross environment as auth)
- `NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK` (frontend Hub/challenge network; allowlisted `test-albatross` or `main-albatross`)
- `NIMNEAR_AUTH_DOMAIN` (default `nimnear.local`)
- `NIMNEAR_AUTH_CHALLENGE_TTL_SECONDS` (1 to 300)
- `NIMNEAR_AUTH_COOKIE_NAME`
- `NIMNEAR_AUTH_COOKIE_SECURE`
- `NIMNEAR_AUTH_COOKIE_SAME_SITE`
- `NIMNEAR_AUTH_CHALLENGE_IP_LIMIT` (default 10 / window)
- `NIMNEAR_AUTH_VERIFY_IP_LIMIT` (default 10 / window)
- `NIMNEAR_AUTH_VERIFY_ABUSE_LIMIT` (default 5 / window)
- `NIMNEAR_AUTH_RATE_LIMIT_WINDOW_SECONDS` (default 60)
- `CORS_ALLOWED_ORIGINS`

## Identity and authorization

A verified `(network, address, public key)` maps to one internal `users.id`. All
customer resources, event organizer ownership, profile updates, purchases, and
calendar ownership continue to use that authenticated user ID from middleware.
There is no provider-specific login and changing a frontend resource ID does not
change the authenticated owner.
