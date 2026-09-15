# NIMNear — AI Agent Instructions

## Purpose

This file is the primary entry point for AI coding agents working on NIMNear.

Before making changes, understand the task and read the relevant documentation under `/docs`.

Do not assume missing product, design, API, or architecture decisions.

## Project

NIMNear is a location-based discovery application built as a Nimiq Mini App.

The application helps users discover nearby places and experiences and will integrate Nimiq functionality where appropriate.

The project is currently under active development.

## Repository Structure

```text
nimnear/
├── frontend/
│   └── web/
├── backend/
├── docs/
└── AGENTS.md
```

## Technology Stack

### Frontend
- Next.js
- App Router
- TypeScript
- Tailwind CSS
- shadcn/ui
- @nimiq/mini-app-sdk
- qrcode
- html5-qrcode

### Backend
- Go
- NIMNear API Go
- PostgreSQL
- Redis
- Kafka

### Infrastructure

Local development uses Docker Compose for infrastructure services.

Default local services:
- Frontend: http://localhost:3000
- Backend: http://localhost:8080
- PostgreSQL: localhost:5432
- Redis: localhost:6379
- Kafka: localhost:9092
- Kafka UI: localhost:8090

## Documentation

Read documentation based on the task.

- Product behavior: `docs/PRODUCT.md`
- Architecture: `docs/ARCHITECTURE.md`
- Frontend implementation: `docs/FRONTEND.md`
- Backend implementation: `docs/BACKEND.md`
- Design and Figma: `docs/DESIGN_SYSTEM.md`
- User journeys: `docs/USER_FLOWS.md`

## Figma

Figma is the source of truth for visual design.

When Figma MCP is available:
1. Inspect the referenced Figma frame before implementing it.
2. Inspect layout, spacing, typography, colors, components and states.
3. Reuse design variables/tokens where possible.
4. Do not redesign the screen unless explicitly requested.
5. Do not replace intentional Figma decisions with arbitrary defaults.
6. Prefer reusable React components over duplicating Figma-generated markup.
7. Use shadcn/ui as a component foundation when it fits, but never force the Figma design into shadcn's default styling.

If documentation conflicts with the latest explicitly referenced Figma design on a visual matter, flag the conflict before making a broad design change.

## General Engineering Rules

Before modifying code:
1. Inspect existing implementation.
2. Read relevant documentation.
3. Understand existing conventions.
4. Prefer minimal changes.
5. Reuse existing abstractions and components.

Do not:
- introduce frameworks without justification;
- perform unrelated refactors;
- hardcode secrets;
- commit `.env`;
- duplicate existing components;
- invent backend contracts;
- invent product behavior;
- rewrite working NIMNear infrastructure unnecessarily.

## Frontend Rules

Frontend lives in `frontend/web`.

The frontend is a Next.js App Router project.

Keep the application compatible with a Nimiq Mini App / WebView environment.

Prefer:
- small reusable components;
- strict TypeScript;
- responsive/mobile-first layouts;
- semantic HTML;
- accessible interactions;
- design tokens instead of repeated arbitrary values;
- server components by default where practical;
- client components only where browser APIs, interactivity, Nimiq SDK, geolocation, QR scanning, or stateful UI require them.

Do not use Vite conventions after migration.

## shadcn/ui Rules

- Treat shadcn/ui components as source code owned by the project.
- Customize generated components to match the Figma design.
- Do not preserve default shadcn appearance if it conflicts with the design.
- Prefer composition over creating many near-duplicate primitives.
- Keep shared UI primitives under `frontend/web/components/ui` unless the project establishes another convention.

## Backend Rules

Backend lives in `backend`.

The backend preserves its existing clean/hexagonal architecture and infrastructure. Preserve that architecture unless a task explicitly requires an architectural change.

Existing infrastructure includes PostgreSQL, Redis, Kafka, authentication, tenant/workspace infrastructure, API management, audit infrastructure, and realtime infrastructure.

Before changing router or middleware behavior, inspect its scope carefully.

## Validation

For frontend changes, run the project's existing lint/type/build checks. At minimum, when applicable:

```bash
npm run lint
npm run build
```

If a dedicated typecheck script exists, run it too.

For backend changes:

```bash
go test ./...
go vet ./...
```

When infrastructure behavior changes, verify:
- `GET /health/live`
- `GET /health/ready`

## Scope Discipline

Implement only what the task requires.

If required information is missing:
- inspect the repository;
- inspect Figma when relevant;
- inspect documentation;
- then ask or clearly state the unresolved assumption.

Never silently invent major product requirements.
