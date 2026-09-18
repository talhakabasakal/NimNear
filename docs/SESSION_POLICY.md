# Session policy

NIMNear uses an HttpOnly session cookie as first-class authentication for the
Mini App and browser. `POST /api/v1/auth/token` issues a Bearer JWT only when
legacy email/password auth is enabled. Production startup rejects email auth
(`NIMNEAR_EMAIL_AUTH_ENABLED` must be false).

There is no jti denylist and no session-version platform. Ordinary Bearer
tokens are a non-browser API contract, not the production Mini App session.

## Cookie logout

`POST /api/v1/auth/logout` expires the host-only session cookie immediately.
Browser and WebSocket requests that relied on that cookie are unauthenticated
afterward.

## Bearer logout

A previously issued Bearer JWT remains cryptographically valid until expiry
after ordinary logout. Clients must discard the token. This is accepted for
the legacy/API boundary; it is not a Mini App production logout gap.

AUDIT-007 is closed as this documented boundary, not as a denylist.

## Deleted or deactivated users

Account deletion is stronger than logout. Authenticated lookups reject inactive
or deleted users, including HTTP and WebSocket requests that still present a
previously valid cookie or Bearer JWT.

Production continues to disable email/password auth. Nimiq login of a revoked
identity does not recreate the deleted account.

## JWT storage

JWTs are not placed in URLs, WebSocket query strings, localStorage,
sessionStorage, share links, or QR codes. The browser session is the HttpOnly
cookie.
