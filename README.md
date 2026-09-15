# NIMNear

NIMNear is a location-based discovery Mini App for the Nimiq ecosystem.

## Stack

Frontend:
- Next.js
- App Router
- TypeScript
- Tailwind CSS
- shadcn/ui
- Nimiq Mini App SDK

Backend:
- Go
- NIMNear API
- PostgreSQL
- Redis
- Kafka

## Structure

- `frontend/web` — frontend application
- `backend` — Go/NIMNear backend
- `docs` — product, architecture and design documentation
- `AGENTS.md` — instructions for AI coding agents

## Development

AI coding agents must read `AGENTS.md` before making changes.

Additional project documentation is available under `docs/`.

## Design

Figma is the source of truth for the NIMNear UI/UX.

When implementing UI, inspect the relevant Figma design through the configured Figma integration before coding.
