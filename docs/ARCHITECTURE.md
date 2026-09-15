# NIMNear — Architecture

## Overview

NIMNear uses a monorepo containing a separate frontend and backend.

```text
nimnear/
├── frontend/
│   └── web/
├── backend/
└── docs/
```

## Frontend

Path: `frontend/web`

Stack:
- Next.js
- App Router
- TypeScript
- Tailwind CSS
- shadcn/ui
- @nimiq/mini-app-sdk
- qrcode
- html5-qrcode

The frontend is deployed separately from the Go backend.

Expected frontend hosting: Vercel.

## Frontend Rendering Model

Use the App Router.

Prefer server components by default for static/data-oriented UI where they are appropriate.

Use client components where required for:
- browser-only APIs;
- geolocation;
- Nimiq Mini App SDK;
- QR camera/scanning;
- interactive local state;
- client-side event handling.

Do not force browser-dependent Nimiq logic into server components.

## Backend

Path: `backend`

Stack:
- Go
- NIMNear Go

The NIMNear backend preserves the existing clean/hexagonal foundation.

Existing infrastructure includes:
- HTTP server/router;
- PostgreSQL;
- Redis;
- Kafka;
- authentication;
- tenant/workspace infrastructure;
- API management;
- audit infrastructure;
- realtime infrastructure.

Do not replace these systems without an explicit architectural decision.

## Data

Primary persistent database: PostgreSQL.

Redis is available for appropriate caching/ephemeral infrastructure.

Kafka is available for asynchronous/event-driven workloads. Kafka should not be introduced into simple request/response flows without a reason.

## Local Architecture

```text
Next.js :3000
     |
     | HTTP
     v
NIMNear API :8080
     |
     +---- PostgreSQL :5432
     +---- Redis :6379
     +---- Kafka :9092
```

Kafka UI is available locally on port 8090.

## Health

Backend health endpoints:
- `GET /health/live`
- `GET /health/ready`

The ready endpoint currently validates PostgreSQL and Redis availability.

## Backend Router

The backend router contains protected middleware scopes.

A previous Chi issue was fixed by ensuring middleware registration occurs before routes on the same mux.

Gateway pipeline scoped routes use a nested Chi group so that WebSocket behavior remains outside the gateway pipeline while retaining its intended authentication/tenant middleware.

Do not flatten or reorder these groups without understanding middleware scope.

## API

The frontend communicates with the backend through HTTP APIs.

Do not invent endpoint contracts in frontend components.

API contracts should eventually be documented separately once the NIMNear domain API is finalized.

## Nimiq

Nimiq Mini App integration belongs primarily at the frontend integration layer unless a particular feature requires backend verification or persistence.

Do not implement custom wallet/key handling when the Nimiq Mini App SDK provides the intended mechanism.

Never expose private keys or secrets to the backend or repository.

## Deployment Direction

Frontend: Vercel.

Backend: separate Go-compatible hosting/container environment.

Possible backend hosting is intentionally not fixed in this document.

PostgreSQL, Redis and Kafka production infrastructure must be configured separately from local Docker infrastructure.

## Paid-event payment boundary

Paid-event money is represented canonically as integer Luna in events.price_lunas. The API exposes decimal price_nim only as presentation data; the frontend never derives the wallet amount from formatted NIM text.

The payment path is:

    authenticated event detail
        -> create/reuse pending purchase
        -> backend payment instructions
        -> Nimiq Pay sendBasicTransaction
        -> hash-only transaction submission
        -> Nimiq RPC lookup and validation
        -> submitted/verifying
        -> later-batch macro finality
        -> confirmed purchase

The backend owns amount, merchant recipient, network, purchase ownership, hash uniqueness, verification, and finality. The frontend owns presentation, wallet invocation, rejection state, and bounded polling. @nimiq/mini-app-sdk is isolated to the client payment component; private keys never enter NIMNear.

The verifier uses the configured JSON-RPC endpoint. It checks the exact recipient and Luna amount, transaction hash, basic transfer fields, execution result, containing micro-block network, inclusion, and later-batch finality. It does not use a confirmation-count heuristic. Not-found/RPC-unavailable results stay retryable; non-final inclusion becomes verifying; only a finalized valid transfer becomes confirmed.

Purchase state transitions are server-owned:

    pending -> submitted -> verifying -> confirmed
    pending -> expired
    submitted/verifying -> failed

The hash submission operation is idempotent for the same owner/hash. A unique database index prevents replaying one hash across purchases. Submitted/verifying purchases remain active for capacity and have their hold expiry cleared, preventing money from finalizing after a silently released capacity hold. Ticket, QR, check-in, refund, payout, notification, entitlement, and background-worker domains are intentionally outside this phase.
