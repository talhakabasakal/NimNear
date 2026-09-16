# Managed Gateway Handlers

This document describes the legacy managed-endpoint gateway in
backend/internal/gateway. It is infrastructure for explicitly configured
organization/app endpoints, not the NIMNear event, place, calendar, profile,
RSVP, or purchase API.

## Scope

The explicit NIMNear domain routes are registered in
backend/internal/infrastructure/http/router/router.go and use domain handlers,
repositories, DTOs, and validation. Do not route those features through a
generic product/example handler.

The gateway pipeline handles a request only when tenant/app context and a
matching managed endpoint definition are present. An unregistered managed path
is a 404; it is not a dynamic NIMNear product collection.

## Resolution order

DynamicHandlerResolver resolves a managed endpoint in this order:

1. an explicitly registered BackendHandler;
2. an HTTP proxy when the endpoint service is a URL or configured mapping;
3. the generic dynamic handler when endpoint metadata supports the operation.

A custom handler must be registered deliberately and must read authoritative
data. The repository contains no shipped sample handler that returns Product 1,
Product 2, new-id, or other fabricated records.

## Managed endpoint requirements

Managed endpoint definitions carry method/path, service/action metadata, and
optional schema/policy configuration. The gateway applies its existing
authentication, tenant, RBAC, validation, and rate-limit pipeline before
dispatching.

Use a managed endpoint only for a separately defined platform integration.
NIMNear domain APIs must be added to the explicit router and documented in
docs/BACKEND.md.

## Safety boundary

- Never put credentials, private keys, seed phrases, or production URLs in a
  handler example.
- Never treat generic dynamic responses as NIMNear application records.
- Never use the gateway to bypass JWT authorization on protected NIMNear
  operations.
- The gateway has no role in Nimiq Pay account connection or the unresolved
  Nimiq signature-to-JWT contract.
