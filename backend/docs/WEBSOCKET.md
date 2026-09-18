# WebSocket Architecture — nimnear-api

## Overview

Real-time event delivery for multi-tenant clients over a dedicated WebSocket endpoint. Domain events published through the existing `EventBus` (in-process or Kafka) are fanned out to connected clients scoped by organization and application.

```
┌─────────────┐     Publish      ┌──────────────┐     Subscribe    ┌─────────────┐
│  Use Cases  │ ───────────────► │   EventBus   │ ───────────────► │ EventBridge │
└─────────────┘                  └──────────────┘                  └──────┬──────┘
                                                                            │ Broadcast
┌─────────────┐     Upgrade      ┌──────────────┐     Register            ▼
│   Browser   │ ───────────────► │ WS Handler   │ ───────────────► ┌─────────────┐
│   / Client  │ ◄─────────────── │  /api/v1/ws  │ ◄─────────────── │     Hub     │
└─────────────┘     JSON push    └──────────────┘     Rooms        └─────────────┘
```

## Design principles

1. **Separate bounded context** — WebSocket is not part of the HTTP gateway pipeline. It lives in a `realtime` domain with its own hub and handler.
2. **Clean architecture** — Domain defines interfaces; application validates connections; infrastructure implements the hub and upgrade logic.
3. **Multi-tenant isolation** — Every room is keyed by `org_id` + `app_id`. Clients cannot subscribe to rooms outside their validated scope.
4. **Event bus integration** — Reuses the existing `EventBus.Subscribe` pattern; no new transport required for phase 1.
5. **Browser-friendly auth** — Browser clients authenticate with the HttpOnly session cookie on the WebSocket handshake. Non-browser clients may send `Authorization: Bearer`. Query-string JWTs are rejected.

## Endpoint

```
GET /api/v1/ws
```

**Auth resolution order:**

1. Router `JWTAuth` context (session cookie or `Authorization: Bearer`)
2. Handler fallback: `Authorization: Bearer`, then the session cookie

Query parameter `token` is ignored.

**Origin:** In production, browser WebSocket connections must send `Origin` matching `CORS_ALLOWED_ORIGINS`. Empty Origin is rejected unless `WS_ALLOW_EMPTY_ORIGIN=true` (development/non-browser).

**RBAC:** requires `app:read` permission (connecting to an app's event stream).

## Room model

```
org:{organization_id}:app:{app_id}:channel:{channel_name}
```

| Channel | Events delivered |
| ------- | ---------------- |
| `events` | All domain events for the org/app (default subscription on connect) |
| `tenant` | Organization, app, workspace lifecycle |
| `api-management` | Endpoint created, retired, activated |
| `iam` | Role assigned, user registered (org-scoped) |

Channel names must match `^[a-zA-Z0-9_-]{1,64}$`.

## Wire protocol

### Client → Server

```json
{ "action": "subscribe",   "channel": "api-management" }
{ "action": "unsubscribe", "channel": "tenant" }
{ "action": "ping" }
```

### Server → Client

```json
{
  "type": "endpoint.created",
  "topic": "masterfabric.api-management",
  "organization_id": "uuid",
  "app_id": "uuid",
  "data": { ... },
  "timestamp": "2026-07-03T12:00:00Z"
}
```

```json
{ "type": "pong" }
{ "type": "subscribed", "channel": "events" }
{ "type": "error", "message": "invalid channel name" }
```

## Layer map

| Layer | Package | Responsibility |
| ----- | ------- | -------------- |
| Domain | `internal/domain/realtime/model` | Room keys, message types, channel validation |
| Domain | `internal/domain/realtime/service` | `Hub` interface |
| Application | `internal/application/realtime/usecase` | Connection validation (org/app/RBAC) |
| Infrastructure | `internal/infrastructure/websocket` | In-memory hub, client pumps, event bridge |
| Infrastructure | `internal/infrastructure/http/handler/realtime` | HTTP upgrade handler |
| Shared | `internal/shared/middleware` | JWT token extraction for upgrade requests |

## Router placement

```
/api/v1 (protected group)
  ├── JWTAuth
  ├── TenantResolver
  ├── GET /ws          ← registered BEFORE GatewayPipeline.Enforce
  ├── GatewayPipeline.Enforce
  └── ... admin routes
```

`/api/v1/ws` is also added to `shouldSkipPipeline()` so the gateway never intercepts upgrade requests.

## Configuration

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `WS_ENABLED` | `true` | Enable WebSocket endpoint |
| `WS_MAX_CONNECTIONS` | `1000` | Per-process connection cap |
| `WS_PING_INTERVAL_SECONDS` | `30` | Server ping interval |
| `WS_READ_BUFFER_SIZE` | `1024` | WebSocket read buffer (bytes) |
| `WS_ALLOW_EMPTY_ORIGIN` | `true` in development, `false` in production | Allow WebSocket handshakes with no Origin header |

## Phase 2 (not in this branch)

- Redis pub/sub for cross-instance fan-out
- Workspace-scoped channels (`X-Workspace-ID`)
- Audit log entries for connect/disconnect
- API key auth for machine clients
- Backpressure and per-client rate limiting
